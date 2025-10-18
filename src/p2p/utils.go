package p2p

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"atsuko-nexus/src/logger"
	"atsuko-nexus/src/settings"
	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"gopkg.in/yaml.v3"
)

const peerRetention = 14 * 24 * time.Hour

// PeerFile stores persisted peer metadata.
type PeerFile struct {
	Peers []PeerEntry `yaml:"peers"`
}

// PeerEntry captures minimal peer metadata used across the UI and discovery logic.
type PeerEntry struct {
	NodeID     string   `yaml:"node_id" json:"node_id"`
	PeerID     string   `yaml:"peer_id" json:"peer_id"`
	Type       string   `yaml:"type" json:"type"`
	IPv4       string   `yaml:"ipv4" json:"ipv4"`
	IPv6       string   `yaml:"ipv6" json:"ipv6"`
	Port       int      `yaml:"port" json:"port"`
	Multiaddrs []string `yaml:"multiaddrs" json:"multiaddrs"`
	LastSeen   string   `yaml:"last_seen" json:"last_seen"`
}

var (
	peerFileMu   sync.Mutex
	peerPathOnce sync.Once
	peerPath     string
)

func peerCachePath() string {
	peerPathOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			logger.Log("WARN", "peers", fmt.Sprintf("os.Executable failed: %v", err))
			peerPath = "./peers.yaml"
			return
		}

		base := filepath.Dir(exe)
		rel := fmt.Sprint(settings.Get("storage.peer_cache_file"))
		if strings.TrimSpace(rel) == "" {
			rel = "./data/peers/peers.yaml"
		}

		peerPath = filepath.Join(base, rel)
	})
	return peerPath
}

func loadPeers(path string) []PeerEntry {
	if path == "" {
		path = peerCachePath()
	}

	peerFileMu.Lock()
	defer peerFileMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Log("ERROR", "peers", fmt.Sprintf("Failed to read peer file at %s: %v", path, err))
		}
		return []PeerEntry{}
	}

	var pf PeerFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		logger.Log("ERROR", "peers", fmt.Sprintf("Failed to unmarshal peer file at %s: %v", path, err))
		return []PeerEntry{}
	}
	return pf.Peers
}

func savePeers(path string, peers []PeerEntry) {
	if path == "" {
		path = peerCachePath()
	}

	peerFileMu.Lock()
	defer peerFileMu.Unlock()

	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			logger.Log("ERROR", "peers", fmt.Sprintf("Failed to remove directory at %s: %v", path, err))
			return
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Log("ERROR", "peers", fmt.Sprintf("Failed to create peer directory %s: %v", filepath.Dir(path), err))
		return
	}

	data, err := yaml.Marshal(PeerFile{Peers: peers})
	if err != nil {
		logger.Log("ERROR", "peers", fmt.Sprintf("Failed to encode peer file: %v", err))
		return
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		logger.Log("ERROR", "peers", fmt.Sprintf("Failed to write peer file to %s: %v", path, err))
		return
	}
}

func updatePeerEntry(entry PeerEntry) {
	path := peerCachePath()
	current := loadPeers(path)
	merged := mergePeers(current, []PeerEntry{entry})
	pruned, removed := pruneStalePeers(merged, peerRetention)
	if removed {
		logger.Log("DEBUG", "peers", fmt.Sprintf("Pruned stale peers; remaining %d entries", len(pruned)))
	}
	savePeers(path, pruned)
}

func getKnownPeers() []PeerEntry {
	path := peerCachePath()
	current := loadPeers(path)
	pruned, removed := pruneStalePeers(current, peerRetention)
	if removed {
		savePeers(path, pruned)
	}
	return pruned
}

func mergePeer(existing, entry PeerEntry) PeerEntry {
	if entry.NodeID != "" {
		existing.NodeID = entry.NodeID
	}
	if entry.PeerID != "" {
		existing.PeerID = entry.PeerID
	}
	if entry.Type != "" {
		existing.Type = entry.Type
	}
	if entry.IPv4 != "" {
		existing.IPv4 = entry.IPv4
	}
	if entry.IPv6 != "" {
		existing.IPv6 = entry.IPv6
	}
	if entry.Port != 0 {
		existing.Port = entry.Port
	}
	if len(entry.Multiaddrs) > 0 {
		existing.Multiaddrs = mergeMultiaddrs(nil, entry.Multiaddrs)
	}

	if parseTime(entry.LastSeen).After(parseTime(existing.LastSeen)) {
		existing.LastSeen = entry.LastSeen
	}
	return existing
}

