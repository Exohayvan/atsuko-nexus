package p2p

import (
    "bufio"
    "encoding/json"
    "fmt"
    "net"
    "strings"
    "time"

    "atsuko-nexus/src/logger"
    "atsuko-nexus/src/nodeid"
    "atsuko-nexus/src/settings"
)

// StartNexusListener spins up your TCP listener and dispatches incoming connections.
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

// handleNexusConn processes either a PEERLIST or SYNC command.
func handleNexusConn(conn net.Conn) {
    defer conn.Close()

    reader := bufio.NewReader(conn)
    conn.SetReadDeadline(time.Now().Add(10 * time.Second))

    // 1) Read the incoming command
    line, err := reader.ReadString('\n')
    if err != nil {
        return
    }
    cmd := strings.TrimSpace(line)

    // 2) Prep common values
    peerPath := fmt.Sprint(settings.Get("storage.peer_cache_file"))
    selfID   := nodeid.GetNodeID()

    switch cmd {

    case "PEERLIST":
        // 3a) Load, bump our LastSeen, persist
        peers := loadPeers(peerPath)
        for i := range peers {
            if peers[i].NodeID == selfID {
                peers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
            }
        }
        savePeers(peerPath, peers)

        // 3b) Send our updated list
        data, _ := json.Marshal(peers)
        conn.Write(data)
        conn.Write([]byte("\n"))

    case "SYNC":
		logger.Log("INFO", "tapsync", fmt.Sprintf("Sync requested from peer %s", conn.RemoteAddr()))
        // 4a) Load, bump our LastSeen, persist
        peers := loadPeers(peerPath)
        for i := range peers {
            if peers[i].NodeID == selfID {
                peers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
            }
        }
        savePeers(peerPath, peers)

        // 4b) Now send our updated peer list
        out, _ := json.Marshal(peers)
        conn.Write(out)
        conn.Write([]byte("\n"))
        logger.Log("INFO", "tapsync",
            fmt.Sprintf("Sent %d peers to %s", len(peers), conn.RemoteAddr()),
        )

        // 4c) Read their list
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

        // 4d) Merge, save, and reply with the merged set
        merged := mergePeers(peers, theirPeers)
        savePeers(peerPath, merged)

        resp, _ := json.Marshal(merged)
        conn.Write(resp)
        conn.Write([]byte("\n"))

    default:
        // Unknown command: ignore
        return
    }
}
