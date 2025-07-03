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
	stateManager  *chain.StateManager
	PendingPosts  []chain.Post        `json:"pending_posts"`
	TransferPool  *chain.TransferPool `json:"transfer_pool"`
	PostThreshold int                 `json:"post_threshold"` // Number of posts needed to create a block
	TimeInterval  time.Duration       `json:"time_interval"`  // Time interval for block creation (10 minutes)
	lastBlockTime time.Time           `json:"last_block_time"`
	mu            sync.RWMutex        `json:"-"`
}

// NewBlockchain creates a new blockchain with persistent storage
func NewBlockchain(storage store.Storage, postThreshold int) (*Blockchain, error) {
	// Validate mainnet rules if using mainnet
	if err := chain.ValidateMainnetRules(postThreshold, "truthchain-mainnet"); err != nil {
		return nil, fmt.Errorf("mainnet validation failed: %w", err)
	}

	bc := &Blockchain{
		storage:       storage,
		stateManager:  chain.NewStateManager(),
		PendingPosts:  []chain.Post{},
		TransferPool:  chain.NewTransferPool(),
		PostThreshold: postThreshold,
		TimeInterval:  10 * time.Minute, // Create blocks every 10 minutes if no posts
		lastBlockTime: time.Now(),
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

	// Initialize state from latest block
	if err := bc.initializeState(); err != nil {
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	// Start background goroutine for time-based block creation
	go bc.timeBasedBlockLoop()

	return bc, nil
}

// initializeState loads the current state from the latest block
func (bc *Blockchain) initializeState() error {
	latestBlock, err := bc.storage.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	// Load state from the latest block's state root
	if latestBlock.StateRoot != nil {
		if err := bc.stateManager.LoadStateFromStateRoot(latestBlock.StateRoot); err != nil {
			return fmt.Errorf("failed to load state from state root: %w", err)
		}
	}

	return nil
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
	return bc.storage.GetBlockByHash(hash)
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

// AddPost adds a post to the pending posts and validates it
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

	// Validate post author has sufficient balance for posting
	// Posts cost 1 character per character in content
	postCost := post.GetCharacterCount()

	// Check effective balance (considering pending transfers)
	pendingTransfers := bc.TransferPool.GetTransfers()
	effectiveBalance := bc.stateManager.GetEffectiveBalance(post.Author, pendingTransfers)

	if effectiveBalance < postCost {
		return fmt.Errorf("insufficient balance for post: %d characters needed, effective balance: %d", postCost, effectiveBalance)
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

	// Check if we should create a new block based on post count or time
	if len(bc.PendingPosts) >= bc.PostThreshold {
		return bc.createBlockFromPending()
	}

	// Check if we should create a block based on time interval
	if bc.shouldCreateTimeBasedBlock() {
		return bc.createTimeBasedBlock()
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

	// Get pending transfers
	pendingTransfers := bc.TransferPool.GetTransfers()

	// Apply pending transfers to state
	for _, transfer := range pendingTransfers {
		if err := bc.stateManager.ApplyTransfer(transfer); err != nil {
			return fmt.Errorf("failed to apply transfer %s: %w", transfer.Hash, err)
		}
	}

	// Apply post costs to state
	for _, post := range bc.PendingPosts {
		postCost := post.GetCharacterCount()
		currentBalance, err := bc.storage.GetCharacterBalance(post.Author)
		if err != nil {
			return fmt.Errorf("failed to get balance for %s: %w", post.Author, err)
		}

		// Deduct post cost
		if err := bc.storage.UpdateCharacterBalance(post.Author, -postCost); err != nil {
			return fmt.Errorf("failed to deduct post cost for %s: %w", post.Author, err)
		}

		// Update state manager
		bc.stateManager.UpdateWalletState(post.Author, currentBalance-postCost, 0)
	}

	// Calculate new state root
	newStateRoot := bc.stateManager.CalculateStateRoot(latestBlock.Index + 1)

	// Create new block
	newBlock := chain.CreateBlock(
		latestBlock.Index+1,
		latestBlock.Hash,
		bc.PendingPosts,
		pendingTransfers,
		newStateRoot,
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

	// Clear transfer pool
	bc.TransferPool.ClearPool()

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
		"wallet_count":            bc.stateManager.GetWalletCount(),
		"total_character_supply":  bc.stateManager.GetTotalCharacterSupply(),
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

// UpdateWalletState updates the wallet state in the state manager
func (bc *Blockchain) UpdateWalletState(address string, balance int, nonce int64) {
	bc.stateManager.UpdateWalletState(address, balance, nonce)
}

// Close closes the storage connection
func (bc *Blockchain) Close() error {
	return bc.storage.Close()
}

// GetPendingPosts returns a copy of all pending posts
func (bc *Blockchain) GetPendingPosts() []chain.Post {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	posts := make([]chain.Post, len(bc.PendingPosts))
	copy(posts, bc.PendingPosts)
	return posts
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

// CreateTransfer creates a new signed transfer transaction
func (bc *Blockchain) CreateTransfer(to string, amount int, w *wallet.Wallet) (*chain.Transfer, error) {
	// Get next nonce for the sender
	nonce := bc.stateManager.GetNextNonce(w.GetAddress())

	// Create transfer without signature first
	transfer := &chain.Transfer{
		From:      w.GetAddress(),
		To:        to,
		Amount:    amount,
		GasFee:    1, // Fixed 1 character gas fee
		Timestamp: time.Now().Unix(),
		Nonce:     nonce,
	}

	// Calculate hash
	hash, err := transfer.CalculateHash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate transfer hash: %w", err)
	}
	transfer.Hash = hash

	// Sign the transfer using wallet's signing method
	// Note: w.Sign() already hashes the data, so we pass the raw transfer data
	transferData := fmt.Sprintf("%s:%s:%d:%d:%d:%d", transfer.From, transfer.To, transfer.Amount, transfer.GasFee, transfer.Timestamp, transfer.Nonce)
	signatureBytes, err := w.Sign([]byte(transferData))
	if err != nil {
		return nil, fmt.Errorf("failed to sign transfer: %w", err)
	}

	transfer.Signature = hex.EncodeToString(signatureBytes)

	return transfer, nil
}

// AddTransfer adds a transfer to the pool
func (bc *Blockchain) AddTransfer(transfer chain.Transfer) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate transfer
	if err := transfer.Validate(); err != nil {
		return fmt.Errorf("invalid transfer: %w", err)
	}

	// Validate against current state
	pendingTransfers := bc.TransferPool.GetTransfers()
	if err := bc.stateManager.ValidateTransfer(transfer, pendingTransfers); err != nil {
		return fmt.Errorf("transfer validation failed: %w", err)
	}

	// Add to transfer pool
	if err := bc.TransferPool.AddTransfer(transfer); err != nil {
		return fmt.Errorf("failed to add transfer to pool: %w", err)
	}

	return nil
}

// ProcessTransfers processes all transfers in the pool
func (bc *Blockchain) ProcessTransfers() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	transfers := bc.TransferPool.GetTransfers()
	if len(transfers) == 0 {
		return nil // No transfers to process
	}

	// Process each transfer
	for _, transfer := range transfers {
		// Apply to state manager
		if err := bc.stateManager.ApplyTransfer(transfer); err != nil {
			return fmt.Errorf("failed to apply transfer %s: %w", transfer.Hash, err)
		}

		// Update storage
		if err := bc.storage.UpdateCharacterBalance(transfer.From, -transfer.GetTotalCost()); err != nil {
			return fmt.Errorf("failed to deduct from sender %s: %w", transfer.From, err)
		}

		if err := bc.storage.UpdateCharacterBalance(transfer.To, transfer.Amount); err != nil {
			// Rollback sender deduction
			bc.storage.UpdateCharacterBalance(transfer.From, transfer.GetTotalCost())
			return fmt.Errorf("failed to add to recipient %s: %w", transfer.To, err)
		}

		// Remove from pool
		if err := bc.TransferPool.RemoveTransfer(transfer.Hash); err != nil {
			return fmt.Errorf("failed to remove transfer from pool: %w", err)
		}
	}

	return nil
}

// GetNextNonce gets the next nonce for an address
func (bc *Blockchain) GetNextNonce(address string) int64 {
	return bc.stateManager.GetNextNonce(address)
}

// GetTransferPoolInfo returns information about the transfer pool
func (bc *Blockchain) GetTransferPoolInfo() map[string]interface{} {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	transfers := bc.TransferPool.GetTransfers()
	info := map[string]interface{}{
		"transfer_count":         len(transfers),
		"total_character_volume": bc.TransferPool.GetTotalCharacterVolume(),
		"transfers":              []map[string]interface{}{},
	}

	// Build transfer list
	transferList := make([]map[string]interface{}, len(transfers))
	for i, transfer := range transfers {
		transferList[i] = map[string]interface{}{
			"hash":      transfer.Hash,
			"from":      transfer.From,
			"to":        transfer.To,
			"amount":    transfer.Amount,
			"gas_fee":   transfer.GasFee,
			"timestamp": transfer.Timestamp,
			"nonce":     transfer.Nonce,
		}
	}
	info["transfers"] = transferList

	return info
}

// GetStateInfo returns information about the current state
func (bc *Blockchain) GetStateInfo() map[string]interface{} {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	wallets := bc.stateManager.GetAllWallets()
	walletList := make([]map[string]interface{}, len(wallets))

	for i, wallet := range wallets {
		walletList[i] = map[string]interface{}{
			"address":      wallet.Address,
			"balance":      wallet.Balance,
			"nonce":        wallet.Nonce,
			"last_tx_time": wallet.LastTxTime,
		}
	}

	return map[string]interface{}{
		"wallet_count":           bc.stateManager.GetWalletCount(),
		"total_character_supply": bc.stateManager.GetTotalCharacterSupply(),
		"wallets":                walletList,
	}
}

// IntegrateBlocksFromSync integrates blocks received from a sync operation
func (bc *Blockchain) IntegrateBlocksFromSync(blocks []*chain.Block) (int, int, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	blocksAdded := 0
	blocksSkipped := 0

	for _, block := range blocks {
		// Check if we already have this block
		existingBlock, err := bc.storage.GetBlock(block.Index)
		if err == nil && existingBlock != nil {
			if existingBlock.Hash == block.Hash {
				blocksSkipped++
				continue
			}
		}

		// Validate block
		if err := block.ValidateBlockWithThreshold(bc.PostThreshold); err != nil {
			return blocksAdded, blocksSkipped, fmt.Errorf("invalid block %d: %w", block.Index, err)
		}

		// Check block index continuity
		if block.Index > 0 {
			prevBlock, err := bc.storage.GetBlock(block.Index - 1)
			if err != nil {
				return blocksAdded, blocksSkipped, fmt.Errorf("missing previous block %d: %w", block.Index-1, err)
			}
			if block.PrevHash != prevBlock.Hash {
				return blocksAdded, blocksSkipped, fmt.Errorf("previous hash mismatch at block %d", block.Index)
			}
		}

		// Save block to storage
		if err := bc.storage.SaveBlock(block); err != nil {
			return blocksAdded, blocksSkipped, fmt.Errorf("failed to save block %d: %w", block.Index, err)
		}

		blocksAdded++
	}

	return blocksAdded, blocksSkipped, nil
}

// shouldCreateTimeBasedBlock checks if we should create a block based on time interval
func (bc *Blockchain) shouldCreateTimeBasedBlock() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	// Check if enough time has passed since the last block
	return time.Since(bc.lastBlockTime) >= bc.TimeInterval
}

// createTimeBasedBlock creates a new block based on time interval (empty block for mining rewards)
func (bc *Blockchain) createTimeBasedBlock() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Get the latest block
	latestBlock, err := bc.storage.GetLatestBlock()
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	// Create a new empty block for mining rewards
	newBlock := &chain.Block{
		Index:     latestBlock.Index + 1,
		Timestamp: time.Now().Unix(),
		PrevHash:  latestBlock.Hash,
		Posts:     []chain.Post{},        // Empty block - no posts
		Transfers: []chain.Transfer{},    // No transfers
		StateRoot: latestBlock.StateRoot, // Keep same state root since no changes
		CharCount: 0,                     // No characters in empty block
	}

	// Calculate and set block hash
	newBlock.SetHash()

	// Save the block
	if err := bc.storage.SaveBlock(newBlock); err != nil {
		return fmt.Errorf("failed to save time-based block: %w", err)
	}

	// Update last block time
	bc.lastBlockTime = time.Now()

	fmt.Printf("Created time-based block %d (empty block for mining rewards)\n", newBlock.Index)
	return nil
}

// timeBasedBlockLoop is a background goroutine to check for time-based blocks and create them
func (bc *Blockchain) timeBasedBlockLoop() {
	for {
		if bc.shouldCreateTimeBasedBlock() {
			if err := bc.createTimeBasedBlock(); err != nil {
				fmt.Printf("Error creating time-based block: %v\n", err)
			}
		}
		time.Sleep(1 * time.Minute) // Check every minute
	}
}