func mergePeers(existing []PeerEntry, incoming []PeerEntry) []PeerEntry {
	order := make([]string, 0, len(existing)+len(incoming))
	index := make(map[string]PeerEntry, len(existing)+len(incoming))
	nodeKey := make(map[string]string)
	peerKey := make(map[string]string)
	seen := make(map[string]struct{})

	selectKey := func(entry PeerEntry, fallback string) string {
		if entry.NodeID != "" {
			if key, ok := nodeKey[entry.NodeID]; ok {
				return key
			}
		}
		if entry.PeerID != "" {
			if key, ok := peerKey[entry.PeerID]; ok {
				return key
			}
		}
		return fallback
	}

	addPeer := func(peer PeerEntry) {
		preparePeerEntry(&peer)
		fallback := dedupeKey(peer, len(order))
		key := selectKey(peer, fallback)

		if prev, ok := index[key]; ok {
			index[key] = mergePeer(prev, peer)
		} else {
			index[key] = peer
			if _, already := seen[key]; !already {
				order = append(order, key)
				seen[key] = struct{}{}
			}
		}

		if peer.NodeID != "" {
			nodeKey[peer.NodeID] = key
		}
		if peer.PeerID != "" {
			peerKey[peer.PeerID] = key
		}
	}

	for _, peer := range existing {
		addPeer(peer)
	}
	for _, peer := range incoming {
		addPeer(peer)
	}

	result := make([]PeerEntry, 0, len(index))
	for _, key := range order {
		if peer, ok := index[key]; ok {
			result = append(result, peer)
		}
	}
	return result
}

func removePeer(peers []PeerEntry, peerID string) []PeerEntry {
	peerID = strings.TrimSpace(peerID)
	out := make([]PeerEntry, 0, len(peers))
	for _, p := range peers {
		if p.PeerID == peerID || (p.PeerID == "" && p.NodeID == peerID) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func dedupeKey(entry PeerEntry, fallbackIdx int) string {
	if entry.NodeID != "" {
		return "node:" + entry.NodeID
	}
	if entry.PeerID != "" {
		return "peer:" + entry.PeerID
	}
	if len(entry.Multiaddrs) > 0 {
		return "addr:" + strings.Join(entry.Multiaddrs, "|")
	}
	if entry.IPv4 != "" || entry.Port != 0 {
		return fmt.Sprintf("net:%s:%d", entry.IPv4, entry.Port)
	}
	return fmt.Sprintf("anon:%d:%s", fallbackIdx, entry.LastSeen)
}

func pruneStalePeers(peers []PeerEntry, maxAge time.Duration) ([]PeerEntry, bool) {
	if maxAge <= 0 {
		return peers, false
	}

	cutoff := time.Now().Add(-maxAge)
	selfNode := localNodeIdentifier()
	selfPeer := hostPeerID()

	pruned := make([]PeerEntry, 0, len(peers))
	removed := false

	for _, p := range peers {
		if (selfNode != "" && p.NodeID == selfNode) || (selfPeer != "" && p.PeerID == selfPeer) {
			pruned = append(pruned, p)
			continue
		}

		ts := parseTime(p.LastSeen)
		if ts.IsZero() || ts.After(cutoff) {
			pruned = append(pruned, p)
			continue
		}

		removed = true
	}

	return pruned, removed
}

func preparePeerEntry(entry *PeerEntry) {
	entry.NodeID = strings.TrimSpace(entry.NodeID)
	entry.PeerID = strings.TrimSpace(entry.PeerID)
	entry.Type = strings.TrimSpace(entry.Type)

	if entry.LastSeen == "" {
		entry.LastSeen = time.Now().UTC().Format(time.RFC3339)
	}

	entry.Multiaddrs = mergeMultiaddrs(nil, entry.Multiaddrs)
	if entry.IPv4 == "" || entry.Port == 0 || entry.IPv6 == "" {
		for _, addr := range entry.Multiaddrs {
			ip4, ip6, port := decodeMultiaddr(addr)
			if entry.IPv4 == "" && ip4 != "" {
				entry.IPv4 = ip4
			}
			if entry.IPv6 == "" && ip6 != "" {
				entry.IPv6 = ip6
			}
			if entry.Port == 0 && port != 0 {
				entry.Port = port
			}
		}
	}

	if entry.IPv6 == "" {
		entry.IPv6 = "none"
	}

	if len(entry.Multiaddrs) == 0 {
		if entry.IPv4 != "" && entry.Port != 0 {
			maddr := fmt.Sprintf("/ip4/%s/tcp/%d", entry.IPv4, entry.Port)
			entry.Multiaddrs = mergeMultiaddrs(entry.Multiaddrs, []string{maddr})
		}
		if entry.IPv6 != "" && entry.IPv6 != "none" && entry.Port != 0 {
			maddr := fmt.Sprintf("/ip6/%s/tcp/%d", entry.IPv6, entry.Port)
			entry.Multiaddrs = mergeMultiaddrs(entry.Multiaddrs, []string{maddr})
		}
	}
}

func mergeMultiaddrs(base []string, extra []string) []string {
	m := make(map[string]struct{})
	for _, addr := range append(base, extra...) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		m[addr] = struct{}{}
	}

	out := make([]string, 0, len(m))
	for addr := range m {
		out = append(out, addr)
	}
	sort.Strings(out)
	return out
}

func decodeMultiaddr(addr string) (string, string, int) {
	if strings.TrimSpace(addr) == "" {
		return "", "", 0
	}

	maddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return "", "", 0
	}

	ip4, _ := maddr.ValueForProtocol(ma.P_IP4)
	ip6, _ := maddr.ValueForProtocol(ma.P_IP6)

	portStr, err := maddr.ValueForProtocol(ma.P_TCP)
	if err != nil {
		portStr, _ = maddr.ValueForProtocol(ma.P_UDP)
	}

	port := 0
	if portStr != "" {
		if parsed, convErr := strconv.Atoi(portStr); convErr == nil {
			port = parsed
		}
	}

	return ip4, ip6, port
}

