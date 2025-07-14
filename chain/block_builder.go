package chain

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// BlockBuilder creates blocks from approved consensus proposals
type BlockBuilder struct {
	consensusEngine *ConsensusEngine
	storage         interface{} // TODO: Replace with proper storage interface

	// Configuration
	postThreshold int
	timeInterval  time.Duration

	// State
	lastBlockTime time.Time
	mu            sync.RWMutex
}

// NewBlockBuilder creates a new block builder
func NewBlockBuilder(consensusEngine *ConsensusEngine, postThreshold int, timeInterval time.Duration) *BlockBuilder {
	return &BlockBuilder{
		consensusEngine: consensusEngine,
		postThreshold:   postThreshold,
		timeInterval:    timeInterval,
		lastBlockTime:   time.Now(),
	}
}

// BuildBlockFromProposal builds a block from an approved proposal
func (bb *BlockBuilder) BuildBlockFromProposal(reservation *BlockReservation, prevHash string, prevStateRoot *StateRoot) (*Block, error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	// Validate reservation
	if err := bb.validateReservation(reservation); err != nil {
		return nil, fmt.Errorf("invalid reservation: %w", err)
	}

	// Get posts from mempool
	posts := make([]Post, 0, len(reservation.PostHashes))
	for _, hash := range reservation.PostHashes {
		post, exists := bb.consensusEngine.mempool.GetPost(hash)
		if !exists {
			return nil, fmt.Errorf("post not found in mempool: %s", hash)
		}
		posts = append(posts, post)
	}

	// Sort posts by timestamp for deterministic ordering
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Timestamp < posts[j].Timestamp
	})

	// Create state root
	stateRoot, err := bb.createStateRoot(reservation.Index, prevStateRoot, posts)
	if err != nil {
		return nil, fmt.Errorf("failed to create state root: %w", err)
	}

	// Create block
	block := &Block{
		Index:     reservation.Index,
		Timestamp: time.Now().Unix(),
		PrevHash:  prevHash,
		Posts:     posts,
		StateRoot: stateRoot,
		CharCount: bb.calculateCharCount(posts),
	}

	// Calculate block hash
	block.SetHash()

	// Validate block
	if err := block.ValidateBlock(); err != nil {
		return nil, fmt.Errorf("invalid block: %w", err)
	}

	log.Printf("[BlockBuilder] Built block %d with %d posts, %d characters",
		block.Index, len(posts), block.CharCount)

	return block, nil
}

// BuildTimeBasedBlock builds a time-based block when no posts are available
func (bb *BlockBuilder) BuildTimeBasedBlock(blockIndex int, prevHash string, prevStateRoot *StateRoot, lastBlockTimestamp int64) (*Block, error) {
	bb.mu.Lock()
	defer bb.mu.Unlock()

	// Check if enough time has passed since the actual last block
	timeSinceLastBlock := time.Since(time.Unix(lastBlockTimestamp, 0))
	if timeSinceLastBlock < bb.timeInterval {
		return nil, fmt.Errorf("not enough time passed since last block")
	}

	// Create empty state root (no changes)
	stateRoot := &StateRoot{
		Wallets:    prevStateRoot.Wallets,
		BlockIndex: blockIndex,
	}
	stateRoot.SetHash()

	// Create empty block
	block := &Block{
		Index:     blockIndex,
		Timestamp: time.Now().Unix(),
		PrevHash:  prevHash,
		Posts:     []Post{}, // Empty posts for time-based block
		StateRoot: stateRoot,
		CharCount: 0,
	}

	// Calculate block hash
	block.SetHash()

	// Update last block time
	bb.lastBlockTime = time.Now()

	log.Printf("[BlockBuilder] Built time-based block %d (no posts)", block.Index)

	return block, nil
}

