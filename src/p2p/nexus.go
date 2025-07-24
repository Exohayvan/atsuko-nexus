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
    "atsuko-nexus/src/types"
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

    // 1) Read incoming command
    line, err := reader.ReadString('\n')
    if err != nil {
        return
    }
    cmd := strings.TrimSpace(line)

    // 2) Common values
    peerPath := fmt.Sprint(settings.Get("storage.peer_cache_file"))
    selfID   := nodeid.GetNodeID()
    localType := types.NodeType()                               // <-- determine your node’s role once

    // 3) Fetch our current public IPs
    ipv4  := fetchPublicIP("https://api.ipify.org")
    rawIP := fetchPublicIP("https://api64.ipify.org")

    ipv6 := "none"
    if rawIP != ipv4 {
        if parsed := net.ParseIP(rawIP); parsed != nil && parsed.To4() == nil {
            ipv6 = parsed.String()
        }
    }
    port := settings.Get("network.listen_port").(int)

    switch cmd {
    case "PEERLIST":
        peers := loadPeers(peerPath)
        for i := range peers {
            if peers[i].NodeID == selfID {
                peers[i].Type     = localType                      // <-- update Type
                peers[i].IPv4     = ipv4
                peers[i].IPv6     = ipv6
                peers[i].Port     = port
                peers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
            }
        }
        savePeers(peerPath, peers)

        data, _ := json.Marshal(peers)
        conn.Write(data)
        conn.Write([]byte("\n"))

    case "SYNC":
        logger.Log("INFO", "tapsync", fmt.Sprintf("Sync requested from peer %s", conn.RemoteAddr()))

        // 1) Update our own entry before sending
        peers := loadPeers(peerPath)
        for i := range peers {
            if peers[i].NodeID == selfID {
                peers[i].Type     = localType                      // <-- update Type
                peers[i].LastSeen = time.Now().UTC().Format(time.RFC3339)
                peers[i].IPv4     = ipv4
                peers[i].IPv6     = ipv6
                peers[i].Port     = port
            }
        }
        savePeers(peerPath, peers)

        // 2) Send out our list
        out, _ := json.Marshal(peers)
        conn.Write(out)
        conn.Write([]byte("\n"))
        logger.Log("INFO", "tapsync",
            fmt.Sprintf("Sent %d peers to %s", len(peers), conn.RemoteAddr()),
        )

        // 3) Read their list back
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

        // 4) Merge—including each peer’s Type from their payload—and persist
        merged := mergePeers(peers, theirPeers)
        savePeers(peerPath, merged)

        resp, _ := json.Marshal(merged)
        conn.Write(resp)
        conn.Write([]byte("\n"))

    default:
        return
    }
}