func addrInfosFromEntry(entry PeerEntry) []*peer.AddrInfo {
	if entry.PeerID == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var infos []*peer.AddrInfo

	for _, raw := range entry.Multiaddrs {
		addrStr := strings.TrimSpace(raw)
		if addrStr == "" {
			continue
		}
		if !strings.Contains(addrStr, "/p2p/") {
			addrStr = strings.TrimRight(addrStr, "/")
			addrStr = fmt.Sprintf("%s/p2p/%s", addrStr, entry.PeerID)
		}
		if _, ok := seen[addrStr]; ok {
			continue
		}
		maddr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		ai, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			continue
		}
		seen[addrStr] = struct{}{}
		infos = append(infos, ai)
	}

	return infos
}

// fetchPublicIP retrieves an IP address from a remote service.
func fetchPublicIP(apiURL string) string {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

// parseTime safely parses an RFC3339 timestamp.
func parseTime(str string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(str))
	if err != nil {
		return time.Time{}
	}
	return t
}

// tryUPnPForward requests a WAN port mapping via UPnP/IGD when available.
func tryUPnPForward(listenPort int) {
	if listenPort <= 0 {
		return
	}

	devices, _, err := internetgateway1.NewWANIPConnection1Clients()
	if err != nil || len(devices) == 0 {
		if err != nil {
			logger.Log("DEBUG", "upnp", "UPnP discovery failed: "+err.Error())
		}
		return
	}
	client := devices[0]

	localIP, err := getLocalIP()
	if err != nil {
		logger.Log("DEBUG", "upnp", "Failed to resolve local IP: "+err.Error())
		return
	}

	if _, err := client.GetExternalIPAddress(); err != nil {
		logger.Log("DEBUG", "upnp", "GetExternalIPAddress failed: "+err.Error())
	}

	lease := uint32(0)
	if err := client.AddPortMapping(
		"",
		uint16(listenPort),
		"TCP",
		uint16(listenPort),
		localIP.String(),
		true,
		"Atsuko-Nexus",
		lease,
	); err != nil {
		logger.Log("DEBUG", "upnp", fmt.Sprintf("AddPortMapping(%d) failed: %v", listenPort, err))
		return
	}

	logger.Log("INFO", "upnp", fmt.Sprintf("UPnP port mapping requested for TCP %d", listenPort))
}

func getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

func multiaddrStrings(addrs []ma.Multiaddr) []string {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}
