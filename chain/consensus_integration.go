package chain

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// BlockchainInterface defines the interface for blockchain operations
type BlockchainInterface interface {
	GetLatestBlock() (*Block, error)
	GetBlockByIndex(index int) (*Block, error)
	GetChainLength() (int, error)
	ValidateBlock(block *Block) error
	SaveBlock(block *Block) error
}

// ConsensusIntegration integrates the consensus system with the blockchain
type ConsensusIntegration struct {
	blockchain      BlockchainInterface
	consensusEngine *ConsensusEngine
	blockBuilder    *BlockBuilder
	syncManager     *SyncManager

	// Configuration
	config *ConsensusConfig
	nodeID string

	// State
	isRunning bool
	mu        sync.RWMutex

	// Channels
	stopChan  chan struct{}
	blockChan chan *Block

	// Node introduction tracking
	peerIntroductions map[string]*NodeIntroduction
	introMutex        sync.RWMutex

	// Time-based block consensus
	timeBasedBlockRequests  map[string]*TimeBasedBlockRequest
	timeBasedBlockVotes     map[string][]*TimeBasedBlockVote
	timeBasedBlockApprovals map[string]*TimeBasedBlockApproval
	timeBasedBlockMutex     sync.RWMutex

	// Network callbacks for time-based block consensus
	onTimeBasedBlockRequest func(*TimeBasedBlockRequest) error
	onTimeBasedBlockVote    func(*TimeBasedBlockVote) error
}

// NewConsensusIntegration creates a new consensus integration
func NewConsensusIntegration(
	blockchain BlockchainInterface,
	nodeID string,
	config *ConsensusConfig,
) *ConsensusIntegration {

	// Create consensus engine
	consensusEngine := NewConsensusEngine(nodeID, config)

	// Create block builder
	blockBuilder := NewBlockBuilder(consensusEngine, config.PostThreshold, 10*time.Minute)

	// Create sync manager
	syncManager := NewSyncManager(blockchain, nodeID, config)

	ci := &ConsensusIntegration{
		blockchain:              blockchain,
		consensusEngine:         consensusEngine,
		blockBuilder:            blockBuilder,
		syncManager:             syncManager,
		config:                  config,
		nodeID:                  nodeID,
		stopChan:                make(chan struct{}),
		blockChan:               make(chan *Block, 50),
		peerIntroductions:       make(map[string]*NodeIntroduction),
		timeBasedBlockRequests:  make(map[string]*TimeBasedBlockRequest),
		timeBasedBlockVotes:     make(map[string][]*TimeBasedBlockVote),
		timeBasedBlockApprovals: make(map[string]*TimeBasedBlockApproval),
	}

	// Set up callbacks
	consensusEngine.SetNetworkCallbacks(
		ci.handlePostGossipOutbound,
		ci.handleBlockProposalOutbound,
		ci.handleBlockVoteOutbound,
		ci.handleBlockCreatedOutbound,
		ci.handleProposalExpiredOutbound,
	)

	// Set up block index provider
	consensusEngine.SetBlockIndexProvider(func() int {
		latestBlock, err := ci.blockchain.GetLatestBlock()
		if err != nil {
			return 0
		}
		return latestBlock.Index + 1
	})

	// Set up peer query provider (will be set later when network is available)
	// This will be set in the main.go when the network is initialized

	return ci
}

// Start starts the consensus integration
func (ci *ConsensusIntegration) Start() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if ci.isRunning {
		return fmt.Errorf("consensus integration already running")
	}

	ci.isRunning = true

	// Step 1: Check and sync if needed before starting consensus
	if err := ci.syncManager.CheckAndSyncIfNeeded(); err != nil {
		log.Printf("[ConsensusIntegration] Initial sync failed: %v", err)
		// Continue anyway - sync will retry in background
	} else {
		log.Printf("[ConsensusIntegration] Initial sync check completed")
	}

	// Start sync manager
	if err := ci.syncManager.Start(); err != nil {
		return fmt.Errorf("failed to start sync manager: %w", err)
	}

	// Start consensus engine
	if err := ci.consensusEngine.Start(); err != nil {
		return fmt.Errorf("failed to start consensus engine: %w", err)
	}

	// Start background workers
	go ci.blockProcessor()
	go ci.timeBasedBlockWorker()
	go ci.reservationMonitor()

	log.Printf("[ConsensusIntegration] Started consensus integration for node %s", ci.nodeID)
	return nil
}

