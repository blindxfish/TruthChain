package network

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/chain"
)

// MeshSyncManager manages chain synchronization using the mesh network
type MeshSyncManager struct {
	trustNetwork *TrustNetwork
	blockchain   *blockchain.Blockchain

	// Configuration
	syncInterval      time.Duration
	maxConcurrentSync int
	syncTimeout       time.Duration
	headerSyncTimeout time.Duration

	// State
	isRunning      bool
	lastSyncTime   time.Time
	syncInProgress bool
	mu             sync.RWMutex

	// Channels
	stopChan        chan struct{}
	syncRequestChan chan SyncRequest
}

// SyncRequest represents a request to sync from a specific peer
type SyncRequest struct {
	PeerID    string
	FromIndex int
	ToIndex   int
	Priority  int // Higher priority = more urgent
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

// NewMeshSyncManager creates a new mesh-integrated sync manager
func NewMeshSyncManager(trustNetwork *TrustNetwork, blockchain *blockchain.Blockchain) *MeshSyncManager {
	return &MeshSyncManager{
		trustNetwork:      trustNetwork,
		blockchain:        blockchain,
		syncInterval:      chain.SyncIntervalFast, // Bitcoin-style: fast sync for active nodes
		maxConcurrentSync: 3,
		syncTimeout:       chain.BlockSyncTimeout,
		headerSyncTimeout: chain.HeaderSyncTimeout,
		stopChan:          make(chan struct{}),
		syncRequestChan:   make(chan SyncRequest, 100),
	}
}

// Start starts the mesh sync manager
func (msm *MeshSyncManager) Start() error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if msm.isRunning {
		return fmt.Errorf("mesh sync manager already running")
	}

	msm.isRunning = true

	// Start background sync worker
	go msm.syncWorker()

	// Start periodic sync
	go msm.periodicSync()

	log.Printf("[MeshSync] Started mesh sync manager")
	return nil
}

// Stop stops the mesh sync manager
func (msm *MeshSyncManager) Stop() error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if !msm.isRunning {
		return nil
	}

	msm.isRunning = false
	close(msm.stopChan)

	log.Printf("[MeshSync] Stopped mesh sync manager")
	return nil
}

// syncWorker processes sync requests
func (msm *MeshSyncManager) syncWorker() {
	for {
		select {
		case <-msm.stopChan:
			return
		case req := <-msm.syncRequestChan:
			msm.processSyncRequest(req)
		}
	}
}

// processSyncRequest processes a single sync request
func (msm *MeshSyncManager) processSyncRequest(req SyncRequest) {
	if msm.syncInProgress {
		log.Printf("[MeshSync] Sync already in progress, queuing request from %s", req.PeerID)
		return
	}

	msm.mu.Lock()
	msm.syncInProgress = true
	msm.mu.Unlock()

	defer func() {
		msm.mu.Lock()
		msm.syncInProgress = false
		msm.mu.Unlock()
	}()

	log.Printf("[MeshSync] Processing sync request from %s (blocks %d-%d)", req.PeerID, req.FromIndex, req.ToIndex)

	// Get peer info
	peer, exists := msm.trustNetwork.PeerTable.GetPeer(req.PeerID)
	if !exists {
		log.Printf("[MeshSync] Peer %s not found in peer table", req.PeerID)
		return
	}

	// Perform sync
	result, err := msm.SyncFromPeer(peer, req.FromIndex, req.ToIndex)
	if err != nil {
		log.Printf("[MeshSync] Sync failed from %s: %v", req.PeerID, err)
		msm.updatePeerTrust(req.PeerID, false)
		return
	}

	log.Printf("[MeshSync] Sync completed from %s: %d blocks added, %d skipped",
		req.PeerID, result.BlocksAdded, result.BlocksSkipped)

	msm.updatePeerTrust(req.PeerID, true)
	msm.lastSyncTime = time.Now()
}

