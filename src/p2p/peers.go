package p2p

import "time"

// CountActivePeers reports peers seen within the last hour, excluding the local node.
func CountActivePeers() int {
	peers := loadPeers(peerCachePath())
	if len(peers) == 0 {
		return 0
	}

	cutoff := time.Now().Add(-60 * time.Minute)
	selfNode := localNodeIdentifier()
	selfPeer := hostPeerID()

	count := 0
	for _, peer := range peers {
		if peer.NodeID != "" && peer.NodeID == selfNode {
			continue
		}
		if peer.PeerID != "" && peer.PeerID == selfPeer {
			continue
		}
		if peer.LastSeen == "" || parseTime(peer.LastSeen).After(cutoff) {
			count++
		}
	}
	return count
}