// Stop stops the consensus integration
func (ci *ConsensusIntegration) Stop() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if !ci.isRunning {
		return nil
	}

	ci.isRunning = false
	close(ci.stopChan)

	// Stop sync manager
	if err := ci.syncManager.Stop(); err != nil {
		log.Printf("[ConsensusIntegration] Error stopping sync manager: %v", err)
	}

	// Stop consensus engine
	if err := ci.consensusEngine.Stop(); err != nil {
		log.Printf("[ConsensusIntegration] Error stopping consensus engine: %v", err)
	}

	log.Printf("[ConsensusIntegration] Stopped consensus integration for node %s", ci.nodeID)
	return nil
}

// AddPost adds a post to the consensus system
func (ci *ConsensusIntegration) AddPost(post Post) error {
	return ci.consensusEngine.AddPost(post)
}

// HandlePostGossip handles a post gossip from the network
func (ci *ConsensusIntegration) HandlePostGossip(gossip *PostGossip) error {
	return ci.consensusEngine.HandlePostGossip(gossip)
}

// HandleBlockProposal handles a block proposal from the network
func (ci *ConsensusIntegration) HandleBlockProposal(proposal *BlockProposal) error {
	return ci.consensusEngine.HandleBlockProposal(proposal)
}

// HandleBlockVote handles a block vote from the network
func (ci *ConsensusIntegration) HandleBlockVote(vote *BlockVote) error {
	return ci.consensusEngine.HandleBlockVote(vote)
}

// HandleBlockCreated handles a block that was created by another node
func (ci *ConsensusIntegration) HandleBlockCreated(block *Block) error {
	// Validate and integrate the block
	if err := ci.validateAndIntegrateBlock(block); err != nil {
		return fmt.Errorf("failed to validate and integrate block: %w", err)
	}

	// Notify consensus engine
	return ci.consensusEngine.HandleBlockCreated(block)
}

// HandleProposalExpired handles a proposal expiration from the network
func (ci *ConsensusIntegration) HandleProposalExpired(expired *ProposalExpired) error {
	log.Printf("[ConsensusIntegration] Proposal expired for block %d from %s", expired.Index, expired.ProposerID)
	return nil
}

// Network callback handlers

func (ci *ConsensusIntegration) handlePostGossipOutbound(gossip *PostGossip) error {
	// This would send the gossip to the network layer
	log.Printf("[ConsensusIntegration] Gossiping post: %s", gossip.Post.Hash[:8])
	return nil
}

func (ci *ConsensusIntegration) handleBlockProposalOutbound(proposal *BlockProposal) error {
	// This would send the proposal to the network layer
	log.Printf("[ConsensusIntegration] Broadcasting block proposal for block %d", proposal.Index)
	return nil
}

func (ci *ConsensusIntegration) handleBlockVoteOutbound(vote *BlockVote) error {
	// This would send the vote to the network layer
	log.Printf("[ConsensusIntegration] Broadcasting vote for block %d: %t", vote.Index, vote.Approved)
	return nil
}

func (ci *ConsensusIntegration) handleBlockCreatedOutbound(block *Block) error {
	// This would send the block to the network layer
	log.Printf("[ConsensusIntegration] Broadcasting block %d", block.Index)
	return nil
}

func (ci *ConsensusIntegration) handleProposalExpiredOutbound(expired *ProposalExpired) error {
	// This would send the expiration to the network layer
	log.Printf("[ConsensusIntegration] Broadcasting proposal expired for block %d", expired.Index)
	return nil
}

// Background workers

func (ci *ConsensusIntegration) blockProcessor() {
	for {
		select {
		case <-ci.stopChan:
			return
		case block := <-ci.blockChan:
			if err := ci.processBlock(block); err != nil {
				log.Printf("[ConsensusIntegration] Failed to process block: %v", err)
			}
		}
	}
}

func (ci *ConsensusIntegration) timeBasedBlockWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ci.stopChan:
			return
		case <-ticker.C:
			ci.checkTimeBasedBlock()
		}
	}
}

func (ci *ConsensusIntegration) reservationMonitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ci.stopChan:
			return
		case <-ticker.C:
			ci.checkReservations()
		}
	}
}

