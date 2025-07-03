package network

import (
	"time"
)

// Peer represents a connected node in the TruthChain network
// Based on NetworkDesign.txt specifications
type Peer struct {
	Address      string   // Node's address (IP:port or wallet address)
	LastSeen     int64    // Unix timestamp of last communication
	FirstSeen    int64    // Unix timestamp when first discovered
	UptimeScore  float64  // Normalized uptime score (0.0 - 1.0)
	AgeScore     float64  // Normalized age score (0.0 - 1.0)
	TrustScore   float64  // Composite trust score (0.0 - 1.0)
	Latency      int      // Measured latency in milliseconds
	HopDistance  int      // Number of logical hops from this node
	Path         []string // Peer routing path to this node
	IsConnected  bool     // Whether currently connected
	ConnectionID string   // Unique connection identifier
}

// TrustEngine manages trust scoring for network peers
type TrustEngine struct {
	UptimeWeight float64 // Weight for uptime in trust calculation (default: 0.6)
	AgeWeight    float64 // Weight for age in trust calculation (default: 0.4)
	MaxAge       int64   // Maximum age in seconds for normalization (default: 365 days)
}

// NewTrustEngine creates a new trust engine with default weights
func NewTrustEngine() *TrustEngine {
	return &TrustEngine{
		UptimeWeight: 0.6,
		AgeWeight:    0.4,
		MaxAge:       365 * 24 * 60 * 60, // 365 days in seconds
	}
}

// CalculateTrustScore computes the composite trust score for a peer
// Formula: TrustScore = 0.6 * UptimeScore + 0.4 * AgeScore
func (te *TrustEngine) CalculateTrustScore(peer *Peer) float64 {
	// Calculate age score (normalized to 0.0 - 1.0)
	ageSeconds := time.Now().Unix() - peer.FirstSeen
	peer.AgeScore = float64(ageSeconds) / float64(te.MaxAge)
	if peer.AgeScore > 1.0 {
		peer.AgeScore = 1.0
	}

	// Calculate composite trust score
	peer.TrustScore = te.UptimeWeight*peer.UptimeScore + te.AgeWeight*peer.AgeScore

	// Ensure trust score is within bounds
	if peer.TrustScore > 1.0 {
		peer.TrustScore = 1.0
	}
	if peer.TrustScore < 0.0 {
		peer.TrustScore = 0.0
	}

	return peer.TrustScore
}

// UpdateUptimeScore updates a peer's uptime score
func (te *TrustEngine) UpdateUptimeScore(peer *Peer, uptimePercent float64) {
	peer.UptimeScore = uptimePercent / 100.0
	if peer.UptimeScore > 1.0 {
		peer.UptimeScore = 1.0
	}
	if peer.UptimeScore < 0.0 {
		peer.UptimeScore = 0.0
	}

	// Recalculate trust score
	te.CalculateTrustScore(peer)
}

// UpdateLatency updates a peer's latency measurement
func (te *TrustEngine) UpdateLatency(peer *Peer, latencyMs int) {
	peer.Latency = latencyMs
	peer.LastSeen = time.Now().Unix()
}

// GetTrustLevel returns a human-readable trust level
func (te *TrustEngine) GetTrustLevel(trustScore float64) string {
	switch {
	case trustScore >= 0.8:
		return "High"
	case trustScore >= 0.6:
		return "Medium"
	case trustScore >= 0.4:
		return "Low"
	default:
		return "Untrusted"
	}
}

// IsTrusted checks if a peer meets minimum trust requirements
func (te *TrustEngine) IsTrusted(peer *Peer, minTrustScore float64) bool {
	return peer.TrustScore >= minTrustScore
}

// GetPeerAge returns the age of a peer in days
func (te *TrustEngine) GetPeerAge(peer *Peer) int {
	ageSeconds := time.Now().Unix() - peer.FirstSeen
	return int(ageSeconds / 86400) // Convert to days
}

// String returns a string representation of the peer
func (p *Peer) String() string {
	return p.Address
}

// IsActive checks if the peer has been seen recently (within last 5 minutes)
func (p *Peer) IsActive() bool {
	return time.Now().Unix()-p.LastSeen < 300 // 5 minutes
}

// GetConnectionAge returns how long this peer has been connected
func (p *Peer) GetConnectionAge() time.Duration {
	return time.Duration(time.Now().Unix()-p.FirstSeen) * time.Second
}
