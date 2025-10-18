package p2p

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"atsuko-nexus/src/logger"
	discopt "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TapSync() {
	if err := ensureHost(); err != nil {
		logger.Log("ERROR", "tapsync", "libp2p host unavailable: "+err.Error())
		return
	}

	knownBefore := getKnownPeers()
	logger.Log("DEBUG", "tapsync", fmt.Sprintf("TapSync start; known peers=%d", len(knownBefore)))

	ctx, cancel := context.WithTimeout(baseCtx, time.Duration(intFromSettings("network.peer_discovery_interval", 60))*time.Second)
	defer cancel()

	ns := rendezvousNamespace()
	peerCh, err := discovery.FindPeers(ctx, ns, discopt.Limit(intFromSettings("network.max_peers", 100)))
	if err != nil {
		logger.Log("WARN", "tapsync", fmt.Sprintf("FindPeers failed: %v", err))
		return
	}

	attempts := 0
	for p := range peerCh {
		if p.ID == libp2pHost.ID() {
			continue
		}
		attempts++
		if err := connectAndExchange(ctx, p); err != nil {
			logger.Log("DEBUG", "tapsync", fmt.Sprintf("DHT sync with %s failed: %v", p.ID.ShortString(), err))
			continue
		}
		logger.Log("DEBUG", "tapsync", fmt.Sprintf("DHT sync with %s succeeded", p.ID.ShortString()))
	}
	logger.Log("DEBUG", "tapsync", fmt.Sprintf("DHT sync attempts completed; tried %d peers", attempts))

	knownCtx, knownCancel := context.WithTimeout(baseCtx, 30*time.Second)
	defer knownCancel()
	syncWithKnownPeers(knownCtx)

	refreshSelfEntry()
	logger.Log("DEBUG", "tapsync", fmt.Sprintf("TapSync complete; known peers=%d", len(getKnownPeers())))
}

func connectAndExchange(parent context.Context, info peer.AddrInfo) error {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()

	if err := libp2pHost.Connect(ctx, info); err != nil {
		return err
	}
	logger.Log("DEBUG", "tapsync", fmt.Sprintf("Initiating sync with %s", info.ID.ShortString()))

	stream, err := libp2pHost.NewStream(ctx, info.ID, protocolID)
	if err != nil {
		return err
	}
	defer stream.Close()

	refreshSelfEntry()
	localPeers := mergePeers(getKnownPeers(), []PeerEntry{buildSelfPeerEntry()})
	logger.Log("DEBUG", "tapsync", fmt.Sprintf("Sending %d peers to %s", len(localPeers), info.ID.ShortString()))

	payload, err := json.Marshal(localPeers)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if _, err := stream.Write(payload); err != nil {
		return err
	}

	stream.SetReadDeadline(time.Now().Add(15 * time.Second))
	resp, err := bufio.NewReader(stream).ReadString('\n')
	if err != nil {
		return err
	}

	var remotePeers []PeerEntry
	if err := json.Unmarshal([]byte(resp), &remotePeers); err != nil {
		return err
	}
	logger.Log("DEBUG", "tapsync", fmt.Sprintf("Received %d peers from %s", len(remotePeers)-1, info.ID.ShortString()))

	remotePeerID := info.ID.String()
	remoteConnAddr := ""
	if remote := stream.Conn().RemoteMultiaddr(); remote != nil {
		remoteConnAddr = remote.String()
	}
	var remoteAddrs []string
	if maddrs, err := peer.AddrInfoToP2pAddrs(&info); err == nil {
		remoteAddrs = multiaddrStrings(maddrs)
	}
	ingestPeerEntries(remotePeers, remotePeerID, remoteConnAddr, remoteAddrs)
	return nil
}

func syncWithKnownPeers(ctx context.Context) {
	known := getKnownPeers()
	if len(known) == 0 {
		logger.Log("DEBUG", "tapsync", "No known peers to sync directly")
		return
	}

	selfID := hostPeerID()
	seen := make(map[string]struct{})

	for _, peerEntry := range known {
		if peerEntry.PeerID == "" || peerEntry.PeerID == selfID {
			continue
		}
		if _, ok := seen[peerEntry.PeerID]; ok {
			continue
		}

		infos := addrInfosFromEntry(peerEntry)
		if len(infos) == 0 {
			continue
		}

		seen[peerEntry.PeerID] = struct{}{}
		for _, ai := range infos {
			if err := connectAndExchange(ctx, *ai); err != nil {
				logger.Log("DEBUG", "tapsync", fmt.Sprintf("Direct sync with %s failed: %v", ai.ID.ShortString(), err))
			} else {
				logger.Log("DEBUG", "tapsync", fmt.Sprintf("Direct sync with %s succeeded", ai.ID.ShortString()))
			}
		}
	}
}
