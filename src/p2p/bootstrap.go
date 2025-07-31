package p2p

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"


	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
	"atsuko-nexus/src/settings"
	"atsuko-nexus/src/types"
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

	localType := types.NodeType()
	self := PeerEntry{
		NodeID:   id,
		Type: localType,
		IPv4:     ipv4,
		IPv6:     ipv6,
		Port:     port,
		LastSeen: time.Now().UTC().Format(time.RFC3339),
	}

	peers := loadPeers(peerPath)
	peers = upsertPeer(peers, self)
	savePeers(peerPath, peers)

	tryUPnPForward(port)

	time.Sleep(500 * time.Millisecond)
	if isPortListening(port) {
		logger.Log("INFO", "nexus", fmt.Sprintf("Confirmed listener active on port %d", port))
	} else {
		logger.Log("WARN", "nexus", fmt.Sprintf("No active listener detected on port %d", port))
	}

	if len(peers) > 1 {
		logger.Log("INFO", "nexus", fmt.Sprintf("Loaded %d peers.", (len(peers)-1)))
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
			logger.Log("INFO", "nexus", "Search mode initiated (not yet implemented).")
			break
		}

		if isValidPeer(input) {
			logger.Log("INFO", "nexus", "Connecting to peer: "+input)
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
