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
	msg, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(msg, "PEERLIST") {
		return
	}

	peerPath := fmt.Sprint(settings.Get("storage.peer_cache_file"))
	peers := loadPeers(peerPath)

	data, _ := json.Marshal(peers)
	conn.Write(data)
	conn.Write([]byte("\n")) // Ensure newline for client to read

	if strings.HasPrefix(msg, "SYNC") {
		peerPath := fmt.Sprint(settings.Get("storage.peer_cache_file"))
		theirData, _ := bufio.NewReader(conn).ReadString('\n')

		var theirPeers []PeerEntry
		if err := json.Unmarshal([]byte(theirData), &theirPeers); err != nil {
			logger.Log("ERROR", "sync", "Invalid sync data from peer: "+err.Error())
			return
		}

		selfID := nodeid.GetNodeID()
		ourPeers := loadPeers(peerPath)
		for i := range ourPeers {
			if ourPeers[i].NodeID == selfID {
				ourPeers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
			}
		}

		// Merge and save incoming peers
		merged := mergePeers(ourPeers, theirPeers)
		savePeers(peerPath, merged)

		// Reply with our list
		resp, _ := json.Marshal(merged)
		conn.Write(resp)
		conn.Write([]byte("\n"))
		return
	}
}