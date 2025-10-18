package p2p

import (
    "time"
)

// ingestPeerEntries normalizes incoming peer metadata before merging it into the cache.
func ingestPeerEntries(entries []PeerEntry, remotePeerID, remoteConnAddr string, remoteAddrs []string) {
	if len(entries) == 0 {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	remoteNodeID := ""

	for _, entry := range entries {
		if entry.PeerID == remotePeerID && entry.NodeID != "" {
			remoteNodeID = entry.NodeID
			break
		}
	}

	for _, entry := range entries {
		if entry.LastSeen == "" {
			entry.LastSeen = now
		}

		isRemoteSelf := entry.PeerID == remotePeerID ||
			(remoteNodeID != "" && entry.NodeID == remoteNodeID)

		if isRemoteSelf {
			combined := make([]string, 0, len(entry.Multiaddrs)+len(remoteAddrs)+1)
			if remoteConnAddr != "" {
				combined = append(combined, remoteConnAddr)
			}
			combined = append(combined, remoteAddrs...)
			combined = append(combined, entry.Multiaddrs...)
			entry.Multiaddrs = mergeMultiaddrs(nil, combined)
		} else {
			entry.Multiaddrs = mergeMultiaddrs(nil, entry.Multiaddrs)
		}

		if isRemoteSelf {
			if entry.PeerID == "" {
				entry.PeerID = remotePeerID
			}
		}

		updatePeerEntry(entry)
	}
}
