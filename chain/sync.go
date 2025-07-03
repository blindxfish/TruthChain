package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// ChainSyncManager manages blockchain synchronization with peers
type ChainSyncManager struct {
	blockchain *Blockchain
	nodeID     string
	peers      map[string]*PeerInfo
}

// PeerInfo represents information about a peer node
type PeerInfo struct {
	NodeID      string    `json:"node_id"`
	IP          string    `json:"ip"`
	Port        int       `json:"port"`
	LastSeen    time.Time `json:"last_seen"`
	TrustScore  float64   `json:"trust_score"`
	IsReachable bool      `json:"is_reachable"`
}

// NewChainSyncManager creates a new chain sync manager
func NewChainSyncManager(blockchain *Blockchain, nodeID string) *ChainSyncManager {
	return &ChainSyncManager{
		blockchain: blockchain,
		nodeID:     nodeID,
		peers:      make(map[string]*PeerInfo),
	}
}

// SyncFromPeer synchronizes blocks from a specific peer
func (csm *ChainSyncManager) SyncFromPeer(peerIP string, peerPort int, fromIndex int) error {
	// Create sync request
	_ = &ChainSyncRequest{
		FromIndex: fromIndex,
		ToIndex:   -1, // Get latest
		NodeID:    csm.nodeID,
		Timestamp: time.Now().Unix(),
	}

	// TODO: Implement actual network communication
	// For now, this is a placeholder that would:
	// 1. Send request to peer
	// 2. Receive response
	// 3. Validate and integrate blocks

	fmt.Printf("Would sync from peer %s:%d starting from block %d\n", peerIP, peerPort, fromIndex)
	return nil
}

// DiscoverBeaconsFromChain extracts beacon nodes from the blockchain
func (csm *ChainSyncManager) DiscoverBeaconsFromChain(maxBlocks int) ([]*BeaconAnnounce, error) {
	var beacons []*BeaconAnnounce

	// Get the latest block index
	latestBlock := csm.blockchain.GetLatestBlock()
	if latestBlock == nil {
		return beacons, fmt.Errorf("no blocks available")
	}

	// Determine how many blocks to scan
	startIndex := 0
	if maxBlocks > 0 && latestBlock.Index >= maxBlocks {
		startIndex = latestBlock.Index - maxBlocks + 1
	}

	// Scan blocks for beacon announcements
	for i := startIndex; i <= latestBlock.Index; i++ {
		block := csm.blockchain.GetBlockByIndex(i)
		if block == nil {
			continue
		}

		if block.BeaconAnnounce != nil {
			// Validate the beacon announcement
			if err := block.BeaconAnnounce.ValidateBeaconAnnounce(); err != nil {
				fmt.Printf("Warning: Invalid beacon announcement in block %d: %v\n", i, err)
				continue
			}

			// Check if beacon is recent (within last 24 hours)
			if time.Now().Unix()-block.BeaconAnnounce.Timestamp < 86400 {
				beacons = append(beacons, block.BeaconAnnounce)
			}
		}
	}

	return beacons, nil
}

// GetReachableBeacons returns only reachable beacon nodes
func (csm *ChainSyncManager) GetReachableBeacons() ([]*BeaconAnnounce, error) {
	allBeacons, err := csm.DiscoverBeaconsFromChain(1000) // Last 1000 blocks
	if err != nil {
		return nil, err
	}

	var reachableBeacons []*BeaconAnnounce
	for _, beacon := range allBeacons {
		// TODO: Implement actual reachability check
		// For now, assume all beacons are reachable
		reachableBeacons = append(reachableBeacons, beacon)
	}

	return reachableBeacons, nil
}

// ValidateBeaconSignature validates a beacon announcement signature
func (csm *ChainSyncManager) ValidateBeaconSignature(beacon *BeaconAnnounce) error {
	// Create the data that was signed (excluding signature)
	beaconData := map[string]interface{}{
		"node_id":   beacon.NodeID,
		"ip":        beacon.IP,
		"port":      beacon.Port,
		"timestamp": beacon.Timestamp,
		"uptime":    beacon.Uptime,
		"version":   beacon.Version,
	}

	// Marshal to JSON
	data, err := json.Marshal(beaconData)
	if err != nil {
		return fmt.Errorf("failed to marshal beacon data: %w", err)
	}

	// Hash the data
	_ = sha256.Sum256(data)

	// Decode signature
	_, err = hex.DecodeString(beacon.Sig)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}

	// TODO: Implement proper signature verification
	// This would require parsing the nodeID to get the public key
	// and then verifying the signature

	fmt.Printf("Would validate signature for beacon %s\n", beacon.NodeID)
	return nil
}

// AddPeer adds a new peer to the sync manager
func (csm *ChainSyncManager) AddPeer(nodeID, ip string, port int) {
	csm.peers[nodeID] = &PeerInfo{
		NodeID:      nodeID,
		IP:          ip,
		Port:        port,
		LastSeen:    time.Now(),
		TrustScore:  0.5, // Initial trust score
		IsReachable: false,
	}
}

// UpdatePeerReachability updates the reachability status of a peer
func (csm *ChainSyncManager) UpdatePeerReachability(nodeID string, isReachable bool) {
	if peer, exists := csm.peers[nodeID]; exists {
		peer.IsReachable = isReachable
		peer.LastSeen = time.Now()

		// Boost trust score for reachable peers
		if isReachable && peer.TrustScore < 0.9 {
			peer.TrustScore += 0.1
			if peer.TrustScore > 0.9 {
				peer.TrustScore = 0.9
			}
		}
	}
}

// GetPeers returns all known peers
func (csm *ChainSyncManager) GetPeers() []*PeerInfo {
	peers := make([]*PeerInfo, 0, len(csm.peers))
	for _, peer := range csm.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetReachablePeers returns only reachable peers
func (csm *ChainSyncManager) GetReachablePeers() []*PeerInfo {
	var reachable []*PeerInfo
	for _, peer := range csm.peers {
		if peer.IsReachable {
			reachable = append(reachable, peer)
		}
	}
	return reachable
}

// CleanupOldPeers removes peers that haven't been seen recently
func (csm *ChainSyncManager) CleanupOldPeers(maxAge time.Duration) int {
	removed := 0
	cutoff := time.Now().Add(-maxAge)

	for nodeID, peer := range csm.peers {
		if peer.LastSeen.Before(cutoff) {
			delete(csm.peers, nodeID)
			removed++
		}
	}

	return removed
}

// GetSyncStats returns statistics about the sync manager
func (csm *ChainSyncManager) GetSyncStats() map[string]interface{} {
	totalPeers := len(csm.peers)
	reachablePeers := 0
	totalTrustScore := 0.0

	for _, peer := range csm.peers {
		if peer.IsReachable {
			reachablePeers++
		}
		totalTrustScore += peer.TrustScore
	}

	avgTrustScore := 0.0
	if totalPeers > 0 {
		avgTrustScore = totalTrustScore / float64(totalPeers)
	}

	return map[string]interface{}{
		"total_peers":         totalPeers,
		"reachable_peers":     reachablePeers,
		"average_trust_score": avgTrustScore,
		"node_id":             csm.nodeID,
	}
}
