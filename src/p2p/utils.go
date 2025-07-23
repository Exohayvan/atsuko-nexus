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
	"gopkg.in/yaml.v3"
)

// PeerFile holds the list of peers
type PeerFile struct {
	Peers []PeerEntry `yaml:"peers"`
}

// PeerEntry is a single peer record
type PeerEntry struct {
	NodeID   string `yaml:"node_id" json:"node_id"`
	IPv4     string `yaml:"ipv4" json:"ipv4"`
	IPv6     string `yaml:"ipv6" json:"ipv6"`
	Port     int    `yaml:"port" json:"port"`
	LastSeen string `yaml:"last_seen" json:"last_seen"`
}

// Fetch external IP from an API
func fetchPublicIP(apiURL string) string {
	client := http.Client{Timeout: 5 * time.Second}
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

// Request peer list from another node via TCP
func fetchPeerListTCP(addr string) []PeerEntry {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		logger.Log("ERROR", "nexus", "Failed to connect to "+addr+": "+err.Error())
		return nil
	}
	defer conn.Close()

	_, err = conn.Write([]byte("PEERLIST\n"))
	if err != nil {
		logger.Log("ERROR", "nexus", "Failed to send request: "+err.Error())
		return nil
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		logger.Log("ERROR", "nexus", "Failed to read response: "+err.Error())
		return nil
	}

	var peers []PeerEntry
	if err := json.Unmarshal([]byte(resp), &peers); err != nil {
		logger.Log("ERROR", "nexus", "Invalid peer format: "+err.Error())
		return nil
	}
	logger.Log("INFO", "nexus", "Peer list received from "+addr)
	return peers
}

// Load peer list from YAML file
func loadPeers(path string) []PeerEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Log("ERROR", "peers", fmt.Sprintf("Failed to read peer file at %s: %v", path, err))
		return []PeerEntry{}
	}
	var pf PeerFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		logger.Log("ERROR", "peers", "Failed to unmarshal peer file: "+err.Error())
		return []PeerEntry{}
	}
	return pf.Peers
}


// Save peer list to YAML file
func savePeers(path string, peers []PeerEntry) {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		logger.Log("WARN", "nexus", fmt.Sprintf("A directory named '%s' exists — removing to save file properly.", path))
		if err := os.RemoveAll(path); err != nil {
			logger.Log("ERROR", "nexus", fmt.Sprintf("Failed to remove directory '%s': %v", path, err))
			return
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Log("ERROR", "nexus", fmt.Sprintf("Failed to create directory '%s': %v", dir, err))
		return
	}

	data, err := yaml.Marshal(PeerFile{Peers: peers})
	if err != nil {
		logger.Log("ERROR", "nexus", "Failed to encode peers: "+err.Error())
		return
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		logger.Log("ERROR", "nexus", fmt.Sprintf("Failed to save peer file to '%s': %v", path, err))
		return
	}

	logger.Log("DEBUG", "nexus", fmt.Sprintf("Successfully saved peers.yaml to '%s'", path))
}

// Insert or update a peer entry
func upsertPeer(list []PeerEntry, new PeerEntry) []PeerEntry {
	for i, p := range list {
		if p.NodeID == new.NodeID {
			list[i] = new
			return list
		}
	}
	return append(list, new)
}

// Simple format validation for peer addresses
func isValidPeer(input string) bool {
	parts := strings.Split(input, ":")
	if len(parts) != 2 {
		return false
	}
	return net.ParseIP(parts[0]) != nil
}

// Check if TCP port is listening locally
func isPortListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Try forwarding port via UPnP
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
	desc := "Atsuko-Nexus Listener"
	err = client.AddPortMapping("", uint16(port), "TCP", uint16(port), ip.String(), true, desc, 0)
	if err != nil {
		logger.Log("ERROR", "upnp", "UPnP port mapping failed: "+err.Error())
		return
	}
	logger.Log("INFO", "upnp", fmt.Sprintf("Port %d successfully forwarded via UPnP", port))
}

// Discover local network IP address
func getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// mergePeers merges existing and incoming lists by taking the record with the newest LastSeen timestamp for each NodeID.
func mergePeers(existing []PeerEntry, incoming []PeerEntry) []PeerEntry {
    // Start with a map for easy lookup
    m := make(map[string]PeerEntry, len(existing))
    for _, ex := range existing {
        m[ex.NodeID] = ex
    }

    // For each incoming
    for _, inc := range incoming {
        if ex, ok := m[inc.NodeID]; ok {
            // Compare timestamps
            tEx  := parseTime(ex.LastSeen)
            tInc := parseTime(inc.LastSeen)
            if tInc.After(tEx) {
                // Incoming is fresher: use it (updates IP/port too)
                m[inc.NodeID] = inc
            }
        } else {
            // New peer entirely
            m[inc.NodeID] = inc
        }
    }

    // Re-flatten map back into slice
    merged := make([]PeerEntry, 0, len(m))
    for _, p := range m {
        merged = append(merged, p)
    }
    return merged
}

// Convert time string to time.Time safely
func parseTime(str string) time.Time {
	t, _ := time.Parse(time.RFC3339, str)
	return t
}

func removePeer(peers []PeerEntry, nodeID string) []PeerEntry {
	var out []PeerEntry
	for _, p := range peers {
		if p.NodeID != nodeID {
			out = append(out, p)
		}
	}
	return out
}
