package network

import (
	"math/rand"
	"sort"
	"sync"
	"time"
)

// MeshPeer represents a node in the mesh network
type MeshPeer struct {
	Address     string    `json:"address"`       // IP:port or node ID
	HopDistance int       `json:"hop_distance"`  // Logical distance (1 = direct, 2 = via peer, etc.)
	Via         string    `json:"via,omitempty"` // Which peer this was learned from
	TrustScore  float64   `json:"trust_score"`   // 0.0 to 1.0
	Latency     int64     `json:"latency"`       // Response time in milliseconds
	LastSeen    time.Time `json:"last_seen"`     // Last successful communication
	IsConnected bool      `json:"is_connected"`  // Currently connected
	IsBeacon    bool      `json:"is_beacon"`     // Is this a beacon node
	Version     string    `json:"version"`       // Node version
	Uptime      float64   `json:"uptime"`        // Reported uptime percentage
}

// PeerTable manages the mesh network peer information
type PeerTable struct {
	peers       map[string]*MeshPeer // address -> MeshPeer
	connections map[string]bool      // currently connected peers
	maxPeers    int                  // maximum number of connections
	mu          sync.RWMutex
}

// NewPeerTable creates a new peer table
func NewPeerTable(maxPeers int) *PeerTable {
	return &PeerTable{
		peers:       make(map[string]*MeshPeer),
		connections: make(map[string]bool),
		maxPeers:    maxPeers,
	}
}

// AddPeer adds or updates a peer in the table
func (pt *PeerTable) AddPeer(address string, hopDistance int, via string, trustScore float64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	peer, exists := pt.peers[address]
	if !exists {
		peer = &MeshPeer{
			Address:     address,
			HopDistance: hopDistance,
			Via:         via,
			TrustScore:  trustScore,
			LastSeen:    time.Now(),
			IsConnected: false,
			IsBeacon:    false,
		}
		pt.peers[address] = peer
	} else {
		// Update existing peer
		if hopDistance < peer.HopDistance {
			peer.HopDistance = hopDistance
			peer.Via = via
		}
		if trustScore > peer.TrustScore {
			peer.TrustScore = trustScore
		}
		peer.LastSeen = time.Now()
	}
}

// UpdatePeerLatency updates the latency for a peer
func (pt *PeerTable) UpdatePeerLatency(address string, latency int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if peer, exists := pt.peers[address]; exists {
		peer.Latency = latency
		peer.LastSeen = time.Now()
	}
}

// UpdatePeerTrust updates the trust score for a peer
func (pt *PeerTable) UpdatePeerTrust(address string, trustScore float64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if peer, exists := pt.peers[address]; exists {
		peer.TrustScore = trustScore
		peer.LastSeen = time.Now()
	}
}

// MarkConnected marks a peer as connected
func (pt *PeerTable) MarkConnected(address string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if peer, exists := pt.peers[address]; exists {
		peer.IsConnected = true
		peer.LastSeen = time.Now()
		pt.connections[address] = true
	}
}

// MarkDisconnected marks a peer as disconnected
func (pt *PeerTable) MarkDisconnected(address string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if peer, exists := pt.peers[address]; exists {
		peer.IsConnected = false
	}
	delete(pt.connections, address)
}

// GetPeer returns a peer by address
func (pt *PeerTable) GetPeer(address string) (*MeshPeer, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	peer, exists := pt.peers[address]
	return peer, exists
}

// GetAllPeers returns all known peers
func (pt *PeerTable) GetAllPeers() []*MeshPeer {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	peers := make([]*MeshPeer, 0, len(pt.peers))
	for _, peer := range pt.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetConnectedPeers returns currently connected peers
func (pt *PeerTable) GetConnectedPeers() []*MeshPeer {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var connected []*MeshPeer
	for address, peer := range pt.peers {
		if pt.connections[address] {
			connected = append(connected, peer)
		}
	}
	return connected
}

// SelectPeers implements the connection selection algorithm from NetworkDesign.txt
func (pt *PeerTable) SelectPeers(count int) []*MeshPeer {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	allPeers := make([]*MeshPeer, 0, len(pt.peers))
	for _, peer := range pt.peers {
		allPeers = append(allPeers, peer)
	}

	if len(allPeers) <= count {
		return allPeers
	}

	// Sort by different criteria and select diverse peers
	selected := make(map[string]*MeshPeer)

	// 1. Select nearest (lowest latency)
	nearest := pt.selectByLatency(allPeers, count/3)
	for _, peer := range nearest {
		selected[peer.Address] = peer
	}

	// 2. Select oldest (highest trust score)
	oldest := pt.selectByTrust(allPeers, count/3)
	for _, peer := range oldest {
		selected[peer.Address] = peer
	}

	// 3. Select distant (highest hop distance)
	distant := pt.selectByHopDistance(allPeers, count/3)
	for _, peer := range distant {
		selected[peer.Address] = peer
	}

	// Convert back to slice
	result := make([]*MeshPeer, 0, len(selected))
	for _, peer := range selected {
		result = append(result, peer)
	}

	// If we don't have enough, fill with random peers
	if len(result) < count {
		remaining := count - len(result)
		random := pt.selectRandom(allPeers, remaining)
		for _, peer := range random {
			if _, exists := selected[peer.Address]; !exists {
				result = append(result, peer)
			}
		}
	}

	return result
}

// selectByLatency selects peers with lowest latency
func (pt *PeerTable) selectByLatency(peers []*MeshPeer, count int) []*MeshPeer {
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Latency < peers[j].Latency
	})
	if count > len(peers) {
		count = len(peers)
	}
	return peers[:count]
}