// checkTimeBasedBlock checks if a time-based block should be created
func (ci *ConsensusIntegration) checkTimeBasedBlock() {
	// Get latest block to check timing
	latestBlock, err := ci.blockchain.GetLatestBlock()
	if err != nil {
		if err.Error() == "no blocks found" {
			log.Printf("[ConsensusIntegration] No blocks found yet - cannot check time-based blocks")
			return
		}
		log.Printf("[ConsensusIntegration] Failed to get latest block for time-based check: %v", err)
		return
	}

	// Check if enough time has passed since last block (10 minutes)
	timeSinceLastBlock := time.Since(time.Unix(latestBlock.Timestamp, 0))
	if timeSinceLastBlock < 10*time.Minute {
		log.Printf("[ConsensusIntegration] Only %v since last block - too soon for time-based block", timeSinceLastBlock)
		return
	}

	// Check if we have pending posts - if so, don't create time-based block
	postCount := ci.consensusEngine.mempool.GetPostCount()
	if postCount > 0 {
		log.Printf("[ConsensusIntegration] Have %d pending posts - should create content block instead", postCount)
		return
	}

	// Check if there's already a pending time-based block request for this index
	nextIndex := latestBlock.Index + 1
	requestID := fmt.Sprintf("time_block_%d_%s", nextIndex, ci.nodeID)

	ci.timeBasedBlockMutex.RLock()
	_, exists := ci.timeBasedBlockRequests[requestID]
	ci.timeBasedBlockMutex.RUnlock()

	if exists {
		log.Printf("[ConsensusIntegration] Already have pending time-based block request for block %d", nextIndex)
		return
	}

	// Check if there's already a block proposal for this index
	if _, exists := ci.consensusEngine.proposalManager.GetReservation(nextIndex); exists {
		log.Printf("[ConsensusIntegration] Already have block proposal for block %d", nextIndex)
		return
	}

	// All conditions met - request time-based block consensus
	log.Printf("[ConsensusIntegration] Requesting time-based block %d consensus (no posts, %v since last block)",
		nextIndex, timeSinceLastBlock)

	if err := ci.RequestTimeBasedBlock(); err != nil {
		log.Printf("[ConsensusIntegration] Failed to request time-based block: %v", err)
	}
}

// checkReservations checks for approved reservations and creates blocks
func (ci *ConsensusIntegration) checkReservations() {
	reservations := ci.consensusEngine.proposalManager.GetActiveReservations()

	for _, reservation := range reservations {
		// Check if this is our reservation
		if reservation.Proposer == ci.nodeID {
			ci.processOurReservation(reservation)
		}

		// Check for expired reservations
		if time.Now().After(reservation.TimeoutAt) {
			ci.handleExpiredReservation(reservation)
		}
	}
}

// handleExpiredReservation handles an expired block reservation
func (ci *ConsensusIntegration) handleExpiredReservation(reservation *BlockReservation) {
	log.Printf("[ConsensusIntegration] Block %d reservation expired for proposer %s",
		reservation.Index, reservation.Proposer)

	// Decrease trust score for failed proposer
	ci.consensusEngine.trustManager.OnProposalFailure(reservation.Proposer)

	// Remove the reservation
	ci.consensusEngine.proposalManager.RemoveReservation(reservation.Index)

	// Broadcast proposal expired message
	expiredMsg := &ProposalExpired{
		Index:      reservation.Index,
		ProposerID: reservation.Proposer,
		Timestamp:  time.Now().Unix(),
		Signature:  "", // TODO: Sign the message
	}

	if err := ci.handleProposalExpiredOutbound(expiredMsg); err != nil {
		log.Printf("[ConsensusIntegration] Failed to broadcast expired message: %v", err)
	}
}

// processOurReservation processes a reservation that we proposed
func (ci *ConsensusIntegration) processOurReservation(reservation *BlockReservation) {
	// Get current blockchain state
	latestBlock, err := ci.blockchain.GetLatestBlock()
	if err != nil {
		// Check if this is because there are no blocks yet
		if err.Error() == "no blocks found" {
			log.Printf("[ConsensusIntegration] No blocks found yet - cannot process reservation for block %d", reservation.Index)
			return
		}
		log.Printf("[ConsensusIntegration] Failed to get latest block: %v", err)
		return
	}

	// Check if we can build the block
	if latestBlock.Index+1 != reservation.Index {
		log.Printf("[ConsensusIntegration] Block index mismatch: expected %d, got %d",
			latestBlock.Index+1, reservation.Index)
		return
	}

	// Check if reservation is still valid
	if time.Now().After(reservation.TimeoutAt) {
		log.Printf("[ConsensusIntegration] Our reservation for block %d has expired", reservation.Index)
		ci.handleExpiredReservation(reservation)
		return
	}

	// Build the block
	block, err := ci.blockBuilder.BuildBlockFromProposal(reservation, latestBlock.Hash, latestBlock.StateRoot)
	if err != nil {
		log.Printf("[ConsensusIntegration] Failed to build block from reservation: %v", err)
		return
	}

	// Process the block
	ci.blockChan <- block
}

