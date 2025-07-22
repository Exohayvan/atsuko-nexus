package p2p

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway1"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
	"atsuko-nexus/src/settings"
	"gopkg.in/yaml.v3"
)

// Bootstrap initializes peer list, adds self, and optionally connects to a bootstrap node
func Bootstrap() {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	peerPath := filepath.Join(exeDir, fmt.Sprint(settings.Get("storage.peer_cache_file")))

	port := settings.Get("network.listen_port").(int)
	id := nodeid.GetNodeID()

	ipv4 := fetchPublicIP("https://api.ipify.org")
	rawIP := fetchPublicIP("https://api64.ipify.org")
	parsed := net.ParseIP(rawIP)
	var ipv6 string
	if parsed != nil && parsed.To4() == nil {
		ipv6 = parsed.String()
	} else {
		ipv6 = "none"
	}

	self := PeerEntry{
		NodeID:   id,
		IPv4:     ipv4,
		IPv6:     ipv6,
		Port:     port,
		LastSeen: time.Now().UTC().Format(time.RFC3339),
	}

	peers := loadPeers(peerPath)
	peers = upsertPeer(peers, self)
	savePeers(peerPath, peers)

	// Try UPnP forwarding
	tryUPnPForward(port)

	// Check if listener is active
	time.Sleep(500 * time.Millisecond)
	if isPortListening(port) {
		logger.Log("INFO", "bootstrap", fmt.Sprintf("Confirmed listener active on port %d", port))
	} else {
		logger.Log("WARN", "bootstrap", fmt.Sprintf("No active listener detected on port %d", port))
	}

	if len(peers) > 1 {
		logger.Log("INFO", "bootstrap", fmt.Sprintf("Loaded %d peers.", len(peers)))
		return
	}

	fmt.Println("❗ No known peers found besides self.")
	fmt.Println("Enter a known peer in IP:PORT format or type 'search' to attempt discovery.")
	fmt.Println("⚠️ WARNING: Searching may take **months or longer** due to current network size.")
	fmt.Print("➡️ Your input: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "search" {
			logger.Log("INFO", "bootstrap", "Search mode initiated (not yet implemented).")
			break
		}

		if isValidPeer(input) {
			logger.Log("INFO", "bootstrap", "Connecting to peer: "+input)
			remotePeers := fetchPeerListTCP(input)
			for _, rp := range remotePeers {
				peers = upsertPeer(peers, rp)
			}
			savePeers(peerPath, peers)
			break
		}

		fmt.Print("❌ Invalid format. Enter IP:PORT or type 'search': ")
	}
}

// StartBootstrapListener runs a TCP server that responds to PEERLIST\n requests
func StartBootstrapListener() {
	port := settings.Get("network.listen_port").(int)
	listenAddr := fmt.Sprintf("0.0.0.0:%d", port)

	go func() {
		ln, err := net.Listen("tcp", listenAddr)
		if err != nil {
			logger.Log("ERROR", "bootstrap", "Failed to start listener: "+err.Error())
			return
		}
		logger.Log("INFO", "bootstrap", "Listening for bootstrap connections on "+listenAddr)

		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleBootstrapConn(conn)
		}
	}()
}

func handleBootstrapConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	msg, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(msg, "PEERLIST") {
		return
	}

	peerPath := fmt.Sprint(settings.Get("storage.peer_cache_file"))
	peers := loadPeers(peerPath)

	data, _ := json.Marshal(peers)
	conn.Write(data)
	conn.Write([]byte("\n")) // Ensure newline for client to read
}

func fetchPublicIP(apiURL string) string {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func fetchPeerListTCP(addr string) []PeerEntry {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		logger.Log("ERROR", "bootstrap", "Failed to connect to "+addr+": "+err.Error())
		return nil
	}
	defer conn.Close()

	_, err = conn.Write([]byte("PEERLIST\n"))
	if err != nil {
		logger.Log("ERROR", "bootstrap", "Failed to send request: "+err.Error())
		return nil
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		logger.Log("ERROR", "bootstrap", "Failed to read response: "+err.Error())
		return nil
	}

	var peers []PeerEntry
	if err := json.Unmarshal([]byte(resp), &peers); err != nil {
		logger.Log("ERROR", "bootstrap", "Invalid peer format: "+err.Error())
	}
	return peers
}

func loadPeers(path string) []PeerEntry {
	var pf PeerFile
	data, err := os.ReadFile(path)
	if err != nil {
		return []PeerEntry{}
	}
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return []PeerEntry{}
	}
	return pf.Peers
}

func savePeers(path string, peers []PeerEntry) {
	// Check if a folder exists where the file should be
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		logger.Log("WARN", "bootstrap", fmt.Sprintf("A directory named '%s' exists — removing to save file properly.", path))
		if err := os.RemoveAll(path); err != nil {
			logger.Log("ERROR", "bootstrap", fmt.Sprintf("Failed to remove directory '%s': %v", path, err))
			return
		}
	}

	// Ensure the parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Log("ERROR", "bootstrap", fmt.Sprintf("Failed to create directory '%s': %v", dir, err))
		return
	}

	// Marshal YAML
	data, err := yaml.Marshal(PeerFile{Peers: peers})
	if err != nil {
		logger.Log("ERROR", "bootstrap", "Failed to encode peers: "+err.Error())
		return
	}

	// Save file
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		logger.Log("ERROR", "bootstrap", fmt.Sprintf("Failed to save peer file to '%s': %v", path, err))
		return
	}

	logger.Log("INFO", "bootstrap", fmt.Sprintf("Successfully saved peers to '%s'", path))
}

func upsertPeer(list []PeerEntry, new PeerEntry) []PeerEntry {
	for i, p := range list {
		if p.NodeID == new.NodeID {
			list[i] = new
			return list
		}
	}
	return append(list, new)
}

func isValidPeer(input string) bool {
	parts := strings.Split(input, ":")
	if len(parts) != 2 {
		return false
	}
	return net.ParseIP(parts[0]) != nil
}

func isPortListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func tryUPnPForward(port int) {
	devices, _, err := internetgateway1.NewWANIPConnection1Clients()
	if err != nil || len(devices) == 0 {
		logger.Log("WARN", "upnp", "UPnP device not found or error occurred.")
		return
	}
	client := devices[0]
	ip, err := getLocalIP()
	if err != nil {
		logger.Log("WARN", "upnp", "Failed to get local IP: "+err.Error())
		return
	}
	desc := "Atsuko-Nexus Bootstrap Listener"

	err = client.AddPortMapping("", uint16(port), "TCP", uint16(port), ip.String(), true, desc, 0)
	if err != nil {
		logger.Log("ERROR", "upnp", "UPnP port mapping failed: "+err.Error())
		return
	}
	logger.Log("INFO", "upnp", fmt.Sprintf("Port %d successfully forwarded via UPnP", port))
}

func getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}
