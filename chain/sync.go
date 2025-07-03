package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ChainSyncManager manages blockchain synchronization with peers
type ChainSyncManager struct {
	blockchain *Blockchain
	nodeID     string
	peers      map[string]*PeerInfo
	mu         sync.RWMutex

	// Sync state
	lastSyncTime   time.Time
	syncInProgress bool
	syncErrors     []string
	maxSyncErrors  int

	// Configuration
	maxBlocksPerSync int
	syncTimeout      time.Duration
	retryInterval    time.Duration
}

// PeerInfo represents information about a peer node
type PeerInfo struct {
	NodeID      string    `json:"node_id"`
	IP          string    `json:"ip"`
	Port        int       `json:"port"`
	LastSeen    time.Time `json:"last_seen"`
	TrustScore  float64   `json:"trust_score"`
	IsReachable bool      `json:"is_reachable"`
	LastSync    time.Time `json:"last_sync"`
	ChainLength int       `json:"chain_length"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Success       bool          `json:"success"`
	BlocksAdded   int           `json:"blocks_added"`
	BlocksSkipped int           `json:"blocks_skipped"`
	Error         string        `json:"error,omitempty"`
	Duration      time.Duration `json:"duration"`
	PeerID        string        `json:"peer_id"`
}

// NewChainSyncManager creates a new chain sync manager
func NewChainSyncManager(blockchain *Blockchain, nodeID string) *ChainSyncManager {
	return &ChainSyncManager{
		blockchain:       blockchain,
		nodeID:           nodeID,
		peers:            make(map[string]*PeerInfo),
		maxBlocksPerSync: 100,
		syncTimeout:      30 * time.Second,
		retryInterval:    5 * time.Minute,
		maxSyncErrors:    10,
	}
}

// SyncFromPeer synchronizes blocks from a specific peer
func (csm *ChainSyncManager) SyncFromPeer(peerIP string, peerPort int, fromIndex int) (*SyncResult, error) {
	csm.mu.Lock()
	if csm.syncInProgress {
		csm.mu.Unlock()
		return nil, fmt.Errorf("sync already in progress")
	}
	csm.syncInProgress = true
	csm.mu.Unlock()

	defer func() {
		csm.mu.Lock()
		csm.syncInProgress = false
		csm.mu.Unlock()
	}()

	startTime := time.Now()
	result := &SyncResult{
		Success: false,
		PeerID:  fmt.Sprintf("%s:%d", peerIP, peerPort),
	}

	// TODO: Implement actual network communication
	// For now, this is a placeholder that would:
	// 1. Send request to peer
	// 2. Receive response
	// 3. Validate and integrate blocks

	fmt.Printf("Would sync from peer %s:%d starting from block %d\n", peerIP, peerPort, fromIndex)

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// SyncBlocks integrates blocks from a sync response
func (csm *ChainSyncManager) SyncBlocks(blocks []*Block, peerID string) (*SyncResult, error) {
	if len(blocks) == 0 {
		return &SyncResult{
			Success:     true,
			BlocksAdded: 0,
			PeerID:      peerID,
		}, nil
	}

	startTime := time.Now()
	result := &SyncResult{
		Success: false,
		PeerID:  peerID,
	}

	// Sort blocks by index to ensure proper order
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Index < blocks[j].Index
	})

	// Get current chain length
	currentLength := csm.blockchain.GetChainLength()

	// Validate and integrate blocks
	blocksAdded := 0
	blocksSkipped := 0

	for _, block := range blocks {
		// Skip if we already have this block
		if block.Index < currentLength {
			existingBlock := csm.blockchain.GetBlockByIndex(block.Index)
			if existingBlock != nil && existingBlock.Hash == block.Hash {
				blocksSkipped++
				continue
			}
			// Hash mismatch indicates a fork - handle it
			if err := csm.handleFork(block, existingBlock); err != nil {
				return result, fmt.Errorf("fork resolution failed: %w", err)
			}
		}

		// Validate block
		if err := block.ValidateBlock(); err != nil {
			return result, fmt.Errorf("invalid block %d: %w", block.Index, err)
		}

		// Check previous hash
		if block.Index > 0 {
			prevBlock := csm.blockchain.GetBlockByIndex(block.Index - 1)
			if prevBlock == nil {
				return result, fmt.Errorf("missing previous block %d", block.Index-1)
			}
			if block.PrevHash != prevBlock.Hash {
				return result, fmt.Errorf("previous hash mismatch at block %d", block.Index)
			}
		}

		// Add block to blockchain
		if err := csm.blockchain.AddBlock(block); err != nil {
			return result, fmt.Errorf("failed to add block %d: %w", block.Index, err)
		}

		blocksAdded++
	}

	result.Success = true
	result.BlocksAdded = blocksAdded
	result.BlocksSkipped = blocksSkipped
	result.Duration = time.Since(startTime)

	// Update peer info
	csm.updatePeerSyncInfo(peerID, len(blocks), true)

	return result, nil
}

// handleFork handles chain forks by determining the correct chain
func (csm *ChainSyncManager) handleFork(newBlock *Block, existingBlock *Block) error {
	// For now, implement a simple longest chain rule
	// In a production system, this would be more sophisticated

	// Calculate chain weight (could be based on difficulty, stake, etc.)
	newChainWeight := csm.calculateChainWeight(newBlock.Index)
	existingChainWeight := csm.calculateChainWeight(existingBlock.Index)

	if newChainWeight > existingChainWeight {
		// New chain is better, reorganize
		return csm.reorganizeChain(newBlock.Index)
	}

	// Existing chain is better, reject new block
	return fmt.Errorf("existing chain is preferred")
}

// calculateChainWeight calculates the weight of a chain
func (csm *ChainSyncManager) calculateChainWeight(blockIndex int) int {
	// Simple implementation: weight = block index (longest chain wins)
	// In production, this could consider:
	// - Proof of work difficulty
	// - Stake amounts
	// - Network consensus
	return blockIndex
}

// reorganizeChain reorganizes the chain to accept a new fork
func (csm *ChainSyncManager) reorganizeChain(newBlockIndex int) error {
	// This is a simplified implementation
	// In production, this would:
	// 1. Find the common ancestor
	// 2. Rollback to that point
	// 3. Apply the new blocks
	// 4. Update state

	fmt.Printf("Chain reorganization needed for block %d\n", newBlockIndex)
	return nil
}

// updatePeerSyncInfo updates peer information after sync
func (csm *ChainSyncManager) updatePeerSyncInfo(peerID string, blocksReceived int, success bool) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	if peer, exists := csm.peers[peerID]; exists {
		peer.LastSync = time.Now()
		peer.LastSeen = time.Now()

		if success {
			// Boost trust score for successful sync
			if peer.TrustScore < 0.9 {
				peer.TrustScore += 0.05
				if peer.TrustScore > 0.9 {
					peer.TrustScore = 0.9
				}
			}
		} else {
			// Reduce trust score for failed sync
			if peer.TrustScore > 0.1 {
				peer.TrustScore -= 0.1
				if peer.TrustScore < 0.1 {
					peer.TrustScore = 0.1
				}
			}
		}
	}
}

// GetBestPeers returns the best peers for syncing
func (csm *ChainSyncManager) GetBestPeers(maxPeers int) []*PeerInfo {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	// Convert to slice and sort by trust score
	peers := make([]*PeerInfo, 0, len(csm.peers))
	for _, peer := range csm.peers {
		if peer.IsReachable {
			peers = append(peers, peer)
		}
	}

	// Sort by trust score (highest first)
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].TrustScore > peers[j].TrustScore
	})

	// Return top peers
	if len(peers) > maxPeers {
		peers = peers[:maxPeers]
	}

	return peers
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
	csm.mu.Lock()
	defer csm.mu.Unlock()

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
	csm.mu.Lock()
	defer csm.mu.Unlock()

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

// UpdatePeerChainLength updates the chain length for a peer
func (csm *ChainSyncManager) UpdatePeerChainLength(nodeID string, chainLength int) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	if peer, exists := csm.peers[nodeID]; exists {
		peer.ChainLength = chainLength
		peer.LastSeen = time.Now()
	}
}

// GetPeers returns all known peers
func (csm *ChainSyncManager) GetPeers() []*PeerInfo {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(csm.peers))
	for _, peer := range csm.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetReachablePeers returns only reachable peers
func (csm *ChainSyncManager) GetReachablePeers() []*PeerInfo {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

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
	csm.mu.Lock()
	defer csm.mu.Unlock()

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
	csm.mu.RLock()
	defer csm.mu.RUnlock()

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
		"last_sync_time":      csm.lastSyncTime,
		"sync_in_progress":    csm.syncInProgress,
		"sync_errors":         len(csm.syncErrors),
	}
}

// IsSyncInProgress returns whether a sync operation is currently running
func (csm *ChainSyncManager) IsSyncInProgress() bool {
	csm.mu.RLock()
	defer csm.mu.RUnlock()
	return csm.syncInProgress
}

// GetLastSyncTime returns the last sync time
func (csm *ChainSyncManager) GetLastSyncTime() time.Time {
	csm.mu.RLock()
	defer csm.mu.RUnlock()
	return csm.lastSyncTime
}
