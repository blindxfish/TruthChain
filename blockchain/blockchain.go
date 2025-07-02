package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// Blockchain represents the TruthChain blockchain with persistent storage
type Blockchain struct {
	storage       store.Storage
	PendingPosts  []chain.Post `json:"pending_posts"`
	PostThreshold int          `json:"post_threshold"` // Number of posts needed to create a block
	mu            sync.RWMutex `json:"-"`
}

// NewBlockchain creates a new blockchain with persistent storage
func NewBlockchain(storage store.Storage, postThreshold int) (*Blockchain, error) {
	// Validate mainnet rules if using mainnet
	if err := chain.ValidateMainnetRules(postThreshold, "truthchain-mainnet"); err != nil {
		return nil, fmt.Errorf("mainnet validation failed: %w", err)
	}

	bc := &Blockchain{
		storage:       storage,
		PendingPosts:  []chain.Post{},
		PostThreshold: postThreshold,
	}

	// Check if we need to create genesis block
	_, err := storage.GetLatestBlock()
	if err != nil {
		// No blocks exist, create genesis block
		genesis := chain.CreateGenesisBlock()
		if err := storage.SaveBlock(genesis); err != nil {
			return nil, fmt.Errorf("failed to save genesis block: %w", err)
		}
	} else {
		// Check if genesis block exists at index 0 and validate it
		genesisBlock, err := storage.GetBlock(0)
		if err != nil {
			return nil, fmt.Errorf("failed to get genesis block: %w", err)
		}
		if genesisBlock == nil {
			return nil, fmt.Errorf("genesis block not found at index 0")
		}

		// Validate genesis block matches mainnet
		if !chain.IsMainnetGenesis(genesisBlock) {
			return nil, fmt.Errorf("invalid genesis block: hash mismatch or wrong timestamp")
		}
	}

	// Load pending posts from storage
	pendingPosts, err := storage.GetPendingPosts()
	if err != nil {
		return nil, fmt.Errorf("failed to load pending posts: %w", err)
	}
	bc.PendingPosts = pendingPosts

	return bc, nil
}

// GetLatestBlock returns the most recent block from storage
func (bc *Blockchain) GetLatestBlock() (*chain.Block, error) {
	return bc.storage.GetLatestBlock()
}

// GetBlockByIndex returns a block by its index from storage
func (bc *Blockchain) GetBlockByIndex(index int) (*chain.Block, error) {
	return bc.storage.GetBlock(index)
}

// GetBlockByHash returns a block by its hash from storage
func (bc *Blockchain) GetBlockByHash(hash string) (*chain.Block, error) {
	// Get block count to iterate through all blocks
	count, err := bc.storage.GetBlockCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get block count: %w", err)
	}

	// Search through all blocks
	for i := 0; i < count; i++ {
		block, err := bc.storage.GetBlock(i)
		if err != nil {
			continue
		}
		if block.Hash == hash {
			return block, nil
		}
	}

	return nil, fmt.Errorf("block not found: %s", hash)
}

// GetPendingCharacterCount returns the total characters in pending posts
func (bc *Blockchain) GetPendingCharacterCount() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	count := 0
	for _, post := range bc.PendingPosts {
		count += post.GetCharacterCount()
	}
	return count
}

// GetPendingPostCount returns the number of pending posts
func (bc *Blockchain) GetPendingPostCount() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return len(bc.PendingPosts)
}

// AddPost adds a new post to the pending posts with storage integration
func (bc *Blockchain) AddPost(post chain.Post) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate the post
	if err := post.ValidatePost(); err != nil {
		return fmt.Errorf("invalid post: %w", err)
	}

	// Verify the signature
	valid, err := bc.VerifyPostSignature(post)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid signature for post")
	}

	// Set hash if not already set
	if post.Hash == "" {
		post.SetHash()
	}

	// Check for duplicate posts in storage
	exists, err := bc.storage.PostExists(post.Hash)
	if err != nil {
		return fmt.Errorf("failed to check post existence: %w", err)
	}
	if exists {
		return fmt.Errorf("duplicate post: %s", post.Hash)
	}

	// Check for duplicate posts in pending posts
	for _, existingPost := range bc.PendingPosts {
		if existingPost.Hash == post.Hash {
			return fmt.Errorf("duplicate pending post: %s", post.Hash)
		}
	}

	// Save post to storage
	if err := bc.storage.SavePost(post); err != nil {
		return fmt.Errorf("failed to save post: %w", err)
	}

	// Save to pending posts storage
	if err := bc.storage.SavePendingPost(post); err != nil {
		return fmt.Errorf("failed to save pending post: %w", err)
	}

	// Add to pending posts
	bc.PendingPosts = append(bc.PendingPosts, post)

	// Check if we should create a new block based on post count
	if len(bc.PendingPosts) >= bc.PostThreshold {
		return bc.createBlockFromPending()
	}

	return nil
}