// SyncFromPeer performs the actual sync operation with Bitcoin-style header-first sync
func (msm *MeshSyncManager) SyncFromPeer(peer *MeshPeer, fromIndex, toIndex int) (*SyncResult, error) {
	startTime := time.Now()

	// Step 1: Header-only sync (Bitcoin-style)
	log.Printf("[MeshSync] Starting header-only sync from %s (blocks %d-%d)", peer.Address, fromIndex, toIndex)

	headerReq := chain.ChainSyncRequest{
		FromIndex:   fromIndex,
		ToIndex:     toIndex,
		NodeID:      msm.trustNetwork.NodeID,
		Timestamp:   time.Now().Unix(),
		HeadersOnly: true,
	}

	// Send header request
	headerResponse, err := msm.sendSyncRequest(peer, headerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get headers: %w", err)
	}

	// Validate headers
	if err := chain.ValidateChainHeaders(headerResponse.Headers); err != nil {
		return nil, fmt.Errorf("invalid headers: %w", err)
	}

	log.Printf("[MeshSync] Validated %d headers from %s", len(headerResponse.Headers), peer.Address)

	// Step 2: Check if we need full blocks
	currentLength, err := msm.blockchain.GetChainLength()
	if err != nil {
		return nil, fmt.Errorf("failed to get current chain length: %w", err)
	}

	// If we have no blocks, we need the full chain
	if currentLength == 0 {
		log.Printf("[MeshSync] No local chain - downloading full blocks from %s", peer.Address)

		// Request full blocks
		blockReq := chain.ChainSyncRequest{
			FromIndex:   fromIndex,
			ToIndex:     toIndex,
			NodeID:      msm.trustNetwork.NodeID,
			Timestamp:   time.Now().Unix(),
			HeadersOnly: false,
		}

		blockResponse, err := msm.sendSyncRequest(peer, blockReq)
		if err != nil {
			return nil, fmt.Errorf("failed to get blocks: %w", err)
		}

		// Validate and integrate the full chain
		blocksAdded, blocksSkipped, err := msm.blockchain.ValidateAndIntegrateChain(blockResponse.Blocks)
		if err != nil {
			return nil, fmt.Errorf("failed to integrate chain: %w", err)
		}

		result := &SyncResult{
			Success:       true,
			BlocksAdded:   blocksAdded,
			BlocksSkipped: blocksSkipped,
			PeerID:        peer.Address,
			Duration:      time.Since(startTime),
		}

		return result, nil
	}

	// Step 3: For existing chains, check if headers indicate a better chain
	latestHeader := headerResponse.Headers[len(headerResponse.Headers)-1]
	if latestHeader.Index <= currentLength-1 {
		// Peer's chain is not longer than ours
		return &SyncResult{
			Success:       true,
			BlocksAdded:   0,
			BlocksSkipped: 0,
			PeerID:        peer.Address,
			Duration:      time.Since(startTime),
		}, nil
	}

	// Step 4: Download full blocks for the new portion
	log.Printf("[MeshSync] Downloading new blocks from %s (index %d-%d)", peer.Address, currentLength, latestHeader.Index)

	blockReq := chain.ChainSyncRequest{
		FromIndex:   currentLength,
		ToIndex:     latestHeader.Index,
		NodeID:      msm.trustNetwork.NodeID,
		Timestamp:   time.Now().Unix(),
		HeadersOnly: false,
	}

	blockResponse, err := msm.sendSyncRequest(peer, blockReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get new blocks: %w", err)
	}

	// Integrate new blocks
	blocksAdded, blocksSkipped, err := msm.blockchain.IntegrateBlocksFromSync(blockResponse.Blocks)
	if err != nil {
		return nil, fmt.Errorf("failed to integrate new blocks: %w", err)
	}

	result := &SyncResult{
		Success:       true,
		BlocksAdded:   blocksAdded,
		BlocksSkipped: blocksSkipped,
		PeerID:        peer.Address,
		Duration:      time.Since(startTime),
	}

	return result, nil
}

// sendSyncRequest sends a sync request via the mesh network
func (msm *MeshSyncManager) sendSyncRequest(peer *MeshPeer, req chain.ChainSyncRequest) (*chain.ChainSyncResponse, error) {
	// TODO: Implement actual mesh network communication
	// For now, use the transport layer directly

	// Parse address to get IP and port
	// For now, assume peer.Address is in format "IP:port"
	return SyncFromPeerTCPWithHeaders(peer.Address, req.FromIndex, req.ToIndex, req.NodeID, req.HeadersOnly)
}

// updatePeerTrust updates peer trust score based on sync result
func (msm *MeshSyncManager) updatePeerTrust(peerID string, success bool) {
	if success {
		msm.trustNetwork.PeerTable.UpdatePeerTrust(peerID, 0.05)
	} else {
		msm.trustNetwork.PeerTable.UpdatePeerTrust(peerID, -0.1)
	}
}

// periodicSync performs periodic chain synchronization (Bitcoin-style frequent checks)
func (msm *MeshSyncManager) periodicSync() {
	ticker := time.NewTicker(msm.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-msm.stopChan:
			return
		case <-ticker.C:
			msm.performPeriodicSync()
		}
	}
}

