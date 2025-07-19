package p2p

type PeerEntry struct {
	NodeID   string `yaml:"node_id" json:"node_id"`
	IPv4     string `yaml:"ipv4" json:"ipv4"`
	IPv6     string `yaml:"ipv6" json:"ipv6"`
	Port     int    `yaml:"port" json:"port"`
	LastSeen string `yaml:"last_seen" json:"last_seen"`
}

type PeerFile struct {
	Peers []PeerEntry `yaml:"peers"`
}