// CreatePost creates a new post from content and wallet
func (bc *Blockchain) CreatePost(content string, w *wallet.Wallet) (*chain.Post, error) {
	if content == "" {
		return nil, fmt.Errorf("post content cannot be empty")
	}

	// Create post data
	postData := fmt.Sprintf("%s%s%d", w.GetAddress(), content, time.Now().Unix())

	// Sign the post data
	signature, err := w.Sign([]byte(postData))
	if err != nil {
		return nil, fmt.Errorf("failed to sign post: %w", err)
	}

	// Create post
	post := chain.Post{
		Author:    w.GetAddress(),
		Signature: hex.EncodeToString(signature),
		Content:   content,
		Timestamp: time.Now().Unix(),
	}

	// Set hash
	post.SetHash()

	return &post, nil
}

// VerifyPostSignature verifies a post's signature and validates authorship
func (bc *Blockchain) VerifyPostSignature(post chain.Post) (bool, error) {
	message := fmt.Sprintf("%s%s%d", post.Author, post.Content, post.Timestamp)
	hash := sha256.Sum256([]byte(message))

	signatureBytes, err := hex.DecodeString(post.Signature)
	if err != nil {
		return false, fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Recover public key
	pubKey, wasCompressed, err := ecdsa.RecoverCompact(signatureBytes, hash[:])
	if err != nil {
		return false, fmt.Errorf("signature recovery failed: %w", err)
	}

	if !wasCompressed {
		return false, fmt.Errorf("signature must be compressed format")
	}

	// Derive address from recovered public key
	derivedAddress := wallet.DeriveAddress(pubKey)

	// Compare with post.Author
	if derivedAddress != post.Author {
		return false, fmt.Errorf("address mismatch: expected %s, got %s", post.Author, derivedAddress)
	}

	// All good
	return true, nil
}

// createBlockFromPending creates a new block from pending posts and saves to storage
func (bc *Blockchain) createBlockFromPending() error {
	if len(bc.PendingPosts) == 0 {
		return fmt.Errorf("no pending posts to create block")
	}

	// Get the latest block from storage
	latestBlock, err := bc.storage.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	// Create new block
	newBlock := chain.CreateBlock(
		latestBlock.Index+1,
		latestBlock.Hash,
		bc.PendingPosts,
	)

	// Validate the new block with post threshold rules
	if err := newBlock.ValidateBlockWithThreshold(bc.PostThreshold); err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}

	// Save block to storage
	if err := bc.storage.SaveBlock(newBlock); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}

	// Clear pending posts from storage
	if err := bc.storage.ClearPendingPosts(); err != nil {
		return fmt.Errorf("failed to clear pending posts: %w", err)
	}

	// Clear pending posts
	bc.PendingPosts = []chain.Post{}

	return nil
}

// ForceCreateBlock forces the creation of a block from pending posts
func (bc *Blockchain) ForceCreateBlock() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.createBlockFromPending()
}

// GetChainLength returns the number of blocks in the chain from storage
func (bc *Blockchain) GetChainLength() (int, error) {
	return bc.storage.GetBlockCount()
}

// GetTotalCharacterCount returns the total characters in all blocks from storage
func (bc *Blockchain) GetTotalCharacterCount() (int, error) {
	count, err := bc.storage.GetBlockCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get block count: %w", err)
	}

	totalChars := 0
	for i := 0; i < count; i++ {
		block, err := bc.storage.GetBlock(i)
		if err != nil {
			continue
		}
		totalChars += block.GetCharacterCount()
	}

	return totalChars, nil
}

// GetTotalPostCount returns the total posts in all blocks from storage
func (bc *Blockchain) GetTotalPostCount() (int, error) {
	count, err := bc.storage.GetBlockCount()
	if err != nil {
		return 0, fmt.Errorf("failed to get block count: %w", err)
	}

	totalPosts := 0
	for i := 0; i < count; i++ {
		block, err := bc.storage.GetBlock(i)
		if err != nil {
			continue
		}
		totalPosts += block.GetPostCount()
	}

	return totalPosts, nil
}

// ValidateBlock validates a single block with post threshold rules
func (bc *Blockchain) ValidateBlock(block *chain.Block) error {
	return block.ValidateBlockWithThreshold(bc.PostThreshold)
}