// processBlock processes a block (either created by us or received from network)
func (ci *ConsensusIntegration) processBlock(block *Block) error {
	// Validate and integrate the block
	if err := ci.validateAndIntegrateBlock(block); err != nil {
		return fmt.Errorf("failed to validate and integrate block: %w", err)
	}

	// Broadcast the block to network
	if err := ci.handleBlockCreatedOutbound(block); err != nil {
		log.Printf("[ConsensusIntegration] Failed to broadcast block: %v", err)
	}

	// Handle mempool cleanup and trust score updates
	ci.handleBlockAccepted(block)

	return nil
}

// handleBlockAccepted handles a block that was accepted into the blockchain
func (ci *ConsensusIntegration) handleBlockAccepted(block *Block) {
	// Remove posts from mempool (this is done in consensus engine)
	ci.consensusEngine.HandleBlockCreated(block)

	// Increase trust score if we created the block
	if block.Index > 0 { // Skip genesis block
		latestBlock, err := ci.blockchain.GetLatestBlock()
		if err != nil {
			// Ignore "no blocks found" error as it might happen during startup
			if err.Error() != "no blocks found" {
				log.Printf("[ConsensusIntegration] Failed to get latest block for trust score update: %v", err)
			}
			return
		}
		if latestBlock.Hash == block.Hash {
			// We successfully created this block
			ci.consensusEngine.trustManager.OnProposalSuccess(ci.nodeID)
			log.Printf("[ConsensusIntegration] Successfully created block %d, trust score increased", block.Index)
		}
	}

	log.Printf("[ConsensusIntegration] Block %d accepted with %d posts", block.Index, len(block.Posts))
}

// validateAndIntegrateBlock validates and integrates a block into the blockchain
func (ci *ConsensusIntegration) validateAndIntegrateBlock(block *Block) error {
	// Validate the block
	if err := ci.blockchain.ValidateBlock(block); err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}

	// Verify authorship of every post/transfer. Blocks arrive over gossip from
	// untrusted peers, so structural validation is not sufficient.
	if err := block.VerifySignatures(); err != nil {
		return fmt.Errorf("block signature verification failed: %w", err)
	}

	// Check if block already exists
	existingBlock, err := ci.blockchain.GetBlockByIndex(block.Index)
	if err == nil && existingBlock != nil {
		if existingBlock.Hash == block.Hash {
			// Block already exists and is the same
			return nil
		} else {
			// Block exists but is different - this shouldn't happen in consensus
			return fmt.Errorf("block index %d already exists with different hash", block.Index)
		}
	}

	// Enforce chain continuity so a peer cannot inject a disconnected block at an
	// arbitrary index and poison the chain tip/length. A non-genesis block must
	// extend an existing predecessor whose hash it references.
	if block.Index > 0 {
		prevBlock, err := ci.blockchain.GetBlockByIndex(block.Index - 1)
		if err != nil || prevBlock == nil {
			return fmt.Errorf("missing previous block %d for incoming block %d", block.Index-1, block.Index)
		}
		if block.PrevHash != prevBlock.Hash {
			return fmt.Errorf("previous hash mismatch at block %d: block references %s but local tip is %s",
				block.Index, block.PrevHash, prevBlock.Hash)
		}
	}

	// Save the block
	if err := ci.blockchain.SaveBlock(block); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}

	log.Printf("[ConsensusIntegration] Integrated block %d with %d posts", block.Index, len(block.Posts))

	return nil
}

// GetStats returns consensus integration statistics
func (ci *ConsensusIntegration) GetStats() map[string]interface{} {
	ci.mu.RLock()
	defer ci.mu.RUnlock()

	consensusStats := ci.consensusEngine.GetStats()
	blockBuilderStats := ci.blockBuilder.GetStats()

	// Get blockchain stats
	chainLength, _ := ci.blockchain.GetChainLength()
	pendingPosts := ci.consensusEngine.mempool.GetPostCount()

	return map[string]interface{}{
		"node_id":                  ci.nodeID,
		"is_running":               ci.isRunning,
		"chain_length":             chainLength,
		"pending_posts":            pendingPosts,
		"consensus":                consensusStats,
		"block_builder":            blockBuilderStats,
		"post_threshold":           ci.config.PostThreshold,
		"proposal_timeout_minutes": ci.config.ProposalTimeout.Minutes(),
		"min_trust_score":          ci.config.MinTrustScore,
	}
}