// validateReservation validates a block reservation
func (bb *BlockBuilder) validateReservation(reservation *BlockReservation) error {
	if reservation.Index < 0 {
		return fmt.Errorf("invalid block index: %d", reservation.Index)
	}

	if reservation.Proposer == "" {
		return fmt.Errorf("empty proposer")
	}

	if len(reservation.PostHashes) != bb.postThreshold {
		return fmt.Errorf("wrong number of posts: got %d, want %d", len(reservation.PostHashes), bb.postThreshold)
	}

	if time.Now().After(reservation.TimeoutAt) {
		return fmt.Errorf("reservation expired")
	}

	return nil
}

// createStateRoot creates a new state root based on posts
func (bb *BlockBuilder) createStateRoot(blockIndex int, prevStateRoot *StateRoot, posts []Post) (*StateRoot, error) {
	// Start with previous state
	newState := &StateRoot{
		Wallets:    make([]WalletState, len(prevStateRoot.Wallets)),
		BlockIndex: blockIndex,
	}
	copy(newState.Wallets, prevStateRoot.Wallets)

	// Process posts to update wallet states
	for _, post := range posts {
		if err := bb.processPostForState(post, newState); err != nil {
			return nil, fmt.Errorf("failed to process post for state: %w", err)
		}
	}

	// Sort wallets by address for deterministic hashing
	sort.Slice(newState.Wallets, func(i, j int) bool {
		return newState.Wallets[i].Address < newState.Wallets[j].Address
	})

	// Calculate hash
	newState.SetHash()

	return newState, nil
}

// processPostForState processes a post to update wallet state
func (bb *BlockBuilder) processPostForState(post Post, state *StateRoot) error {
	// Find or create wallet state for post author
	walletState, exists := state.GetWalletState(post.Author)
	if !exists {
		// Create new wallet state
		walletState = &WalletState{
			Address:    post.Author,
			Balance:    0, // New wallets start with 0 balance
			Nonce:      0,
			LastTxTime: post.Timestamp,
		}
		state.Wallets = append(state.Wallets, *walletState)
	} else {
		// Update existing wallet state
		walletState.Balance -= post.GetCharacterCount() // Burn characters
		walletState.Nonce++
		walletState.LastTxTime = post.Timestamp
		state.UpdateWalletState(*walletState)
	}

	return nil
}

// calculateCharCount calculates the total character count of posts
func (bb *BlockBuilder) calculateCharCount(posts []Post) int {
	total := 0
	for _, post := range posts {
		total += post.GetCharacterCount()
	}
	return total
}

// GetNextBlockIndex gets the next block index
func (bb *BlockBuilder) GetNextBlockIndex() int {
	// This would query the blockchain for the latest block index
	// For now, return a placeholder
	return 0 // TODO: Implement proper block index tracking
}

// GetPrevHash gets the hash of the previous block
func (bb *BlockBuilder) GetPrevHash() string {
	// This would query the blockchain for the latest block hash
	// For now, return a placeholder
	return "" // TODO: Implement proper hash tracking
}

// GetPrevStateRoot gets the state root of the previous block
func (bb *BlockBuilder) GetPrevStateRoot() *StateRoot {
	// This would query the blockchain for the latest state root
	// For now, return a placeholder
	return &StateRoot{
		Wallets:    []WalletState{},
		Hash:       "",
		BlockIndex: -1,
	} // TODO: Implement proper state root tracking
}

// ShouldCreateTimeBasedBlock checks if a time-based block should be created
func (bb *BlockBuilder) ShouldCreateTimeBasedBlock() bool {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	return time.Since(bb.lastBlockTime) >= bb.timeInterval
}

// GetStats returns block builder statistics
func (bb *BlockBuilder) GetStats() map[string]interface{} {
	bb.mu.RLock()
	defer bb.mu.RUnlock()

	return map[string]interface{}{
		"post_threshold":           bb.postThreshold,
		"time_interval_secs":       bb.timeInterval.Seconds(),
		"last_block_time":          bb.lastBlockTime,
		"time_since_last":          time.Since(bb.lastBlockTime).Seconds(),
		"should_create_time_based": bb.ShouldCreateTimeBasedBlock(),
	}
}
