package chain

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ConsensusEngine orchestrates the forkless block creation system
type ConsensusEngine struct {
	config          *ConsensusConfig
	mempool         *PostMempool
	trustManager    *TrustManager
	proposalManager *BlockProposalManager

	// Network callbacks
	onPostGossip      func(*PostGossip) error
	onBlockProposal   func(*BlockProposal) error
	onBlockVote       func(*BlockVote) error
	onBlockCreated    func(*Block) error
	onProposalExpired func(*ProposalExpired) error

	// State
	nodeID    string
	isRunning bool
	mu        sync.RWMutex

	// Channels
	stopChan     chan struct{}
	postChan     chan Post
	proposalChan chan *BlockProposal
	voteChan     chan *BlockVote
	blockChan    chan *Block
}

// NewConsensusEngine creates a new consensus engine
func NewConsensusEngine(nodeID string, config *ConsensusConfig) *ConsensusEngine {
	return &ConsensusEngine{
		config:          config,
		mempool:         NewPostMempool(),
		trustManager:    NewTrustManager(config),
		proposalManager: NewBlockProposalManager(config),
		nodeID:          nodeID,
		stopChan:        make(chan struct{}),
		postChan:        make(chan Post, 100),
		proposalChan:    make(chan *BlockProposal, 50),
		voteChan:        make(chan *BlockVote, 100),
		blockChan:       make(chan *Block, 50),
	}
}

// Start starts the consensus engine
func (ce *ConsensusEngine) Start() error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if ce.isRunning {
		return fmt.Errorf("consensus engine already running")
	}

	ce.isRunning = true

	// Start background workers
	go ce.postWorker()
	go ce.proposalWorker()
	go ce.voteWorker()
	go ce.blockWorker()
	go ce.cleanupWorker()

	log.Printf("[Consensus] Started consensus engine for node %s", ce.nodeID)
	return nil
}

// Stop stops the consensus engine
func (ce *ConsensusEngine) Stop() error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if !ce.isRunning {
		return nil
	}

	ce.isRunning = false
	close(ce.stopChan)

	log.Printf("[Consensus] Stopped consensus engine for node %s", ce.nodeID)
	return nil
}

// SetNetworkCallbacks sets the network communication callbacks
func (ce *ConsensusEngine) SetNetworkCallbacks(
	onPostGossip func(*PostGossip) error,
	onBlockProposal func(*BlockProposal) error,
	onBlockVote func(*BlockVote) error,
	onBlockCreated func(*Block) error,
	onProposalExpired func(*ProposalExpired) error,
) {
	ce.onPostGossip = onPostGossip
	ce.onBlockProposal = onBlockProposal
	ce.onBlockVote = onBlockVote
	ce.onBlockCreated = onBlockCreated
	ce.onProposalExpired = onProposalExpired
}

// AddPost adds a post to the consensus system
func (ce *ConsensusEngine) AddPost(post Post) error {
	// Validate post
	if err := post.ValidatePost(); err != nil {
		return fmt.Errorf("invalid post: %w", err)
	}

	// Set hash if not already set
	if post.Hash == "" {
		post.SetHash()
	}

	// Add to mempool
	if err := ce.mempool.AddPost(post); err != nil {
		return fmt.Errorf("failed to add post to mempool: %w", err)
	}

	// Gossip the post to network
	if ce.onPostGossip != nil {
		gossip := &PostGossip{
			Post:      post,
			NodeID:    ce.nodeID,
			Timestamp: time.Now().Unix(),
			Signature: post.Signature, // Reuse post signature for now
		}

		if err := ce.onPostGossip(gossip); err != nil {
			log.Printf("[Consensus] Failed to gossip post: %v", err)
		}
	}

	// Check if we should propose a block
	if ce.mempool.IsReadyForBlock(ce.config.PostThreshold) && ce.trustManager.CanPropose(ce.nodeID) {
		ce.tryProposeBlock()
	}

	return nil
}

// HandlePostGossip handles a post gossip from another node
func (ce *ConsensusEngine) HandlePostGossip(gossip *PostGossip) error {
	// Validate the gossip
	if err := gossip.Post.ValidatePost(); err != nil {
		return fmt.Errorf("invalid post in gossip: %w", err)
	}

	// Add to mempool
	if err := ce.mempool.AddPost(gossip.Post); err != nil {
		// Ignore duplicate posts
		if err.Error() == fmt.Sprintf("duplicate post: %s", gossip.Post.Hash) {
			return nil
		}
		return fmt.Errorf("failed to add gossiped post: %w", err)
	}

	log.Printf("[Consensus] Added gossiped post from %s: %s", gossip.NodeID, gossip.Post.Hash[:8])

	// Check if we should propose a block
	if ce.mempool.IsReadyForBlock(ce.config.PostThreshold) && ce.trustManager.CanPropose(ce.nodeID) {
		ce.tryProposeBlock()
	}

	return nil
}

