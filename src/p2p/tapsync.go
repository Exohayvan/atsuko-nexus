package p2p

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"time"
	"bufio"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/settings"
	"atsuko-nexus/src/nodeid"
)

// TapSync performs a one-time two-way sync with a random peer.
func TapSync() {
	peerPath := fmt.Sprint(settings.Get("storage.peer_cache_file"))
	peers := loadPeers(peerPath)

	if len(peers) <= 1 {
		logger.Log("WARN", "tapsync", "No other peers to sync with.")
		return
	}

	// Pick a random peer (not self)
	var ipv4Peers []PeerEntry
	selfID := nodeid.GetNodeID()
	for _, p := range peers {
		if p.NodeID != selfID && net.ParseIP(p.IPv4) != nil {
			ipv4Peers = append(ipv4Peers, p)
		}
	}

	if len(ipv4Peers) == 0 {
		logger.Log("WARN", "tapsync", "No IPv4 peers available.")
		return
	}

	peer := ipv4Peers[rand.Intn(len(ipv4Peers))]
	addr := net.JoinHostPort(peer.IPv4, fmt.Sprint(peer.Port))


	logger.Log("DEBUG", "tapsync", "Attempting sync with "+addr)

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		lastSeen := parseTime(peer.LastSeen)
		if time.Since(lastSeen) > time.Hour {
			logger.Log("INFO", "tapsync", fmt.Sprintf("Peer %s is offline >1h. Removing.", peer.NodeID))
			peers = removePeer(peers, peer.NodeID)
			savePeers(peerPath, peers)
		} else {
			logger.Log("WARN", "tapsync", fmt.Sprintf("Peer %s offline, but recently seen. Keeping.", peer.NodeID))
		}
		return
	}
	defer conn.Close()

	// Send SYNC and our peer list
	conn.Write([]byte("SYNC\n"))

	selfID = nodeid.GetNodeID()
	for i := range peers {
		if peers[i].NodeID == selfID {
			peers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
		}
	}
	outgoing, _ := json.Marshal(peers)
	conn.Write(outgoing)
	conn.Write([]byte("\n"))

	// Receive their peer list
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var theirPeers []PeerEntry
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		logger.Log("ERROR", "tapsync", "Failed to read peer sync reply: "+err.Error())
		return
	}
	if err := json.Unmarshal([]byte(resp), &theirPeers); err != nil {
		logger.Log("ERROR", "tapsync", "Invalid peer response format: "+err.Error())
		return
	}

	logger.Log("INFO", "tapsync", fmt.Sprintf("Received %d peers from %s", len(theirPeers), peer.NodeID))
	merged := mergePeers(peers, theirPeers)
	savePeers(peerPath, merged)
}
