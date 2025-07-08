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
	onBlockRequest     func(*BlockRequest) error
	onBlockResponse    func(*BlockResponse) error
	onChainTipQuery    func(*ChainTipQuery) error
	onChainTipResponse func(*ChainTipResponse) error

	// Peer query interface
	peerQueryProvider func() ([]string, error) // Returns list of connected peer addresses

	// State
	isRunning bool
	isSyncing bool
	mu        sync.RWMutex
	stopChan  chan struct{}

	// Sync state
	syncStartTime time.Time
	syncProgress  *SyncProgress

	// Block request tracking
	pendingBlockResponses map[int]chan *Block
	responseTimeout       time.Duration

	// Chain tip query tracking
	pendingChainTipResponses map[string]chan *ChainTipResponse
	chainTipQueryTimeout     time.Duration
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

// ChainTipQuery represents a query for a peer's latest block height
type ChainTipQuery struct {
	NodeID    string `json:"node_id"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"` // TODO: Add signature validation
}

// ChainTipResponse represents a response to a chain tip query
type ChainTipResponse struct {
	NodeID    string `json:"node_id"`
	ChainTip  int    `json:"chain_tip"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"` // TODO: Add signature validation
}

// NewSyncManager creates a new sync manager
func NewSyncManager(blockchain BlockchainInterface, nodeID string, config *ConsensusConfig) *SyncManager {
	return &SyncManager{
		blockchain:               blockchain,
		nodeID:                   nodeID,
		config:                   config,
		stopChan:                 make(chan struct{}),
		pendingBlockResponses:    make(map[int]chan *Block),
		responseTimeout:          30 * time.Second, // 30 second timeout for block requests
		pendingChainTipResponses: make(map[string]chan *ChainTipResponse),
		chainTipQueryTimeout:     10 * time.Second, // 10 second timeout for chain tip queries
	}
}

// SetNetworkCallbacks sets the network communication callbacks
func (sm *SyncManager) SetNetworkCallbacks(
	onBlockRequest func(*BlockRequest) error,
	onBlockResponse func(*BlockResponse) error,
	onChainTipQuery func(*ChainTipQuery) error,
	onChainTipResponse func(*ChainTipResponse) error,
) {
	sm.onBlockRequest = onBlockRequest
	sm.onBlockResponse = onBlockResponse
	sm.onChainTipQuery = onChainTipQuery
	sm.onChainTipResponse = onChainTipResponse
}

// SetPeerQueryProvider sets the function to query connected peers
func (sm *SyncManager) SetPeerQueryProvider(provider func() ([]string, error)) {
	sm.peerQueryProvider = provider
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

	// Clear all pending block responses
	for index, responseChan := range sm.pendingBlockResponses {
		select {
		case responseChan <- nil: // Send nil to unblock waiting requests
		default:
			// Channel might be full, just close it
		}
		close(responseChan)
		delete(sm.pendingBlockResponses, index)
	}

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

	// Step 4: Check if we have peers available for sync
	if peerTip == 0 {
		log.Printf("[SyncManager] No peers available for sync - will retry when peers connect")
		return nil
	}

	// Step 5: Perform sync
	log.Printf("[SyncManager] Starting sync: local tip %d, peer tip %d", myTip, peerTip)
	return sm.performBlockSync(myTip+1, peerTip)
}

// getMyChainTip gets the current chain tip (latest block index)
func (sm *SyncManager) getMyChainTip() (int, error) {
	chainLength, err := sm.blockchain.GetChainLength()
	if err != nil {
		return -1, fmt.Errorf("failed to get chain length: %w", err)
	}
	chainTip := chainLength - 1 // Convert length to index
	log.Printf("[SyncManager] Chain length: %d, calculated chain tip: %d", chainLength, chainTip)
	return chainTip, nil
}