// HandleBlockProposal handles a block proposal from another node
func (ce *ConsensusEngine) HandleBlockProposal(proposal *BlockProposal) error {
	// Validate proposal
	if err := ce.validateProposal(proposal); err != nil {
		return fmt.Errorf("invalid proposal: %w", err)
	}

	// Submit to proposal manager
	if err := ce.proposalManager.SubmitProposal(proposal); err != nil {
		return fmt.Errorf("failed to submit proposal: %w", err)
	}

	log.Printf("[Consensus] Received block proposal from %s for block %d", proposal.ProposerID, proposal.Index)

	// Vote on the proposal
	vote := ce.createVote(proposal, true) // For now, always approve valid proposals
	if err := ce.proposalManager.SubmitVote(vote); err != nil {
		return fmt.Errorf("failed to submit vote: %w", err)
	}

	// Send vote to network
	if ce.onBlockVote != nil {
		if err := ce.onBlockVote(vote); err != nil {
			log.Printf("[Consensus] Failed to send vote: %v", err)
		}
	}

	return nil
}

// HandleBlockVote handles a block vote from another node
func (ce *ConsensusEngine) HandleBlockVote(vote *BlockVote) error {
	// Validate vote
	if err := ce.validateVote(vote); err != nil {
		return fmt.Errorf("invalid vote: %w", err)
	}

	// Submit to proposal manager
	if err := ce.proposalManager.SubmitVote(vote); err != nil {
		return fmt.Errorf("failed to submit vote: %w", err)
	}

	log.Printf("[Consensus] Received vote from %s on block %d: %t", vote.VoterID, vote.Index, vote.Approved)

	return nil
}

// HandleBlockCreated handles a block that was created and accepted
func (ce *ConsensusEngine) HandleBlockCreated(block *Block) error {
	// Remove posts from mempool
	postHashes := make([]string, len(block.Posts))
	for i, post := range block.Posts {
		postHashes[i] = post.Hash
	}
	ce.mempool.RemovePosts(postHashes)

	// Remove any reservation for this block
	ce.proposalManager.RemoveReservation(block.Index)

	log.Printf("[Consensus] Block %d created, removed %d posts from mempool", block.Index, len(postHashes))

	return nil
}

// tryProposeBlock attempts to propose a block if conditions are met
func (ce *ConsensusEngine) tryProposeBlock() {
	// Check if we can propose
	if !ce.trustManager.CanPropose(ce.nodeID) {
		return
	}

	// Check if we have enough posts
	if !ce.mempool.IsReadyForBlock(ce.config.PostThreshold) {
		return
	}

	// Check if there's already a proposal for the next block
	nextBlockIndex := ce.getNextBlockIndex()
	if _, exists := ce.proposalManager.GetReservation(nextBlockIndex); exists {
		return
	}

	// Select posts for the block
	posts := ce.mempool.SelectPostsForBlock(ce.config.PostThreshold)
	postHashes := make([]string, len(posts))
	for i, post := range posts {
		postHashes[i] = post.Hash
	}

	// Create proposal
	proposal := &BlockProposal{
		Index:      nextBlockIndex,
		ProposerID: ce.nodeID,
		PostHashes: postHashes,
		Timestamp:  time.Now().Unix(),
		TrustScore: ce.trustManager.GetTrustScore(ce.nodeID),
		Signature:  "", // TODO: Sign the proposal
	}

	// Submit proposal
	if err := ce.proposalManager.SubmitProposal(proposal); err != nil {
		log.Printf("[Consensus] Failed to submit proposal: %v", err)
		return
	}

	// Send to network
	if ce.onBlockProposal != nil {
		if err := ce.onBlockProposal(proposal); err != nil {
			log.Printf("[Consensus] Failed to send proposal: %v", err)
		}
	}

	log.Printf("[Consensus] Proposed block %d with %d posts", nextBlockIndex, len(posts))
}

// createVote creates a vote on a proposal
func (ce *ConsensusEngine) createVote(proposal *BlockProposal, approved bool) *BlockVote {
	return &BlockVote{
		Index:      proposal.Index,
		ProposerID: proposal.ProposerID,
		VoterID:    ce.nodeID,
		Timestamp:  time.Now().Unix(),
		Approved:   approved,
		Signature:  "", // TODO: Sign the vote
	}
}

