package chain

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// SyncManager handles block catch-up and initial chain synchronization
type SyncManager struct {
	blockchain BlockchainInterface
	nodeID     string
	config     *ConsensusConfig

	// Network callbacks
	onBlockRequest  func(*BlockRequest) error
	onBlockResponse func(*BlockResponse) error

	// State
	isRunning bool
	isSyncing bool
	mu        sync.RWMutex
	stopChan  chan struct{}

	// Sync state
	syncStartTime time.Time
	syncProgress  *SyncProgress
}

// SyncProgress tracks sync progress
type SyncProgress struct {
	StartIndex    int
	TargetIndex   int
	CurrentIndex  int
	BlocksFetched int
	BlocksFailed  int
	StartTime     time.Time
}

// BlockRequest represents a request for a specific block
type BlockRequest struct {
	Index     int    `json:"index"`
	NodeID    string `json:"node_id"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"` // TODO: Add signature validation
}

// BlockResponse represents a response with a requested block
type BlockResponse struct {
	Index     int    `json:"index"`
	Block     *Block `json:"block"`
	NodeID    string `json:"node_id"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"` // TODO: Add signature validation
}

// NewSyncManager creates a new sync manager
func NewSyncManager(blockchain BlockchainInterface, nodeID string, config *ConsensusConfig) *SyncManager {
	return &SyncManager{
		blockchain: blockchain,
		nodeID:     nodeID,
		config:     config,
		stopChan:   make(chan struct{}),
	}
}

// SetNetworkCallbacks sets the network communication callbacks
func (sm *SyncManager) SetNetworkCallbacks(
	onBlockRequest func(*BlockRequest) error,
	onBlockResponse func(*BlockResponse) error,
) {
	sm.onBlockRequest = onBlockRequest
	sm.onBlockResponse = onBlockResponse
}

// Start starts the sync manager
func (sm *SyncManager) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.isRunning {
		return fmt.Errorf("sync manager already running")
	}

	sm.isRunning = true

	// Start background sync checker
	go sm.syncChecker()

	log.Printf("[SyncManager] Started sync manager for node %s", sm.nodeID)
	return nil
}

// Stop stops the sync manager
func (sm *SyncManager) Stop() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.isRunning {
		return nil
	}

	sm.isRunning = false
	close(sm.stopChan)

	log.Printf("[SyncManager] Stopped sync manager for node %s", sm.nodeID)
	return nil
}

// CheckAndSyncIfNeeded checks if the node needs to sync and performs sync if necessary
func (sm *SyncManager) CheckAndSyncIfNeeded() error {
	sm.mu.Lock()
	if sm.isSyncing {
		sm.mu.Unlock()
		return fmt.Errorf("sync already in progress")
	}
	sm.isSyncing = true
	sm.mu.Unlock()

	defer func() {
		sm.mu.Lock()
		sm.isSyncing = false
		sm.mu.Unlock()
	}()

	// Step 1: Check current chain status
	myTip, err := sm.getMyChainTip()
	if err != nil {
		return fmt.Errorf("failed to get local chain tip: %w", err)
	}

	// Step 2: Query peers for their latest block height
	peerTip, err := sm.queryPeerChainTips()
	if err != nil {
		return fmt.Errorf("failed to query peer chain tips: %w", err)
	}

	// Step 3: Check if we need to sync
	if peerTip <= myTip {
		log.Printf("[SyncManager] Node is up to date: local tip %d, peer tip %d", myTip, peerTip)
		return nil
	}

	// Step 4: Perform sync
	log.Printf("[SyncManager] Starting sync: local tip %d, peer tip %d", myTip, peerTip)
	return sm.performBlockSync(myTip+1, peerTip)
}

// getMyChainTip gets the current chain tip (latest block index)
func (sm *SyncManager) getMyChainTip() (int, error) {
	chainLength, err := sm.blockchain.GetChainLength()
	if err != nil {
		return -1, fmt.Errorf("failed to get chain length: %w", err)
	}
	return chainLength - 1, nil // Convert length to index
}

// queryPeerChainTips queries multiple peers for their latest block height
func (sm *SyncManager) queryPeerChainTips() (int, error) {
	// TODO: Implement peer querying
	// For now, return a placeholder value
	// In real implementation, this would:
	// 1. Get list of connected peers
	// 2. Send chain tip requests to multiple peers
	// 3. Return the highest tip received
	return 0, nil
}

// performBlockSync performs the actual block synchronization
func (sm *SyncManager) performBlockSync(fromIndex, toIndex int) error {
	sm.syncProgress = &SyncProgress{
		StartIndex:   fromIndex,
		TargetIndex:  toIndex,
		CurrentIndex: fromIndex,
		StartTime:    time.Now(),
	}

	log.Printf("[SyncManager] Starting block sync from %d to %d", fromIndex, toIndex)

	// Step 2: Block fetch loop
	for sm.syncProgress.CurrentIndex <= toIndex {
		select {
		case <-sm.stopChan:
			return fmt.Errorf("sync interrupted")
		default:
			// Continue sync
		}

		// Request block
		block, err := sm.requestBlock(sm.syncProgress.CurrentIndex)
		if err != nil {
			sm.syncProgress.BlocksFailed++
			log.Printf("[SyncManager] Failed to fetch block %d: %v", sm.syncProgress.CurrentIndex, err)

			// Retry logic
			if sm.syncProgress.BlocksFailed > 3 {
				return fmt.Errorf("too many failed block requests")
			}
			time.Sleep(1 * time.Second)
			continue
		}

		// Step 3: Validation and integration
		if err := sm.validateAndIntegrateBlock(block); err != nil {
			sm.syncProgress.BlocksFailed++
			log.Printf("[SyncManager] Failed to validate block %d: %v", sm.syncProgress.CurrentIndex, err)
			continue
		}

		// Success
		sm.syncProgress.BlocksFetched++
		sm.syncProgress.CurrentIndex++

		// Log progress
		if sm.syncProgress.BlocksFetched%10 == 0 || sm.syncProgress.CurrentIndex > toIndex {
			progress := float64(sm.syncProgress.BlocksFetched) / float64(toIndex-fromIndex+1) * 100
			log.Printf("[SyncManager] Sync progress: %d/%d (%.1f%%)",
				sm.syncProgress.BlocksFetched, toIndex-fromIndex+1, progress)
		}
	}

	// Step 4: Sync complete
	duration := time.Since(sm.syncProgress.StartTime)
	log.Printf("[SyncManager] Sync completed: %d blocks in %v",
		sm.syncProgress.BlocksFetched, duration)

	return nil
}

