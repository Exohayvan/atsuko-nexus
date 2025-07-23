package p2p

import (
    "bufio"
    "encoding/json"
    "fmt"
    "math/rand"
    "net"
    "os"
    "path/filepath"
    "time"

    "atsuko-nexus/src/logger"
    "atsuko-nexus/src/nodeid"
    "atsuko-nexus/src/settings"
)

var (
	staletime = 24 * time.Hour
)

func TapSync() {
    logger.Log("DEBUG", "tapsync", "Running TapSync")

    // ——— make peerPath absolute ———
    exe, err := os.Executable()
    if err != nil {
        logger.Log("ERROR", "tapsync", "os.Executable failed: "+err.Error())
        return
    }
    baseDir := filepath.Dir(exe)
    peerRel  := fmt.Sprint(settings.Get("storage.peer_cache_file"))
    peerPath := filepath.Join(baseDir, peerRel)
    logger.Log("DEBUG", "tapsync", "Peer cache file (absolute): "+peerPath)

    // 1) Load peers
    peers := loadPeers(peerPath)
    logger.Log("DEBUG", "tapsync", fmt.Sprintf("Loaded %d peers", len(peers)))
    for i, p := range peers {
        logger.Log("DEBUG", "tapsync", fmt.Sprintf("  peer[%d]=%+v", i, p))
    }

    // 2) Filter out self & invalid IPs
    selfID := nodeid.GetNodeID()
    var candidates []PeerEntry
    for _, p := range peers {
        if p.NodeID == selfID {
            continue
        }
        if net.ParseIP(p.IPv4) == nil {
            logger.Log("WARN", "tapsync", fmt.Sprintf("Skipping peer %s (invalid IPv4 %s)", p.NodeID, p.IPv4))
            continue
        }
        candidates = append(candidates, p)
    }
    logger.Log("DEBUG", "tapsync", fmt.Sprintf("Found %d candidate peers", len(candidates)))
    if len(candidates) == 0 {
        logger.Log("WARN", "tapsync", "No other peers to sync with.")
        return
    }

    // 3) Shuffle and try each
    rand.Shuffle(len(candidates), func(i, j int) {
        candidates[i], candidates[j] = candidates[j], candidates[i]
    })

    for _, peer := range candidates {
        addr := net.JoinHostPort(peer.IPv4, fmt.Sprint(peer.Port))
        logger.Log("DEBUG", "tapsync", "Dialing "+peer.NodeID)
        conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
        if err != nil {
            lastSeen := parseTime(peer.LastSeen)
            if time.Since(lastSeen) > staletime {
                logger.Log("INFO", "tapsync", fmt.Sprintf("Peer %s stale; removing.", peer.NodeID))
                peers = removePeer(peers, peer.NodeID)
                savePeers(peerPath, peers)
            } else {
                logger.Log("WARN", "tapsync", fmt.Sprintf("Peer %s unreachable; skipping.", peer.NodeID))
            }
            continue
        }
        defer conn.Close()

        // 4a) Send SYNC
        if _, err := conn.Write([]byte("SYNC\n")); err != nil {
            logger.Log("ERROR", "tapsync", "Failed to send SYNC: "+err.Error())
            continue
        }

        // 4b) Read their list
        conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        incoming, err := bufio.NewReader(conn).ReadString('\n')
        if err != nil {
            logger.Log("ERROR", "tapsync", "Read error: "+err.Error())
            continue
        }
        var theirPeers []PeerEntry
        if err := json.Unmarshal([]byte(incoming), &theirPeers); err != nil {
            logger.Log("ERROR", "tapsync", "JSON unmarshal error: "+err.Error())
            continue
        }
        logger.Log("INFO", "tapsync", fmt.Sprintf("Received %d peers", len(theirPeers)))

        // 4c) Update our lastSeen
        for i := range peers {
            if peers[i].NodeID == selfID {
                peers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
            }
        }

		// persist immediately:
		savePeers(peerPath, peers)

        // 4d) Send our list
        out, _ := json.Marshal(peers)
        conn.Write(out)
        conn.Write([]byte("\n"))

        // 4e) Optionally read merged response
        conn.SetReadDeadline(time.Now().Add(5 * time.Second))
        mergedResp, err := bufio.NewReader(conn).ReadString('\n')
        if err == nil {
            var merged []PeerEntry
            if err := json.Unmarshal([]byte(mergedResp), &merged); err == nil {
                logger.Log("INFO", "tapsync", fmt.Sprintf("Got merged list (%d entries)", len(merged)))
                savePeers(peerPath, merged)
                return
            }
        }

        // 5) Fallback: manual merge
        final := mergePeers(peers, theirPeers)
        savePeers(peerPath, final)
        return
    }

    logger.Log("WARN", "tapsync", "Could not connect to any peer.")
}