// GetMempoolInfo returns information about the post mempool
func (ci *ConsensusIntegration) GetMempoolInfo() map[string]interface{} {
	posts := ci.consensusEngine.mempool.GetPosts()

	// Calculate total characters
	totalChars := 0
	for _, post := range posts {
		totalChars += post.GetCharacterCount()
	}

	return map[string]interface{}{
		"post_count":      len(posts),
		"total_chars":     totalChars,
		"threshold":       ci.config.PostThreshold,
		"ready_for_block": ci.consensusEngine.mempool.IsReadyForBlock(ci.config.PostThreshold),
	}
}

// GetActiveProposals returns active block proposals
func (ci *ConsensusIntegration) GetActiveProposals() []*BlockProposal {
	return ci.consensusEngine.proposalManager.GetActiveProposals()
}

// GetActiveReservations returns active block reservations
func (ci *ConsensusIntegration) GetActiveReservations() []*BlockReservation {
	return ci.consensusEngine.proposalManager.GetActiveReservations()
}

// GetTrustScores returns all trust scores
func (ci *ConsensusIntegration) GetTrustScores() map[string]float64 {
	return ci.consensusEngine.trustManager.GetAllScores()
}

// ConsensusEngine returns the underlying consensus engine
func (ci *ConsensusIntegration) ConsensusEngine() *ConsensusEngine {
	return ci.consensusEngine
}

// SyncManager returns the sync manager
func (ci *ConsensusIntegration) SyncManager() *SyncManager {
	return ci.syncManager
}

// SetPeerQueryProvider sets the peer query provider for the sync manager
func (ci *ConsensusIntegration) SetPeerQueryProvider(provider func() ([]string, error)) {
	ci.syncManager.SetPeerQueryProvider(provider)
}

// SetSyncNetworkCallbacks sets the network callbacks for the sync manager
func (ci *ConsensusIntegration) SetSyncNetworkCallbacks(
	onBlockRequest func(*BlockRequest) error,
	onBlockResponse func(*BlockResponse) error,
	onChainTipQuery func(*ChainTipQuery) error,
	onChainTipResponse func(*ChainTipResponse) error,
) {
	ci.syncManager.SetNetworkCallbacks(onBlockRequest, onBlockResponse, onChainTipQuery, onChainTipResponse)
}

// SetTimeBasedBlockNetworkCallbacks sets the network callbacks for time-based block consensus
func (ci *ConsensusIntegration) SetTimeBasedBlockNetworkCallbacks(
	onTimeBasedBlockRequest func(*TimeBasedBlockRequest) error,
	onTimeBasedBlockVote func(*TimeBasedBlockVote) error,
) {
	ci.onTimeBasedBlockRequest = onTimeBasedBlockRequest
	ci.onTimeBasedBlockVote = onTimeBasedBlockVote
}

// HandleBlockRequest handles an incoming block request from the network
func (ci *ConsensusIntegration) HandleBlockRequest(request *BlockRequest) error {
	return ci.syncManager.HandleBlockRequest(request)
}

// HandleBlockResponse handles an incoming block response from the network
func (ci *ConsensusIntegration) HandleBlockResponse(response *BlockResponse) error {
	return ci.syncManager.HandleBlockResponse(response)
}

// HandleChainTipQuery handles an incoming chain tip query from the network
func (ci *ConsensusIntegration) HandleChainTipQuery(query *ChainTipQuery) error {
	return ci.syncManager.HandleChainTipQuery(query)
}

// HandleChainTipResponse handles an incoming chain tip response from the network
func (ci *ConsensusIntegration) HandleChainTipResponse(response *ChainTipResponse) error {
	return ci.syncManager.HandleChainTipResponse(response)
}

// Node Introduction Methods

// CreateNodeIntroduction creates a node introduction message
func (ci *ConsensusIntegration) CreateNodeIntroduction() (*NodeIntroduction, error) {
	// Get chain tip
	chainTip, err := ci.syncManager.getMyChainTip()
	if err != nil {
		return nil, fmt.Errorf("failed to get chain tip: %w", err)
	}

	// Get genesis block hash
	genesisHash := ""
	genesisBlock, err := ci.blockchain.GetBlockByIndex(0)
	if err == nil && genesisBlock != nil {
		genesisHash = genesisBlock.Hash
	}

	// Get uptime (placeholder - should come from uptime tracker)
	uptime := 0.0 // TODO: Get actual uptime from uptime tracker

	intro := &NodeIntroduction{
		NodeID:        ci.nodeID,
		WalletAddress: ci.nodeID, // TODO: Get actual wallet address
		ChainTip:      chainTip,
		GenesisHash:   genesisHash,
		IsBeacon:      false, // TODO: Get from config
		Uptime:        uptime,
		NetworkID:     ci.config.NetworkID,
		Timestamp:     time.Now().Unix(),
		Signature:     "", // TODO: Sign the introduction
	}

	return intro, nil
}

