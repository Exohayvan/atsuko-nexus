package p2p

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/nodeid"
	"atsuko-nexus/src/settings"
	"atsuko-nexus/src/types"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	routingdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	defaultRendezvous = "atsuko-nexus"
)

var (
	protocolID = protocol.ID("/atsuko-nexus/1.0.0")

	libp2pHost host.Host
	kadDHT     *dht.IpfsDHT
	discovery  *routingdisc.RoutingDiscovery
	baseCtx    context.Context
	cancelCtx  context.CancelFunc

	hostOnce sync.Once
	initErr  error

	selfNodeID string
	nodeOnce   sync.Once
)

// StartNexusListener ensures the libp2p host is running and logs listen addresses.
func StartNexusListener() {
	if err := ensureHost(); err != nil {
		logger.Log("ERROR", "p2p", "Failed to start libp2p host: "+err.Error())
		return
	}

	for _, addr := range hostAddrs() {
		logger.Log("INFO", "p2p", "Listening on "+addr)
	}
}

// Bootstrap initialises libp2p, joins the Kademlia DHT, dials bootstrap peers and advertises our service.
func Bootstrap() {
	if isPeerListEmpty() && len(bootstrapPeers()) == 0 {
		userAddr := promptForBootstrapPeer()
		if userAddr != "" {
			logger.Log("INFO", "p2p", "Using manual bootstrap: "+userAddr)
			err := connectToSinglePeer(userAddr)
			if err != nil {
				logger.Log("ERROR", "p2p", "Manual bootstrap failed: "+err.Error())
			}
		}
	}
	if err := ensureHost(); err != nil {
		logger.Log("ERROR", "p2p", "Bootstrap aborted: "+err.Error())
		return
	}

	refreshSelfEntry()

	if boolFromSettings("network.enable_upnp", false) {
		if port := listenPortFromAddrs(libp2pHost.Addrs()); port > 0 {
			go tryUPnPForward(port)
		}
	}

	if err := connectBootstrapPeers(baseCtx); err != nil {
		logger.Log("WARN", "p2p", "Bootstrap peers unreachable: "+err.Error())
	}

	ns := rendezvousNamespace()
	if _, err := discovery.Advertise(baseCtx, ns); err != nil {
		logger.Log("WARN", "p2p", fmt.Sprintf("Failed to advertise namespace %q: %v", ns, err))
	} else {
		logger.Log("INFO", "p2p", fmt.Sprintf("Advertising namespace %q", ns))
	}
}

func isPeerListEmpty() bool {
	peers := CountActivePeers()
	return peers == 0
}

