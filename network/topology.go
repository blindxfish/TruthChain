package network

import (
	"time"
)

// NetworkTopology manages the network's logical topology and routing
type NetworkTopology struct {
	NodeID         string                // This node's identifier
	Peers          map[string]*Peer      // Direct peer connections
	RouteTable     map[string]*PeerRoute // Routing table for all known nodes
	GossipInterval time.Duration         // How often to send gossip messages
	MaxHops        int                   // Maximum hop distance to track
	LastGossip     int64                 // Timestamp of last gossip
}

// PeerRoute represents a route to a peer in the network
type PeerRoute struct {
	Address     string   // Destination address
	HopDistance int      // Number of hops to reach this peer
	Via         string   // Next hop address (empty if direct)
	TrustScore  float64  // Trust score of the route
	LastUpdate  int64    // When this route was last updated
	Path        []string // Complete path to destination
}

// GossipMessage represents a gossip packet for peer discovery
type GossipMessage struct {
	SourceAddress string     // Address of the sending node
	Peers         []PeerInfo // Known peers and their routes
	Timestamp     int64      // When this message was created
	TTL           int        // Time to live (hop count)
}

// PeerInfo represents peer information in gossip messages
type PeerInfo struct {
	Address  string  `json:"address"`
	Hop      int     `json:"hop"`
	Via      string  `json:"via,omitempty"`
	Trust    float64 `json:"trust"`
	Latency  int     `json:"latency,omitempty"`
	LastSeen int64   `json:"last_seen"`
}

// NewNetworkTopology creates a new network topology manager
func NewNetworkTopology(nodeID string) *NetworkTopology {
	return &NetworkTopology{
		NodeID:         nodeID,
		Peers:          make(map[string]*Peer),
		RouteTable:     make(map[string]*PeerRoute),
		GossipInterval: 30 * time.Second, // 30 seconds as per design
		MaxHops:        10,               // Maximum 10 hops
		LastGossip:     0,
	}
}

// AddPeer adds a direct peer connection
func (nt *NetworkTopology) AddPeer(peer *Peer) {
	nt.Peers[peer.Address] = peer

	// Add direct route (hop distance 0)
	nt.RouteTable[peer.Address] = &PeerRoute{
		Address:     peer.Address,
		HopDistance: 0,
		Via:         "",
		TrustScore:  peer.TrustScore,
		LastUpdate:  time.Now().Unix(),
		Path:        []string{peer.Address},
	}
}

// RemovePeer removes a direct peer connection
func (nt *NetworkTopology) RemovePeer(address string) {
	delete(nt.Peers, address)

	// Remove direct route
	delete(nt.RouteTable, address)

	// Remove any routes that go through this peer
	for dest, route := range nt.RouteTable {
		if route.Via == address {
			delete(nt.RouteTable, dest)
		}
	}
}

// UpdateRoute updates or adds a route in the routing table
func (nt *NetworkTopology) UpdateRoute(route *PeerRoute) bool {
	existing, exists := nt.RouteTable[route.Address]

	// Don't update if we have a better route (lower hop count)
	if exists && existing.HopDistance <= route.HopDistance {
		return false
	}

	// Update the route
	nt.RouteTable[route.Address] = route
	return true
}

// GetRoute returns the best route to a destination
func (nt *NetworkTopology) GetRoute(destination string) *PeerRoute {
	return nt.RouteTable[destination]
}

// GetHopDistance returns the hop distance to a destination
func (nt *NetworkTopology) GetHopDistance(destination string) int {
	route := nt.GetRoute(destination)
	if route == nil {
		return -1 // Unreachable
	}
	return route.HopDistance
}

// CreateGossipMessage creates a gossip message with current peer information
func (nt *NetworkTopology) CreateGossipMessage() *GossipMessage {
	peers := make([]PeerInfo, 0, len(nt.RouteTable))

	for address, route := range nt.RouteTable {
		peerInfo := PeerInfo{
			Address:  address,
			Hop:      route.HopDistance,
			Trust:    route.TrustScore,
			LastSeen: route.LastUpdate,
		}

		if route.Via != "" {
			peerInfo.Via = route.Via
		}

		// Add latency for direct peers
		if peer, exists := nt.Peers[address]; exists {
			peerInfo.Latency = peer.Latency
		}

		peers = append(peers, peerInfo)
	}

	return &GossipMessage{
		SourceAddress: nt.NodeID,
		Peers:         peers,
		Timestamp:     time.Now().Unix(),
		TTL:           nt.MaxHops,
	}
}