// HandleNodeIntroduction handles a node introduction from a peer
func (ci *ConsensusIntegration) HandleNodeIntroduction(intro *NodeIntroduction) (*NodeIntroductionResponse, error) {
	log.Printf("[ConsensusIntegration] Received node introduction from %s: tip=%d, genesis=%s",
		intro.NodeID, intro.ChainTip, intro.GenesisHash[:8])

	// Store the introduction
	ci.introMutex.Lock()
	ci.peerIntroductions[intro.NodeID] = intro
	ci.introMutex.Unlock()

	// Create our response
	response, err := ci.CreateNodeIntroduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create introduction response: %w", err)
	}

	// Determine if we need to sync
	syncRequested := ci.shouldRequestSync(intro)

	// Convert to response
	introResponse := &NodeIntroductionResponse{
		NodeID:        response.NodeID,
		WalletAddress: response.WalletAddress,
		ChainTip:      response.ChainTip,
		GenesisHash:   response.GenesisHash,
		IsBeacon:      response.IsBeacon,
		Uptime:        response.Uptime,
		NetworkID:     response.NetworkID,
		SyncRequested: syncRequested,
		Timestamp:     response.Timestamp,
		Signature:     response.Signature,
	}

	if syncRequested {
		log.Printf("[ConsensusIntegration] Requesting sync from %s (our tip: %d, their tip: %d)",
			intro.NodeID, response.ChainTip, intro.ChainTip)
		// Trigger sync
		go ci.syncManager.CheckAndSyncIfNeeded()
	}

	return introResponse, nil
}

// HandleNodeIntroductionResponse handles a response to our node introduction
func (ci *ConsensusIntegration) HandleNodeIntroductionResponse(response *NodeIntroductionResponse) error {
	log.Printf("[ConsensusIntegration] Received introduction response from %s: tip=%d, sync_requested=%t",
		response.NodeID, response.ChainTip, response.SyncRequested)

	if response.SyncRequested {
		log.Printf("[ConsensusIntegration] Peer %s requested sync from us", response.NodeID)
		// TODO: Handle sync request from peer
	}

	return nil
}

// shouldRequestSync determines if we should request sync from a peer
func (ci *ConsensusIntegration) shouldRequestSync(intro *NodeIntroduction) bool {
	myTip, err := ci.syncManager.getMyChainTip()
	if err != nil {
		log.Printf("[ConsensusIntegration] Failed to get own chain tip: %v", err)
		return false
	}

	// If we have no blocks, accept any chain
	if myTip == -1 {
		log.Printf("[ConsensusIntegration] No blocks - will accept chain from %s", intro.NodeID)
		return true
	}

	// If peer has higher tip, check genesis block
	if intro.ChainTip > myTip {
		// Check if we have the same genesis block
		myGenesisHash := ""
		myGenesisBlock, err := ci.blockchain.GetBlockByIndex(0)
		if err == nil && myGenesisBlock != nil {
			myGenesisHash = myGenesisBlock.Hash
		}

		if myGenesisHash == "" {
			// We have no genesis block, accept any chain
			log.Printf("[ConsensusIntegration] No genesis block - will accept chain from %s", intro.NodeID)
			return true
		}

		if intro.GenesisHash == myGenesisHash {
			// Same genesis block, safe to sync
			log.Printf("[ConsensusIntegration] Same genesis block - will sync from %s", intro.NodeID)
			return true
		} else {
			// Different genesis block, don't sync
			log.Printf("[ConsensusIntegration] Different genesis block - won't sync from %s", intro.NodeID)
			return false
		}
	}

	return false
}

// Time-Based Block Consensus Methods

