package p2p

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
	"strings"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/settings"
	"atsuko-nexus/src/nodeid"
)

func StartNexusListener() {
	port := settings.Get("network.listen_port").(int)
	listenAddr := fmt.Sprintf("0.0.0.0:%d", port)

	go func() {
		ln, err := net.Listen("tcp", listenAddr)
		if err != nil {
			logger.Log("ERROR", "nexus", "Failed to start listener: "+err.Error())
			return
		}
		logger.Log("INFO", "nexus", "Listening for connections on "+listenAddr)

		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleNexusConn(conn)
		}
	}()
}

func handleNexusConn(conn net.Conn) {
    defer conn.Close()
    reader := bufio.NewReader(conn)
    conn.SetReadDeadline(time.Now().Add(10 * time.Second))

    // Read the command ("PEERLIST\n" or "SYNC\n")
    line, err := reader.ReadString('\n')
    if err != nil {
        return
    }
    cmd := strings.TrimSpace(line)

    // Common path: load our peers file
    peerPath := fmt.Sprint(settings.Get("storage.peer_cache_file"))

    switch cmd {
    case "PEERLIST":
        // Just send our list
        local := loadPeers(peerPath)
        data, _ := json.Marshal(local)
        conn.Write(data)
        conn.Write([]byte("\n"))

    case "SYNC":
        // 1) Send our current list
        local := loadPeers(peerPath)
        data, _ := json.Marshal(local)
        conn.Write(data)
        conn.Write([]byte("\n"))

        // 2) Read their list
        incoming, err := reader.ReadString('\n')
        if err != nil {
            logger.Log("ERROR", "sync", "Failed to read incoming peers: "+err.Error())
            return
        }
        var theirPeers []PeerEntry
        if err := json.Unmarshal([]byte(incoming), &theirPeers); err != nil {
            logger.Log("ERROR", "sync", "Invalid sync data: "+err.Error())
            return
        }

        // 3) Update our own LastSeen
        selfID := nodeid.GetNodeID()
        ourPeers := loadPeers(peerPath)
        for i := range ourPeers {
            if ourPeers[i].NodeID == selfID {
                ourPeers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
            }
        }

        // 4) Merge & save
        merged := mergePeers(ourPeers, theirPeers)
        savePeers(peerPath, merged)

        // 5) Send merged list back
        resp, _ := json.Marshal(merged)
        conn.Write(resp)
        conn.Write([]byte("\n"))

    default:
        // unknown command: do nothing
        return
    }
}