func promptForBootstrapPeer() string {
	fmt.Println("No peers found.")
	fmt.Print("Enter a bootstrap multiaddr (or press Enter to skip): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func ensureHost() error {
	hostOnce.Do(func() {
		baseCtx, cancelCtx = context.WithCancel(context.Background())

		listenPort := intFromSettings("network.listen_port", 0)
		bindAddr := stringFromSettings("network.bind_address", "0.0.0.0")
		listenAddr := fmt.Sprintf("/ip4/%s/tcp/%d", bindAddr, listenPort)

		opts := []libp2p.Option{
			libp2p.ListenAddrStrings(listenAddr),
		}

		if privKey, err := deriveLibp2pIdentity(localNodeIdentifier()); err != nil {
			logger.Log("WARN", "p2p", fmt.Sprintf("Falling back to ephemeral libp2p identity: %v", err))
		} else {
			opts = append(opts, libp2p.Identity(privKey))
		}

		if boolFromSettings("network.enable_nat_traversal", true) {
			opts = append(opts, libp2p.NATPortMap())
		}

		libp2pHost, initErr = libp2p.New(opts...)
		if initErr != nil {
			cancelCtx()
			return
		}

		kadDHT, initErr = dht.New(baseCtx, libp2pHost, dht.Mode(dht.ModeAuto))
		if initErr != nil {
			cancelCtx()
			libp2pHost.Close()
			libp2pHost = nil
			return
		}

		if err := kadDHT.Bootstrap(baseCtx); err != nil {
			initErr = err
			cancelCtx()
			libp2pHost.Close()
			libp2pHost = nil
			return
		}

		discovery = routingdisc.NewRoutingDiscovery(kadDHT)
		libp2pHost.SetStreamHandler(protocolID, handleStream)

		refreshSelfEntry()
	})

	return initErr
}

func connectBootstrapPeers(ctx context.Context) error {
	peers := bootstrapPeers()
	if len(peers) == 0 {
		logger.Log("INFO", "p2p", "No bootstrap peers configured; skipping bootstrap dial.")
		return nil
	}

	var successes int
	var failures []string

	for _, addr := range peers {
		ai, err := addrInfoFromString(addr)
		if err != nil {
			failures = append(failures, err.Error())
			logger.Log("WARN", "p2p", fmt.Sprintf("Invalid bootstrap multiaddr %q: %v", addr, err))
			continue
		}

		if err := libp2pHost.Connect(ctx, *ai); err != nil {
			failures = append(failures, err.Error())
			logger.Log("WARN", "p2p", fmt.Sprintf("Failed to connect bootstrap peer %s: %v", ai.ID, err))
			continue
		}

		logger.Log("INFO", "p2p", fmt.Sprintf("Connected to bootstrap peer %s", ai.ID.ShortString()))
		successes++

		if err := connectAndExchange(ctx, *ai); err != nil {
			logger.Log("WARN", "p2p", fmt.Sprintf("Bootstrap sync with %s failed: %v", ai.ID.ShortString(), err))
		} else {
			logger.Log("INFO", "p2p", fmt.Sprintf("Bootstrap sync with %s completed", ai.ID.ShortString()))
		}
	}

	if successes == 0 && len(failures) > 0 {
		return fmt.Errorf("failed to connect to configured bootstrap peers")
	}
	return nil
}

func refreshSelfEntry() {
	if libp2pHost == nil {
		return
	}

	entry := buildSelfPeerEntry()
	updatePeerEntry(entry)
}

func buildSelfPeerEntry() PeerEntry {
	entry := PeerEntry{
		NodeID:   localNodeIdentifier(),
		PeerID:   libp2pHost.ID().String(),
		Type:     types.NodeType(),
		LastSeen: time.Now().UTC().Format(time.RFC3339),
	}

	info := peer.AddrInfo{ID: libp2pHost.ID(), Addrs: libp2pHost.Addrs()}
	if maddrs, err := peer.AddrInfoToP2pAddrs(&info); err == nil {
		entry.Multiaddrs = multiaddrStrings(maddrs)
	} else {
		logger.Log("WARN", "p2p", "Failed to get local multiaddrs: "+err.Error())
	}

	ipv4 := fetchPublicIP("https://api.ipify.org")
	if ipv4 != "" {
		entry.IPv4 = ipv4
	} else {
		logger.Log("WARN", "p2p", "No public IPv4 detected from ipify")
	}

	ipv6 := fetchPublicIPv6()
	if ipv6 != "" {
		entry.IPv6 = ipv6
	} else {
		entry.IPv6 = "none"
	}

	port := listenPortFromAddrs(libp2pHost.Addrs())
	extra := make([]string, 0, 2)
	if entry.IPv4 != "" && port != 0 {
		extra = append(extra, fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", entry.IPv4, port, entry.PeerID))
	}
	if entry.IPv6 != "" && entry.IPv6 != "none" && port != 0 {
		extra = append(extra, fmt.Sprintf("/ip6/%s/tcp/%d/p2p/%s", entry.IPv6, port, entry.PeerID))
	}

	entry.Multiaddrs = mergeMultiaddrs(entry.Multiaddrs, extra)
	preparePeerEntry(&entry)
	return entry
}

func hostAddrs() []string {
	if libp2pHost == nil {
		return nil
	}

	info := peer.AddrInfo{ID: libp2pHost.ID(), Addrs: libp2pHost.Addrs()}
	maddrs, err := peer.AddrInfoToP2pAddrs(&info)
	if err != nil {
		return nil
	}
	return multiaddrStrings(maddrs)
}

func listenPortFromAddrs(addrs []ma.Multiaddr) int {
	for _, addr := range addrs {
		_, _, port := decodeMultiaddr(addr.String())
		if port != 0 {
			return port
		}
	}
	return 0
}

func bootstrapPeers() []string {
	raw := settings.Get("network.bootstrap_peers")
	switch v := raw.(type) {
	case []string:
		return filterStrings(v)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return filterStrings(out)
	case string:
		return filterStrings([]string{v})
	default:
		return nil
	}
}

func filterStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, val := range values {
		if s := strings.TrimSpace(val); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func addrInfoFromString(addr string) (*peer.AddrInfo, error) {
	maddr, err := ma.NewMultiaddr(strings.TrimSpace(addr))
	if err != nil {
		return nil, err
	}
	return peer.AddrInfoFromP2pAddr(maddr)
}

func rendezvousNamespace() string {
	if ns := stringFromSettings("network.discovery_namespace", defaultRendezvous); ns != "" {
		return ns
	}
	return defaultRendezvous
}

func localNodeIdentifier() string {
	nodeOnce.Do(func() {
		selfNodeID = nodeid.GetNodeID()
	})
	return selfNodeID
}

func hostPeerID() string {
	if libp2pHost == nil {
		return ""
	}
	return libp2pHost.ID().String()
}

func deriveLibp2pIdentity(nodeID string) (crypto.PrivKey, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, fmt.Errorf("empty node ID")
	}

	raw, err := hex.DecodeString(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID hex: %w", err)
	}

	seed := sha256.Sum256(raw)
	privKey, _, err := crypto.GenerateEd25519Key(bytes.NewReader(seed[:]))
	if err != nil {
		return nil, fmt.Errorf("failed to derive libp2p identity: %w", err)
	}
	return privKey, nil
}

func intFromSettings(key string, fallback int) int {
	switch v := settings.Get(key).(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return fallback
	}
}

func stringFromSettings(key, fallback string) string {
	if val, ok := settings.Get(key).(string); ok && strings.TrimSpace(val) != "" {
		return val
	}
	return fallback
}

func boolFromSettings(key string, fallback bool) bool {
	switch v := settings.Get(key).(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		if lower == "true" || lower == "1" || lower == "yes" {
			return true
		}
		if lower == "false" || lower == "0" || lower == "no" {
			return false
		}
	}
	return fallback
}

func connectToSinglePeer(addr string) error {
	ai, err := addrInfoFromString(addr)
	if err != nil {
		return fmt.Errorf("invalid multiaddr: %w", err)
	}

	err = libp2pHost.Connect(baseCtx, *ai)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	logger.Log("INFO", "p2p", "Manually connected to "+ai.ID.String())

	if maddrs, err := peer.AddrInfoToP2pAddrs(ai); err == nil {
		updatePeerEntry(PeerEntry{
			PeerID:     ai.ID.String(),
			Multiaddrs: multiaddrStrings(maddrs),
			LastSeen:   time.Now().UTC().Format(time.RFC3339),
		})
	}

	return nil
}
