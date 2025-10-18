package p2p

import (
	"bufio"
	"encoding/json"
	"time"

	"atsuko-nexus/src/logger"
	"github.com/libp2p/go-libp2p/core/network"
)

func handleStream(stream network.Stream) {
	defer stream.Close()

	if err := stream.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		logger.Log("DEBUG", "p2p", "Failed to set stream deadline: "+err.Error())
	}

	reader := bufio.NewReader(stream)
	line, err := reader.ReadString('\n')
	if err != nil {
		logger.Log("DEBUG", "p2p", "Failed reading stream payload: "+err.Error())
		return
	}

	var incoming []PeerEntry
	if err := json.Unmarshal([]byte(line), &incoming); err != nil {
		logger.Log("DEBUG", "p2p", "Invalid stream payload: "+err.Error())
		return
	}

	remotePeer := stream.Conn().RemotePeer().String()
	remoteAddr := ""
	if remote := stream.Conn().RemoteMultiaddr(); remote != nil {
		remoteAddr = remote.String()
	}
	ingestPeerEntries(incoming, remotePeer, remoteAddr, nil)

	refreshSelfEntry()
	responsePeers := mergePeers(getKnownPeers(), []PeerEntry{buildSelfPeerEntry()})

	payload, err := json.Marshal(responsePeers)
	if err != nil {
		logger.Log("ERROR", "p2p", "Failed to marshal response payload: "+err.Error())
		return
	}

	payload = append(payload, '\n')
	if _, err := stream.Write(payload); err != nil {
		logger.Log("DEBUG", "p2p", "Failed writing response payload: "+err.Error())
	}
}