// queryPeerChainTips queries multiple peers for their latest block height
func (sm *SyncManager) queryPeerChainTips() (int, error) {
	// Check if we have a peer query provider
	if sm.peerQueryProvider == nil {
		log.Printf("[SyncManager] No peer query provider available - returning 0")
		return 0, nil
	}

	// Get connected peers
	peers, err := sm.peerQueryProvider()
	if err != nil {
		log.Printf("[SyncManager] Failed to get peers: %v", err)
		return 0, nil
	}

	if len(peers) == 0 {
		log.Printf("[SyncManager] No connected peers available for chain tip query - returning 0")
		return 0, nil
	}

	log.Printf("[SyncManager] Querying %d peers for chain tips", len(peers))

	// Query each peer for their chain tip
	responses := make(chan *ChainTipResponse, len(peers))
	queryID := fmt.Sprintf("tip_query_%d", time.Now().UnixNano())

	// Create response channel for this query
	sm.mu.Lock()
	sm.pendingChainTipResponses[queryID] = responses
	sm.mu.Unlock()

	// Cleanup function
	defer func() {
		sm.mu.Lock()
		delete(sm.pendingChainTipResponses, queryID)
		sm.mu.Unlock()
		close(responses)
	}()

	// Send chain tip queries to all peers
	query := &ChainTipQuery{
		NodeID:    sm.nodeID,
		Timestamp: time.Now().Unix(),
		Signature: "", // TODO: Add signature
	}

	if sm.onChainTipQuery != nil {
		if err := sm.onChainTipQuery(query); err != nil {
			log.Printf("[SyncManager] Network not ready for chain tip query (will retry): %v", err)
		}
	} else {
		log.Printf("[SyncManager] Chain tip query callback not set up yet (will retry)")
	}

	// Wait for responses with timeout
	highestTip := -1
	responseCount := 0

	timeout := time.After(sm.chainTipQueryTimeout)

	for {
		select {
		case response := <-responses:
			if response != nil {
				responseCount++
				if response.ChainTip > highestTip {
					highestTip = response.ChainTip
				}
				log.Printf("[SyncManager] Received chain tip response: %d from %s (count: %d)",
					response.ChainTip, response.NodeID, responseCount)

				// If we've received responses from most peers, we can proceed
				if responseCount >= len(peers)/2 {
					log.Printf("[SyncManager] Received responses from %d/%d peers, highest tip: %d",
						responseCount, len(peers), highestTip)
					return highestTip, nil
				}
			}
		case <-timeout:
			if responseCount > 0 {
				log.Printf("[SyncManager] Timeout reached, received %d responses, highest tip: %d",
					responseCount, highestTip)
				return highestTip, nil
			}
			log.Printf("[SyncManager] Timeout waiting for chain tip responses")
			break
		case <-sm.stopChan:
			return 0, fmt.Errorf("sync interrupted during chain tip query")
		}
	}

	// If no responses received, return conservative estimate
	myTip, err := sm.getMyChainTip()
	if err != nil {
		log.Printf("[SyncManager] Failed to get own chain tip: %v", err)
		return 0, nil
	}

	// Assume peers might be ahead by 1 block (conservative estimate)
	estimatedPeerTip := myTip + 1
	log.Printf("[SyncManager] No chain tip responses, using estimate: %d (my tip: %d)", estimatedPeerTip, myTip)
	return estimatedPeerTip, nil
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
	maxRetries := 3
	retryDelay := 1 * time.Second

	for sm.syncProgress.CurrentIndex <= toIndex {
		select {
		case <-sm.stopChan:
			return fmt.Errorf("sync interrupted")
		default:
			// Continue sync
		}

		// Request block with retries
		var block *Block
		var err error
		retries := 0

		for retries < maxRetries {
			block, err = sm.requestBlock(sm.syncProgress.CurrentIndex)
			if err == nil {
				break // Success
			}

			retries++
			sm.syncProgress.BlocksFailed++
			log.Printf("[SyncManager] Failed to fetch block %d (attempt %d/%d): %v",
				sm.syncProgress.CurrentIndex, retries, maxRetries, err)

			if retries < maxRetries {
				time.Sleep(retryDelay)
				retryDelay *= 2 // Exponential backoff
			}
		}

		if err != nil {
			return fmt.Errorf("failed to fetch block %d after %d attempts: %w",
				sm.syncProgress.CurrentIndex, maxRetries, err)
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
		Signature: "", // TODO: Add signature validation
	}

	// Create response channel
	responseChan := make(chan *Block, 1)

	// Register pending response
	sm.mu.Lock()
	sm.pendingBlockResponses[index] = responseChan
	sm.mu.Unlock()

	// Cleanup function to remove from pending map
	defer func() {
		sm.mu.Lock()
		delete(sm.pendingBlockResponses, index)
		sm.mu.Unlock()
	}()

	// Send request to network
	if sm.onBlockRequest != nil {
		if err := sm.onBlockRequest(request); err != nil {
			return nil, fmt.Errorf("failed to send block request: %w", err)
		}
	} else {
		return nil, fmt.Errorf("block request callback not set up yet (will retry)")
	}

	// Wait for response with timeout
	select {
	case block := <-responseChan:
		if block == nil {
			return nil, fmt.Errorf("received nil block for index %d", index)
		}
		return block, nil
	case <-time.After(sm.responseTimeout):
		return nil, fmt.Errorf("timeout waiting for block %d", index)
	case <-sm.stopChan:
		return nil, fmt.Errorf("sync interrupted while waiting for block %d", index)
	}
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

	// Check if we have a pending request for this block
	sm.mu.Lock()
	responseChan, exists := sm.pendingBlockResponses[response.Index]
	sm.mu.Unlock()

	if exists {
		// Send block to waiting request
		select {
		case responseChan <- response.Block:
			log.Printf("[SyncManager] Delivered block %d to waiting request", response.Index)
		default:
			log.Printf("[SyncManager] Response channel full for block %d", response.Index)
		}
	} else {
		// No pending request - this might be an unsolicited response
		log.Printf("[SyncManager] Received unsolicited block %d from %s", response.Index, response.NodeID)
	}

	return nil
}

// HandleChainTipQuery handles an incoming chain tip query from another node
func (sm *SyncManager) HandleChainTipQuery(query *ChainTipQuery) error {
	// Get our current chain tip
	chainTip, err := sm.getMyChainTip()
	if err != nil {
		return fmt.Errorf("failed to get chain tip: %w", err)
	}

	// Create response
	response := &ChainTipResponse{
		NodeID:    sm.nodeID,
		ChainTip:  chainTip,
		Timestamp: time.Now().Unix(),
		Signature: "", // TODO: Add signature
	}

	// Send response to network
	if sm.onChainTipResponse != nil {
		if err := sm.onChainTipResponse(response); err != nil {
			return fmt.Errorf("failed to send chain tip response: %w", err)
		}
	}

	log.Printf("[SyncManager] Sent chain tip %d to %s (node: %s)", chainTip, query.NodeID, sm.nodeID)
	return nil
}

// HandleChainTipResponse handles an incoming chain tip response from another node
func (sm *SyncManager) HandleChainTipResponse(response *ChainTipResponse) error {
	// Validate response
	if response.ChainTip < 0 {
		return fmt.Errorf("invalid chain tip in response: %d", response.ChainTip)
	}

	// Check if we have any pending chain tip queries
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find the first pending query and send the response
	for queryID, responseChan := range sm.pendingChainTipResponses {
		select {
		case responseChan <- response:
			log.Printf("[SyncManager] Delivered chain tip %d to waiting query %s", response.ChainTip, queryID)
			return nil
		default:
			// Channel full, try next one
			continue
		}
	}

	// No pending queries - this might be an unsolicited response
	log.Printf("[SyncManager] Received unsolicited chain tip %d from %s", response.ChainTip, response.NodeID)
	return nil
}

// syncChecker periodically checks if the node needs to sync
func (sm *SyncManager) syncChecker() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
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

// NotifyPeersAvailable can be called when peers become available
func (sm *SyncManager) NotifyPeersAvailable() {
	log.Printf("[SyncManager] Peers became available - triggering sync check")

	// Trigger an immediate sync check
	go func() {
		if err := sm.CheckAndSyncIfNeeded(); err != nil {
			log.Printf("[SyncManager] Sync check failed after peer notification: %v", err)
		}
	}()
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
		// Calculate progress percentage
		totalBlocks := sm.syncProgress.TargetIndex - sm.syncProgress.StartIndex + 1
		progressPercent := 0.0
		if totalBlocks > 0 {
			progressPercent = float64(sm.syncProgress.BlocksFetched) / float64(totalBlocks) * 100
		}

		// Calculate duration
		duration := time.Since(sm.syncProgress.StartTime)

		// Calculate rate
		blocksPerSecond := 0.0
		if duration.Seconds() > 0 {
			blocksPerSecond = float64(sm.syncProgress.BlocksFetched) / duration.Seconds()
		}

		status["sync_progress"] = map[string]interface{}{
			"start_index":       sm.syncProgress.StartIndex,
			"target_index":      sm.syncProgress.TargetIndex,
			"current_index":     sm.syncProgress.CurrentIndex,
			"blocks_fetched":    sm.syncProgress.BlocksFetched,
			"blocks_failed":     sm.syncProgress.BlocksFailed,
			"progress_percent":  progressPercent,
			"blocks_per_second": blocksPerSecond,
			"duration":          duration.String(),
			"start_time":        sm.syncProgress.StartTime,
		}
	}

	// Add pending requests info
	status["pending_block_requests"] = len(sm.pendingBlockResponses)
	status["pending_chain_tip_queries"] = len(sm.pendingChainTipResponses)
	status["response_timeout"] = sm.responseTimeout.Seconds()
	status["chain_tip_query_timeout"] = sm.chainTipQueryTimeout.Seconds()

	return status
}
