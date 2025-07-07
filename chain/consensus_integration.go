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
		blockchain:      blockchain,
		consensusEngine: consensusEngine,
		blockBuilder:    blockBuilder,
		syncManager:     syncManager,
		config:          config,
		nodeID:          nodeID,
		stopChan:        make(chan struct{}),
		blockChan:       make(chan *Block, 50),
	}

	// Set up callbacks
	consensusEngine.SetNetworkCallbacks(
		ci.handlePostGossipOutbound,
		ci.handleBlockProposalOutbound,
		ci.handleBlockVoteOutbound,
		ci.handleBlockCreatedOutbound,
		ci.handleProposalExpiredOutbound,
	)

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
	if !ci.blockBuilder.ShouldCreateTimeBasedBlock() {
		return
	}

	// Get current blockchain state
	latestBlock, err := ci.blockchain.GetLatestBlock()
	if err != nil {
		log.Printf("[ConsensusIntegration] Failed to get latest block: %v", err)
		return
	}

	// Check if we have any pending posts
	if ci.consensusEngine.mempool.GetPostCount() > 0 {
		// Posts are available, let consensus handle it
		return
	}

	// Create time-based block
	nextIndex := latestBlock.Index + 1
	block, err := ci.blockBuilder.BuildTimeBasedBlock(nextIndex, latestBlock.Hash, latestBlock.StateRoot)
	if err != nil {
		log.Printf("[ConsensusIntegration] Failed to build time-based block: %v", err)
		return
	}

	// Process the block
	ci.blockChan <- block
}

// checkReservations checks for approved reservations and creates blocks
func (ci *ConsensusIntegration) checkReservations() {
	reservations := ci.consensusEngine.proposalManager.GetActiveReservations()

	for _, reservation := range reservations {
		// Check if this is our reservation
		if reservation.Proposer == ci.nodeID {
			ci.processOurReservation(reservation)
		}
	}
}

// processOurReservation processes a reservation that we proposed
func (ci *ConsensusIntegration) processOurReservation(reservation *BlockReservation) {
	// Get current blockchain state
	latestBlock, err := ci.blockchain.GetLatestBlock()
	if err != nil {
		log.Printf("[ConsensusIntegration] Failed to get latest block: %v", err)
		return
	}

	// Check if we can build the block
	if latestBlock.Index+1 != reservation.Index {
		log.Printf("[ConsensusIntegration] Block index mismatch: expected %d, got %d",
			latestBlock.Index+1, reservation.Index)
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

	// Increase trust score if we created the block
	if block.Index > 0 { // Skip genesis block
		latestBlock, err := ci.blockchain.GetLatestBlock()
		if err == nil && latestBlock.Hash == block.Hash {
			// We successfully created this block
			ci.consensusEngine.trustManager.OnProposalSuccess(ci.nodeID)
		}
	}

	return nil
}

// validateAndIntegrateBlock validates and integrates a block into the blockchain
func (ci *ConsensusIntegration) validateAndIntegrateBlock(block *Block) error {
	// Validate the block
	if err := ci.blockchain.ValidateBlock(block); err != nil {
		return fmt.Errorf("invalid block: %w", err)
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