// ValidateChain validates the entire blockchain from storage
func (bc *Blockchain) ValidateChain() error {
	count, err := bc.storage.GetBlockCount()
	if err != nil {
		return fmt.Errorf("failed to get block count: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("blockchain is empty")
	}

	// Validate each block
	for i := 0; i < count; i++ {
		block, err := bc.storage.GetBlock(i)
		if err != nil {
			return fmt.Errorf("failed to get block %d: %w", i, err)
		}

		// Use enhanced validation with post threshold rules
		if err := block.ValidateBlockWithThreshold(bc.PostThreshold); err != nil {
			return fmt.Errorf("invalid block at index %d: %w", i, err)
		}

		// Check block index
		if block.Index != i {
			return fmt.Errorf("block index mismatch at %d: expected %d, got %d", i, i, block.Index)
		}

		// Check previous hash (except genesis)
		if i > 0 {
			prevBlock, err := bc.storage.GetBlock(i - 1)
			if err != nil {
				return fmt.Errorf("failed to get previous block %d: %w", i-1, err)
			}
			if block.PrevHash != prevBlock.Hash {
				return fmt.Errorf("previous hash mismatch at block %d", i)
			}
		}

		// Check block hash
		calculatedHash := block.CalculateHash()
		if block.Hash != calculatedHash {
			return fmt.Errorf("block hash mismatch at index %d", i)
		}
	}

	return nil
}

// GetBlockchainInfo returns information about the blockchain from storage
func (bc *Blockchain) GetBlockchainInfo() (map[string]interface{}, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	latestBlock, err := bc.storage.GetLatestBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}

	chainLength, err := bc.GetChainLength()
	if err != nil {
		return nil, fmt.Errorf("failed to get chain length: %w", err)
	}

	totalCharCount, err := bc.GetTotalCharacterCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get total character count: %w", err)
	}

	totalPostCount, err := bc.GetTotalPostCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get total post count: %w", err)
	}

	info := map[string]interface{}{
		"chain_length":            chainLength,
		"total_character_count":   totalCharCount,
		"total_post_count":        totalPostCount,
		"pending_post_count":      len(bc.PendingPosts),
		"pending_character_count": bc.GetPendingCharacterCount(),
		"post_threshold":          bc.PostThreshold,
		"latest_block_index":      latestBlock.Index,
		"latest_block_hash":       latestBlock.Hash,
		"latest_block_timestamp":  latestBlock.Timestamp,
	}

	return info, nil
}

// GetCharacterBalance returns the character balance for an address
func (bc *Blockchain) GetCharacterBalance(address string) (int, error) {
	return bc.storage.GetCharacterBalance(address)
}

// UpdateCharacterBalance updates the character balance for an address
func (bc *Blockchain) UpdateCharacterBalance(address string, amount int) error {
	return bc.storage.UpdateCharacterBalance(address, amount)
}

// Close closes the storage connection
func (bc *Blockchain) Close() error {
	return bc.storage.Close()
}

// GetPendingPosts returns a copy of all pending posts
func (bc *Blockchain) GetPendingPosts() []chain.Post {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	// Return a copy to avoid race conditions
	pendingCopy := make([]chain.Post, len(bc.PendingPosts))
	copy(pendingCopy, bc.PendingPosts)
	return pendingCopy
}

// GetPendingPostByHash returns a specific pending post by hash
func (bc *Blockchain) GetPendingPostByHash(hash string) *chain.Post {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for _, post := range bc.PendingPosts {
		if post.Hash == hash {
			// Return a copy to avoid race conditions
			postCopy := post
			return &postCopy
		}
	}
	return nil
}

// RemovePendingPost removes a post from the pending pool (for editing/deletion)
func (bc *Blockchain) RemovePendingPost(hash string) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for i, post := range bc.PendingPosts {
		if post.Hash == hash {
			// Remove from pending posts
			bc.PendingPosts = append(bc.PendingPosts[:i], bc.PendingPosts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("pending post not found: %s", hash)
}

// UpdatePendingPost updates a pending post (for editing)
func (bc *Blockchain) UpdatePendingPost(hash string, newContent string, w *wallet.Wallet) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Find the post to update
	for i, post := range bc.PendingPosts {
		if post.Hash == hash {
			// Verify the post belongs to this wallet
			if post.Author != w.GetAddress() {
				return fmt.Errorf("post does not belong to this wallet")
			}

			// Create new post with updated content
			newPost, err := bc.CreatePost(newContent, w)
			if err != nil {
				return fmt.Errorf("failed to create updated post: %w", err)
			}

			// Replace the old post
			bc.PendingPosts[i] = *newPost

			// Update in storage
			if err := bc.storage.SavePost(*newPost); err != nil {
				return fmt.Errorf("failed to save updated post: %w", err)
			}

			return nil
		}
	}
	return fmt.Errorf("pending post not found: %s", hash)
}

// GetMempoolInfo returns information about the mempool
func (bc *Blockchain) GetMempoolInfo() map[string]interface{} {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	info := map[string]interface{}{
		"pending_post_count":      len(bc.PendingPosts),
		"pending_character_count": 0,
		"post_threshold":          bc.PostThreshold,
		"posts":                   []map[string]interface{}{},
	}

	// Calculate character count and build post list
	charCount := 0
	posts := make([]map[string]interface{}, len(bc.PendingPosts))

	for i, post := range bc.PendingPosts {
		charCount += post.GetCharacterCount()
		posts[i] = map[string]interface{}{
			"hash":       post.Hash,
			"author":     post.Author,
			"content":    post.Content,
			"timestamp":  post.Timestamp,
			"characters": post.GetCharacterCount(),
		}
	}

	info["pending_character_count"] = charCount
	info["posts"] = posts

	return info
}