// requestBlock requests a specific block from peers
func (sm *SyncManager) requestBlock(index int) (*Block, error) {
	// Create block request
	request := &BlockRequest{
		Index:     index,
		NodeID:    sm.nodeID,
		Timestamp: time.Now().Unix(),
		Signature: "", // TODO: Add signature
	}

	// Send request to network
	if sm.onBlockRequest != nil {
		if err := sm.onBlockRequest(request); err != nil {
			return nil, fmt.Errorf("failed to send block request: %w", err)
		}
	}

	// TODO: Wait for response with timeout
	// For now, return a placeholder
	// In real implementation, this would:
	// 1. Send request to multiple peers
	// 2. Wait for responses with timeout
	// 3. Return the first valid block received

	return nil, fmt.Errorf("block request not yet implemented")
}

// validateAndIntegrateBlock validates and integrates a received block
func (sm *SyncManager) validateAndIntegrateBlock(block *Block) error {
	// Validate block structure
	if err := block.ValidateBlock(); err != nil {
		return fmt.Errorf("invalid block structure: %w", err)
	}

	// Check if block already exists
	existingBlock, err := sm.blockchain.GetBlockByIndex(block.Index)
	if err == nil && existingBlock != nil {
		if existingBlock.Hash == block.Hash {
			// Block already exists and is the same
			return nil
		} else {
			// Block exists but is different - this shouldn't happen in consensus
			return fmt.Errorf("block index %d already exists with different hash", block.Index)
		}
	}

	// Validate block index continuity
	if block.Index > 0 {
		prevBlock, err := sm.blockchain.GetBlockByIndex(block.Index - 1)
		if err != nil {
			return fmt.Errorf("missing previous block %d: %w", block.Index-1, err)
		}
		if block.PrevHash != prevBlock.Hash {
			return fmt.Errorf("previous hash mismatch at block %d", block.Index)
		}
	}

	// Save block to blockchain
	if err := sm.blockchain.SaveBlock(block); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}

	log.Printf("[SyncManager] Integrated block %d with %d posts", block.Index, len(block.Posts))
	return nil
}

// HandleBlockRequest handles an incoming block request from another node
func (sm *SyncManager) HandleBlockRequest(request *BlockRequest) error {
	// Validate request
	if request.Index < 0 {
		return fmt.Errorf("invalid block index: %d", request.Index)
	}

	// Get the requested block
	block, err := sm.blockchain.GetBlockByIndex(request.Index)
	if err != nil {
		return fmt.Errorf("block %d not found: %w", request.Index, err)
	}

	// Create response
	response := &BlockResponse{
		Index:     request.Index,
		Block:     block,
		NodeID:    sm.nodeID,
		Timestamp: time.Now().Unix(),
		Signature: "", // TODO: Add signature
	}

	// Send response to network
	if sm.onBlockResponse != nil {
		if err := sm.onBlockResponse(response); err != nil {
			return fmt.Errorf("failed to send block response: %w", err)
		}
	}

	log.Printf("[SyncManager] Sent block %d to %s", request.Index, request.NodeID)
	return nil
}

// HandleBlockResponse handles an incoming block response from another node
func (sm *SyncManager) HandleBlockResponse(response *BlockResponse) error {
	// Validate response
	if response.Block == nil {
		return fmt.Errorf("empty block in response")
	}

	if response.Index != response.Block.Index {
		return fmt.Errorf("block index mismatch in response")
	}

	// Process the block if we're currently syncing
	sm.mu.RLock()
	isSyncing := sm.isSyncing
	sm.mu.RUnlock()

	if isSyncing {
		// TODO: Send block to sync process
		log.Printf("[SyncManager] Received block %d from %s", response.Index, response.NodeID)
	}

	return nil
}

// syncChecker periodically checks if the node needs to sync
func (sm *SyncManager) syncChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stopChan:
			return
		case <-ticker.C:
			// Check if we need to sync
			sm.mu.RLock()
			isSyncing := sm.isSyncing
			sm.mu.RUnlock()

			if !isSyncing {
				if err := sm.CheckAndSyncIfNeeded(); err != nil {
					log.Printf("[SyncManager] Sync check failed: %v", err)
				}
			}
		}
	}
}

// GetSyncStatus returns the current sync status
func (sm *SyncManager) GetSyncStatus() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	status := map[string]interface{}{
		"is_running": sm.isRunning,
		"is_syncing": sm.isSyncing,
	}

	if sm.syncProgress != nil {
		status["sync_progress"] = map[string]interface{}{
			"start_index":    sm.syncProgress.StartIndex,
			"target_index":   sm.syncProgress.TargetIndex,
			"current_index":  sm.syncProgress.CurrentIndex,
			"blocks_fetched": sm.syncProgress.BlocksFetched,
			"blocks_failed":  sm.syncProgress.BlocksFailed,
			"start_time":     sm.syncProgress.StartTime,
		}
	}

	return status
}