// performPeriodicSync performs a periodic sync with the best available peers
func (msm *MeshSyncManager) performPeriodicSync() {
	if msm.syncInProgress {
		log.Printf("[MeshSync] Skipping periodic sync - sync already in progress")
		return
	}

	// Get current chain length
	currentLength, err := msm.blockchain.GetChainLength()
	if err != nil {
		log.Printf("[MeshSync] Failed to get chain length: %v", err)
		return
	}

	// Get best peers for syncing
	bestPeers := msm.getBestPeersForSync(3)
	if len(bestPeers) == 0 {
		log.Printf("[MeshSync] No suitable peers available for sync")
		return
	}

	log.Printf("[MeshSync] Starting periodic sync with %d peers", len(bestPeers))

	// Sync from the best peer
	bestPeer := bestPeers[0]
	req := SyncRequest{
		PeerID:    bestPeer.Address,
		FromIndex: currentLength,
		ToIndex:   -1, // Get latest
		Priority:  1,
	}

	select {
	case msm.syncRequestChan <- req:
		log.Printf("[MeshSync] Queued sync request from %s", bestPeer.Address)
	default:
		log.Printf("[MeshSync] Sync request queue full, skipping periodic sync")
	}
}

// getBestPeersForSync returns the best peers for syncing
func (msm *MeshSyncManager) getBestPeersForSync(maxPeers int) []*MeshPeer {
	// Get peers from peer table
	peers := msm.trustNetwork.PeerTable.GetConnectedPeers()

	// Filter and sort by trust score
	var suitablePeers []*MeshPeer
	for _, peer := range peers {
		if peer.TrustScore >= 0.3 && peer.IsConnected {
			suitablePeers = append(suitablePeers, peer)
		}
	}

	// Sort by trust score (highest first)
	for i := 0; i < len(suitablePeers)-1; i++ {
		for j := i + 1; j < len(suitablePeers); j++ {
			if suitablePeers[i].TrustScore < suitablePeers[j].TrustScore {
				suitablePeers[i], suitablePeers[j] = suitablePeers[j], suitablePeers[i]
			}
		}
	}

	// Return top peers
	if len(suitablePeers) > maxPeers {
		suitablePeers = suitablePeers[:maxPeers]
	}

	return suitablePeers
}

// RequestSync requests a sync from a specific peer
func (msm *MeshSyncManager) RequestSync(peerID string, fromIndex, toIndex int, priority int) error {
	req := SyncRequest{
		PeerID:    peerID,
		FromIndex: fromIndex,
		ToIndex:   toIndex,
		Priority:  priority,
	}

	select {
	case msm.syncRequestChan <- req:
		return nil
	default:
		return fmt.Errorf("sync request queue full")
	}
}

// GetSyncStats returns sync statistics
func (msm *MeshSyncManager) GetSyncStats() map[string]interface{} {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["mesh_sync_running"] = msm.isRunning
	stats["mesh_sync_in_progress"] = msm.syncInProgress
	stats["mesh_last_sync_time"] = msm.lastSyncTime
	stats["mesh_sync_interval"] = msm.syncInterval.String()

	return stats
}

// DiscoverPeersFromBeacons discovers peers from beacon announcements
func (msm *MeshSyncManager) DiscoverPeersFromBeacons() error {
	// For now, skip beacon discovery since syncManager is not initialized
	// TODO: Implement beacon discovery from blockchain or direct beacon queries
	log.Printf("[MeshSync] Beacon discovery not yet implemented - skipping")
	return nil
}

// BroadcastNewBlock broadcasts a new block to all connected peers
func (msm *MeshSyncManager) BroadcastNewBlock(block *chain.Block) error {
	// TODO: Implement block broadcasting via mesh network
	// For now, just log the broadcast attempt

	log.Printf("[MeshSync] Would broadcast new block %d to mesh network", block.Index)
	return nil
}

// HandleBlockAnnouncement handles incoming block announcements
func (msm *MeshSyncManager) HandleBlockAnnouncement(block *chain.Block, sourcePeer string) error {
	// Check if we already have this block
	currentLength, err := msm.blockchain.GetChainLength()
	if err != nil {
		return fmt.Errorf("failed to get chain length: %w", err)
	}

	if block.Index < currentLength {
		// We already have this block or a conflicting one
		existingBlock, err := msm.blockchain.GetBlockByIndex(block.Index)
		if err != nil {
			return fmt.Errorf("failed to get existing block: %w", err)
		}

		if existingBlock != nil && existingBlock.Hash == block.Hash {
			// We already have this exact block
			return nil
		}

		// Hash mismatch - potential fork
		log.Printf("[MeshSync] Potential fork detected at block %d", block.Index)
	}

	// Request full sync from the source peer
	return msm.RequestSync(sourcePeer, block.Index, -1, 2)
}