// RequestTimeBasedBlock requests consensus to create a time-based block
func (ci *ConsensusIntegration) RequestTimeBasedBlock() error {
	// Get next block index
	latestBlock, err := ci.blockchain.GetLatestBlock()
	if err != nil {
		if err.Error() == "no blocks found" {
			log.Printf("[ConsensusIntegration] No blocks found - cannot request time-based block")
			return nil
		}
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	nextIndex := latestBlock.Index + 1
	requestID := fmt.Sprintf("time_block_%d_%s", nextIndex, ci.nodeID)

	request := &TimeBasedBlockRequest{
		ProposerID: ci.nodeID,
		BlockIndex: nextIndex,
		Timestamp:  time.Now().Unix(),
		Signature:  "", // TODO: Sign the request
	}

	ci.timeBasedBlockMutex.Lock()
	ci.timeBasedBlockRequests[requestID] = request
	ci.timeBasedBlockMutex.Unlock()

	log.Printf("[ConsensusIntegration] Requesting time-based block %d consensus", nextIndex)

	// Vote for our own request since we know conditions are met
	approved := ci.shouldApproveTimeBasedBlock(request)
	vote := &TimeBasedBlockVote{
		ProposerID: ci.nodeID,
		VoterID:    ci.nodeID,
		BlockIndex: nextIndex,
		Approved:   approved,
		Timestamp:  time.Now().Unix(),
		Signature:  "", // TODO: Sign the vote
	}

	// Add our own vote
	ci.timeBasedBlockMutex.Lock()
	if ci.timeBasedBlockVotes[requestID] == nil {
		ci.timeBasedBlockVotes[requestID] = make([]*TimeBasedBlockVote, 0)
	}
	ci.timeBasedBlockVotes[requestID] = append(ci.timeBasedBlockVotes[requestID], vote)
	ci.timeBasedBlockMutex.Unlock()

	log.Printf("[ConsensusIntegration] Self-voted %t on time-based block %d", approved, nextIndex)

	// Broadcast the request to other nodes
	return ci.handleTimeBasedBlockRequestOutbound(request)
}

// HandleTimeBasedBlockRequest handles a time-based block request from another node
func (ci *ConsensusIntegration) HandleTimeBasedBlockRequest(request *TimeBasedBlockRequest) error {
	log.Printf("[ConsensusIntegration] Received time-based block request from %s for block %d",
		request.ProposerID, request.BlockIndex)

	// Check if we should approve
	approved := ci.shouldApproveTimeBasedBlock(request)

	vote := &TimeBasedBlockVote{
		ProposerID: request.ProposerID,
		VoterID:    ci.nodeID,
		BlockIndex: request.BlockIndex,
		Approved:   approved,
		Timestamp:  time.Now().Unix(),
		Signature:  "", // TODO: Sign the vote
	}

	requestID := fmt.Sprintf("time_block_%d_%s", request.BlockIndex, request.ProposerID)

	ci.timeBasedBlockMutex.Lock()
	if ci.timeBasedBlockVotes[requestID] == nil {
		ci.timeBasedBlockVotes[requestID] = make([]*TimeBasedBlockVote, 0)
	}
	ci.timeBasedBlockVotes[requestID] = append(ci.timeBasedBlockVotes[requestID], vote)
	ci.timeBasedBlockMutex.Unlock()

	log.Printf("[ConsensusIntegration] Voted %t on time-based block %d from %s",
		approved, request.BlockIndex, request.ProposerID)

	return ci.handleTimeBasedBlockVoteOutbound(vote)
}

// HandleTimeBasedBlockVote handles a vote on a time-based block request
func (ci *ConsensusIntegration) HandleTimeBasedBlockVote(vote *TimeBasedBlockVote) error {
	requestID := fmt.Sprintf("time_block_%d_%s", vote.BlockIndex, vote.ProposerID)

	ci.timeBasedBlockMutex.Lock()
	if ci.timeBasedBlockVotes[requestID] == nil {
		ci.timeBasedBlockVotes[requestID] = make([]*TimeBasedBlockVote, 0)
	}
	ci.timeBasedBlockVotes[requestID] = append(ci.timeBasedBlockVotes[requestID], vote)
	ci.timeBasedBlockMutex.Unlock()

	log.Printf("[ConsensusIntegration] Received vote %t on time-based block %d from %s",
		vote.Approved, vote.BlockIndex, vote.VoterID)

	// Check if we have enough votes for approval
	ci.checkTimeBasedBlockApproval(requestID)

	return nil
}

// shouldApproveTimeBasedBlock determines if we should approve a time-based block request
func (ci *ConsensusIntegration) shouldApproveTimeBasedBlock(request *TimeBasedBlockRequest) bool {
	// Check if this is the next block in sequence
	latestBlock, err := ci.blockchain.GetLatestBlock()
	if err != nil {
		return false
	}

	if request.BlockIndex != latestBlock.Index+1 {
		log.Printf("[ConsensusIntegration] Block index mismatch: expected %d, got %d",
			latestBlock.Index+1, request.BlockIndex)
		return false
	}

	// Check if we have no pending posts (time-based blocks should be empty)
	postCount := ci.consensusEngine.mempool.GetPostCount()
	if postCount > 0 {
		log.Printf("[ConsensusIntegration] Have %d pending posts - should create content block instead", postCount)
		return false
	}

	// Check if enough time has passed since last block
	timeSinceLastBlock := time.Since(time.Unix(latestBlock.Timestamp, 0))
	if timeSinceLastBlock < 10*time.Minute {
		log.Printf("[ConsensusIntegration] Only %v since last block - too soon for time-based block", timeSinceLastBlock)
		return false
	}

	return true
}

// checkTimeBasedBlockApproval checks if we have enough votes to approve a time-based block
func (ci *ConsensusIntegration) checkTimeBasedBlockApproval(requestID string) {
	ci.timeBasedBlockMutex.Lock()
	votes := ci.timeBasedBlockVotes[requestID]
	ci.timeBasedBlockMutex.Unlock()

	if len(votes) == 0 {
		return
	}

	// Count approvals
	approvals := 0
	for _, vote := range votes {
		if vote.Approved {
			approvals++
		}
	}

	// Get actual peer count from network
	peerCount := 1 // Default to 1 (ourselves)
	if ci.syncManager != nil {
		// Use the sync manager's peer query provider
		if peers, err := ci.syncManager.GetConnectedPeers(); err == nil {
			peerCount = len(peers) + 1 // Total network size (peers + ourselves)
		}
	}

	// Require majority approval (more than 50% of total network)
	requiredApprovals := (peerCount / 2) + 1

	log.Printf("[ConsensusIntegration] Time-based block votes: %d/%d approvals (need %d) - total network size: %d",
		approvals, peerCount, requiredApprovals, peerCount)

	// Log individual votes for debugging
	for i, vote := range votes {
		log.Printf("[ConsensusIntegration] Vote %d: %s voted %t", i+1, vote.VoterID, vote.Approved)
	}

	if approvals >= requiredApprovals {
		log.Printf("[ConsensusIntegration] Time-based block approved with %d/%d votes", approvals, peerCount)

		// Create the time-based block
		ci.createApprovedTimeBasedBlock(requestID)
	}
}

// createApprovedTimeBasedBlock creates a time-based block after approval
func (ci *ConsensusIntegration) createApprovedTimeBasedBlock(requestID string) {
	ci.timeBasedBlockMutex.Lock()
	request := ci.timeBasedBlockRequests[requestID]
	ci.timeBasedBlockMutex.Unlock()

	if request == nil {
		log.Printf("[ConsensusIntegration] No request found for %s", requestID)
		return
	}

	// Get latest block
	latestBlock, err := ci.blockchain.GetLatestBlock()
	if err != nil {
		log.Printf("[ConsensusIntegration] Failed to get latest block: %v", err)
		return
	}

	// Create time-based block
	block, err := ci.blockBuilder.BuildTimeBasedBlock(request.BlockIndex, latestBlock.Hash, latestBlock.StateRoot, latestBlock.Timestamp)
	if err != nil {
		log.Printf("[ConsensusIntegration] Failed to build time-based block: %v", err)
		return
	}

	// Process the block
	ci.blockChan <- block

	log.Printf("[ConsensusIntegration] Created approved time-based block %d", block.Index)
}

// Network callback handlers for time-based blocks
func (ci *ConsensusIntegration) handleTimeBasedBlockRequestOutbound(request *TimeBasedBlockRequest) error {
	log.Printf("[ConsensusIntegration] Broadcasting time-based block request for block %d", request.BlockIndex)

	if ci.onTimeBasedBlockRequest != nil {
		return ci.onTimeBasedBlockRequest(request)
	}

	log.Printf("[ConsensusIntegration] No network callback set for time-based block request")
	return nil
}

func (ci *ConsensusIntegration) handleTimeBasedBlockVoteOutbound(vote *TimeBasedBlockVote) error {
	log.Printf("[ConsensusIntegration] Broadcasting time-based block vote for block %d: %t", vote.BlockIndex, vote.Approved)

	if ci.onTimeBasedBlockVote != nil {
		return ci.onTimeBasedBlockVote(vote)
	}

	log.Printf("[ConsensusIntegration] No network callback set for time-based block vote")
	return nil
}