// selectByTrust selects peers with highest trust score
func (pt *PeerTable) selectByTrust(peers []*MeshPeer, count int) []*MeshPeer {
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].TrustScore > peers[j].TrustScore
	})
	if count > len(peers) {
		count = len(peers)
	}
	return peers[:count]
}

// selectByHopDistance selects peers with highest hop distance
func (pt *PeerTable) selectByHopDistance(peers []*MeshPeer, count int) []*MeshPeer {
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].HopDistance > peers[j].HopDistance
	})
	if count > len(peers) {
		count = len(peers)
	}
	return peers[:count]
}

// selectRandom selects random peers
func (pt *PeerTable) selectRandom(peers []*MeshPeer, count int) []*MeshPeer {
	if count > len(peers) {
		count = len(peers)
	}

	// Create a copy to avoid modifying original slice
	peersCopy := make([]*MeshPeer, len(peers))
	copy(peersCopy, peers)

	// Fisher-Yates shuffle
	for i := len(peersCopy) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		peersCopy[i], peersCopy[j] = peersCopy[j], peersCopy[i]
	}

	return peersCopy[:count]
}

// CleanupOldPeers removes peers that haven't been seen recently
func (pt *PeerTable) CleanupOldPeers(maxAge time.Duration) int {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	removed := 0
	cutoff := time.Now().Add(-maxAge)

	for address, peer := range pt.peers {
		if peer.LastSeen.Before(cutoff) {
			delete(pt.peers, address)
			delete(pt.connections, address)
			removed++
		}
	}

	return removed
}

// GetMeshStats returns statistics about the mesh
func (pt *PeerTable) GetMeshStats() map[string]interface{} {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	totalPeers := len(pt.peers)
	connectedPeers := len(pt.connections)
	beaconPeers := 0
	totalTrustScore := 0.0
	totalLatency := int64(0)
	avgHopDistance := 0.0

	for _, peer := range pt.peers {
		if peer.IsBeacon {
			beaconPeers++
		}
		totalTrustScore += peer.TrustScore
		totalLatency += peer.Latency
		avgHopDistance += float64(peer.HopDistance)
	}

	avgTrustScore := 0.0
	avgLatency := int64(0)
	avgHop := 0.0

	if totalPeers > 0 {
		avgTrustScore = totalTrustScore / float64(totalPeers)
		avgLatency = totalLatency / int64(totalPeers)
		avgHop = avgHopDistance / float64(totalPeers)
	}

	return map[string]interface{}{
		"total_peers":          totalPeers,
		"connected_peers":      connectedPeers,
		"beacon_peers":         beaconPeers,
		"average_trust":        avgTrustScore,
		"average_latency":      avgLatency,
		"average_hop_distance": avgHop,
		"max_peers":            pt.maxPeers,
	}
}

// ProcessGossipMessage processes incoming gossip messages and updates peer table
func (pt *PeerTable) ProcessGossipMessage(senderAddress string, peerUpdates []*MeshPeer) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Mark sender as connected and update its info
	if sender, exists := pt.peers[senderAddress]; exists {
		sender.IsConnected = true
		sender.LastSeen = time.Now()
		pt.connections[senderAddress] = true
	}

	// Process peer updates
	for _, update := range peerUpdates {
		// Increment hop distance for peers learned via gossip
		hopDistance := update.HopDistance + 1
		via := senderAddress

		existing, exists := pt.peers[update.Address]
		if !exists {
			// New peer
			pt.peers[update.Address] = &MeshPeer{
				Address:     update.Address,
				HopDistance: hopDistance,
				Via:         via,
				TrustScore:  update.TrustScore,
				Latency:     update.Latency,
				LastSeen:    time.Now(),
				IsConnected: false,
				IsBeacon:    update.IsBeacon,
				Version:     update.Version,
				Uptime:      update.Uptime,
			}
		} else {
			// Update existing peer if we found a shorter path
			if hopDistance < existing.HopDistance {
				existing.HopDistance = hopDistance
				existing.Via = via
			}
			// Update other fields if they're more recent
			if update.LastSeen.After(existing.LastSeen) {
				existing.TrustScore = update.TrustScore
				existing.Latency = update.Latency
				existing.IsBeacon = update.IsBeacon
				existing.Version = update.Version
				existing.Uptime = update.Uptime
			}
			existing.LastSeen = time.Now()
		}
	}
}

// CreateGossipMessage creates a gossip message with peer updates
func (pt *PeerTable) CreateGossipMessage() []*MeshPeer {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	// Include all peers in gossip message
	peers := make([]*MeshPeer, 0, len(pt.peers))
	for _, peer := range pt.peers {
		peers = append(peers, peer)
	}

	return peers
}