// validateProposal validates a block proposal
func (ce *ConsensusEngine) validateProposal(proposal *BlockProposal) error {
	if proposal.Index < 0 {
		return fmt.Errorf("invalid block index: %d", proposal.Index)
	}

	if proposal.ProposerID == "" {
		return fmt.Errorf("empty proposer ID")
	}

	if len(proposal.PostHashes) != ce.config.PostThreshold {
		return fmt.Errorf("wrong number of posts: got %d, want %d", len(proposal.PostHashes), ce.config.PostThreshold)
	}

	if proposal.TrustScore < ce.config.MinTrustScore {
		return fmt.Errorf("insufficient trust score: %f < %f", proposal.TrustScore, ce.config.MinTrustScore)
	}

	// Validate that all posts exist in mempool
	for _, hash := range proposal.PostHashes {
		if _, exists := ce.mempool.GetPost(hash); !exists {
			return fmt.Errorf("post not found in mempool: %s", hash)
		}
	}

	return nil
}

// validateVote validates a block vote
func (ce *ConsensusEngine) validateVote(vote *BlockVote) error {
	if vote.Index < 0 {
		return fmt.Errorf("invalid block index: %d", vote.Index)
	}

	if vote.ProposerID == "" {
		return fmt.Errorf("empty proposer ID")
	}

	if vote.VoterID == "" {
		return fmt.Errorf("empty voter ID")
	}

	if vote.VoterID == vote.ProposerID {
		return fmt.Errorf("voter cannot vote on own proposal")
	}

	return nil
}

// getNextBlockIndex gets the next block index (simplified - in real implementation,
// this would query the blockchain)
func (ce *ConsensusEngine) getNextBlockIndex() int {
	// This is a simplified implementation
	// In a real system, you'd query the blockchain for the latest block index
	return 0 // TODO: Implement proper block index tracking
}

// Background workers

func (ce *ConsensusEngine) postWorker() {
	for {
		select {
		case <-ce.stopChan:
			return
		case post := <-ce.postChan:
			if err := ce.AddPost(post); err != nil {
				log.Printf("[Consensus] Failed to add post: %v", err)
			}
		}
	}
}

func (ce *ConsensusEngine) proposalWorker() {
	for {
		select {
		case <-ce.stopChan:
			return
		case proposal := <-ce.proposalChan:
			if err := ce.HandleBlockProposal(proposal); err != nil {
				log.Printf("[Consensus] Failed to handle proposal: %v", err)
			}
		}
	}
}

func (ce *ConsensusEngine) voteWorker() {
	for {
		select {
		case <-ce.stopChan:
			return
		case vote := <-ce.voteChan:
			if err := ce.HandleBlockVote(vote); err != nil {
				log.Printf("[Consensus] Failed to handle vote: %v", err)
			}
		}
	}
}

func (ce *ConsensusEngine) blockWorker() {
	for {
		select {
		case <-ce.stopChan:
			return
		case block := <-ce.blockChan:
			if err := ce.HandleBlockCreated(block); err != nil {
				log.Printf("[Consensus] Failed to handle block: %v", err)
			}
		}
	}
}

func (ce *ConsensusEngine) cleanupWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ce.stopChan:
			return
		case <-ticker.C:
			ce.cleanupExpiredReservations()
		}
	}
}

// cleanupExpiredReservations handles expired block reservations
func (ce *ConsensusEngine) cleanupExpiredReservations() {
	expired := ce.proposalManager.CleanupExpiredReservations()

	for _, blockIndex := range expired {
		// Get the reservation before it was removed
		reservation, _ := ce.proposalManager.GetReservation(blockIndex)
		if reservation != nil {
			// Decrease trust score of the proposer
			ce.trustManager.OnProposalFailure(reservation.Proposer)

			// Announce expiration
			if ce.onProposalExpired != nil {
				expiredMsg := &ProposalExpired{
					Index:      blockIndex,
					ProposerID: reservation.Proposer,
					Timestamp:  time.Now().Unix(),
					Signature:  "", // TODO: Sign the message
				}

				if err := ce.onProposalExpired(expiredMsg); err != nil {
					log.Printf("[Consensus] Failed to announce expiration: %v", err)
				}
			}

			log.Printf("[Consensus] Block %d proposal expired, decreased trust for %s", blockIndex, reservation.Proposer)
		}
	}
}

// GetStats returns consensus statistics
func (ce *ConsensusEngine) GetStats() map[string]interface{} {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	return map[string]interface{}{
		"node_id":                  ce.nodeID,
		"is_running":               ce.isRunning,
		"mempool_post_count":       ce.mempool.GetPostCount(),
		"active_proposals":         len(ce.proposalManager.GetActiveProposals()),
		"active_reservations":      len(ce.proposalManager.GetActiveReservations()),
		"trust_score":              ce.trustManager.GetTrustScore(ce.nodeID),
		"can_propose":              ce.trustManager.CanPropose(ce.nodeID),
		"post_threshold":           ce.config.PostThreshold,
		"min_trust_score":          ce.config.MinTrustScore,
		"proposal_timeout_minutes": ce.config.ProposalTimeout.Minutes(),
	}
}