// ProcessGossipMessage processes an incoming gossip message
func (nt *NetworkTopology) ProcessGossipMessage(msg *GossipMessage) int {
	updatedRoutes := 0

	for _, peerInfo := range msg.Peers {
		// Skip if this is about ourselves
		if peerInfo.Address == nt.NodeID {
			continue
		}

		// Calculate new hop distance
		newHopDistance := peerInfo.Hop + 1
		if newHopDistance > nt.MaxHops {
			continue // Too far away
		}

		// Create new route
		newRoute := &PeerRoute{
			Address:     peerInfo.Address,
			HopDistance: newHopDistance,
			Via:         msg.SourceAddress,
			TrustScore:  peerInfo.Trust,
			LastUpdate:  time.Now().Unix(),
			Path:        append([]string{msg.SourceAddress}, peerInfo.Address),
		}

		// Update route if it's better
		if nt.UpdateRoute(newRoute) {
			updatedRoutes++
		}
	}

	return updatedRoutes
}

// SelectPeers implements the connection strategy from NetworkDesign.txt
// Returns the best peers based on: nearest, most trusted, most distant
func (nt *NetworkTopology) SelectPeers(count int) []*Peer {
	if count <= 0 {
		return []*Peer{}
	}

	// Convert peers map to slice
	peers := make([]*Peer, 0, len(nt.Peers))
	for _, peer := range nt.Peers {
		peers = append(peers, peer)
	}

	if len(peers) <= count {
		return peers
	}

	// Sort by different criteria
	nearest := nt.getNearestPeer(peers)
	mostTrusted := nt.getMostTrustedPeer(peers)
	mostDistant := nt.getMostDistantPeer(peers)

	// Create result set
	result := make([]*Peer, 0, count)
	seen := make(map[string]bool)

	// Add unique peers in priority order
	candidates := []*Peer{nearest, mostTrusted, mostDistant}
	for _, peer := range candidates {
		if peer != nil && !seen[peer.Address] && len(result) < count {
			result = append(result, peer)
			seen[peer.Address] = true
		}
	}

	// Fill remaining slots with other peers
	for _, peer := range peers {
		if !seen[peer.Address] && len(result) < count {
			result = append(result, peer)
			seen[peer.Address] = true
		}
	}

	return result
}

// getNearestPeer returns the peer with lowest latency
func (nt *NetworkTopology) getNearestPeer(peers []*Peer) *Peer {
	if len(peers) == 0 {
		return nil
	}

	var nearest *Peer
	minLatency := int(^uint(0) >> 1) // Max int

	for _, peer := range peers {
		if peer.Latency > 0 && peer.Latency < minLatency {
			minLatency = peer.Latency
			nearest = peer
		}
	}

	return nearest
}

// getMostTrustedPeer returns the peer with highest trust score
func (nt *NetworkTopology) getMostTrustedPeer(peers []*Peer) *Peer {
	if len(peers) == 0 {
		return nil
	}

	var mostTrusted *Peer
	maxTrust := -1.0

	for _, peer := range peers {
		if peer.TrustScore > maxTrust {
			maxTrust = peer.TrustScore
			mostTrusted = peer
		}
	}

	return mostTrusted
}

// getMostDistantPeer returns the peer with highest hop distance
func (nt *NetworkTopology) getMostDistantPeer(peers []*Peer) *Peer {
	if len(peers) == 0 {
		return nil
	}

	var mostDistant *Peer
	maxHops := -1

	for _, peer := range peers {
		hopDistance := nt.GetHopDistance(peer.Address)
		if hopDistance > maxHops {
			maxHops = hopDistance
			mostDistant = peer
		}
	}

	return mostDistant
}

// GetNetworkStats returns statistics about the network topology
func (nt *NetworkTopology) GetNetworkStats() map[string]interface{} {
	totalPeers := len(nt.Peers)
	totalRoutes := len(nt.RouteTable)

	// Calculate average trust score
	totalTrust := 0.0
	trustCount := 0
	for _, peer := range nt.Peers {
		totalTrust += peer.TrustScore
		trustCount++
	}

	avgTrust := 0.0
	if trustCount > 0 {
		avgTrust = totalTrust / float64(trustCount)
	}

	// Calculate hop distribution
	hopDistribution := make(map[int]int)
	for _, route := range nt.RouteTable {
		hopDistribution[route.HopDistance]++
	}

	return map[string]interface{}{
		"total_peers":      totalPeers,
		"total_routes":     totalRoutes,
		"average_trust":    avgTrust,
		"hop_distribution": hopDistribution,
		"last_gossip":      nt.LastGossip,
		"gossip_interval":  nt.GossipInterval.Seconds(),
	}
}

// CleanupStaleRoutes removes routes that haven't been updated recently
func (nt *NetworkTopology) CleanupStaleRoutes(maxAge time.Duration) int {
	removed := 0
	cutoff := time.Now().Unix() - int64(maxAge.Seconds())

	for address, route := range nt.RouteTable {
		if route.LastUpdate < cutoff {
			delete(nt.RouteTable, address)
			removed++
		}
	}

	return removed
}
