package p2p

import (
	"os"
	"time"
	"strings"

	"atsuko-nexus/src/settings"
	"gopkg.in/yaml.v3"
)

// CountActivePeers returns how many peers have been seen within the last 30 minutes.
func CountActivePeers() int {
	exePath, _ := os.Executable()
	peerPath := exePath[:strings.LastIndex(exePath, "/")+1] + settings.Get("storage.peer_cache_file").(string)

	data, err := os.ReadFile(peerPath)
	if err != nil {
		return 0
	}

	var pf PeerFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return 0
	}

	count := -1
	cutoff := time.Now().Add(-60 * time.Minute)
	for _, peer := range pf.Peers {
		if ts, err := time.Parse(time.RFC3339, peer.LastSeen); err == nil {
			if ts.After(cutoff) {
				count++
			}
		}
	}

	return count
}
