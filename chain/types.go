package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// Post represents a user-submitted text post on the blockchain
type Post struct {
	Author    string `json:"author"`    // public key (wallet address)
	Signature string `json:"signature"` // signed content hash
	Content   string `json:"content"`   // text (counted in chars)
	Timestamp int64  `json:"timestamp"` // Unix timestamp
	Hash      string `json:"hash"`      // hash of the post
}

// WalletState represents the state of a wallet at a given block
type WalletState struct {
	Address    string `json:"address"`      // Wallet address
	Balance    int    `json:"balance"`      // Character balance
	Nonce      int64  `json:"nonce"`        // Transaction nonce
	LastTxTime int64  `json:"last_tx_time"` // Timestamp of last transaction
}

// StateRoot represents the global state at a given block
type StateRoot struct {
	Wallets    []WalletState `json:"wallets"`     // Sorted wallet states
	Hash       string        `json:"hash"`        // Hash of the state root
	BlockIndex int           `json:"block_index"` // Block this state belongs to
}

// Block represents a block in the TruthChain blockchain
type Block struct {
	Index     int        `json:"index"`      // block index
	Timestamp int64      `json:"timestamp"`  // Unix timestamp
	PrevHash  string     `json:"prev_hash"`  // hash of previous block
	Hash      string     `json:"hash"`       // hash of this block
	Posts     []Post     `json:"posts"`      // posts in this block
	Transfers []Transfer `json:"transfers"`  // transfers in this block
	StateRoot *StateRoot `json:"state_root"` // global state root
	CharCount int        `json:"char_count"` // total characters in this block
}

// PostRequest represents a request to create a new post
type PostRequest struct {
	Content   string `json:"content"`
	Signature string `json:"signature"`
	Author    string `json:"author"`
}

// BlockHeader represents the header information of a block
type BlockHeader struct {
	Index     int    `json:"index"`
	Timestamp int64  `json:"timestamp"`
	PrevHash  string `json:"prev_hash"`
	Hash      string `json:"hash"`
	CharCount int    `json:"char_count"`
	PostCount int    `json:"post_count"`
}

// CalculateHash calculates the hash of a post
func (p *Post) CalculateHash() string {
	// Create a deterministic string representation
	data := fmt.Sprintf("%s%s%d", p.Author, p.Content, p.Timestamp)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SetHash sets the hash field of the post
func (p *Post) SetHash() {
	p.Hash = p.CalculateHash()
}

// ValidatePost validates a post structure
func (p *Post) ValidatePost() error {
	if p.Author == "" {
		return fmt.Errorf("post author cannot be empty")
	}
	if p.Content == "" {
		return fmt.Errorf("post content cannot be empty")
	}
	if p.Signature == "" {
		return fmt.Errorf("post signature cannot be empty")
	}
	if p.Timestamp <= 0 {
		return fmt.Errorf("post timestamp must be positive")
	}
	if len(p.Content) > 10000 { // Reasonable limit
		return fmt.Errorf("post content too long: %d characters", len(p.Content))
	}
	return nil
}

// GetCharacterCount returns the number of characters in the post
func (p *Post) GetCharacterCount() int {
	return len(p.Content)
}

// CalculateHash calculates the hash of a state root
func (sr *StateRoot) CalculateHash() string {
	// Sort wallets by address for deterministic hashing
	sortedWallets := make([]WalletState, len(sr.Wallets))
	copy(sortedWallets, sr.Wallets)

	sort.Slice(sortedWallets, func(i, j int) bool {
		return sortedWallets[i].Address < sortedWallets[j].Address
	})

	// Create deterministic JSON representation
	stateData := map[string]interface{}{
		"block_index": sr.BlockIndex,
		"wallets":     sortedWallets,
	}

	jsonData, err := json.Marshal(stateData)
	if err != nil {
		return "" // This shouldn't happen with valid data
	}

	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:])
}

// SetHash sets the hash field of the state root
func (sr *StateRoot) SetHash() {
	sr.Hash = sr.CalculateHash()
}

// GetWalletState returns the state for a specific wallet
func (sr *StateRoot) GetWalletState(address string) (*WalletState, bool) {
	for _, wallet := range sr.Wallets {
		if wallet.Address == address {
			return &wallet, true
		}
	}
	return nil, false
}

// UpdateWalletState updates or adds a wallet state
func (sr *StateRoot) UpdateWalletState(wallet WalletState) {
	for i, existing := range sr.Wallets {
		if existing.Address == wallet.Address {
			sr.Wallets[i] = wallet
			return
		}
	}
	sr.Wallets = append(sr.Wallets, wallet)
}

// CalculateHash calculates the hash of a block
func (b *Block) CalculateHash() string {
	// Create a deterministic string representation
	data := fmt.Sprintf("%d%d%s%d", b.Index, b.Timestamp, b.PrevHash, b.CharCount)

	// Include post hashes for immutability
	for _, post := range b.Posts {
		data += post.Hash
	}

	// Include transfer hashes for immutability
	for _, transfer := range b.Transfers {
		data += transfer.Hash
	}

	// Include state root hash
	if b.StateRoot != nil {
		data += b.StateRoot.Hash
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SetHash sets the hash field of the block
func (b *Block) SetHash() {
	b.Hash = b.CalculateHash()
}

// ValidateBlock validates a block structure
func (b *Block) ValidateBlock() error {
	if b.Index < 0 {
		return fmt.Errorf("block index cannot be negative")
	}
	if b.Timestamp <= 0 {
		return fmt.Errorf("block timestamp must be positive")
	}
	if b.PrevHash == "" && b.Index != 0 {
		return fmt.Errorf("genesis block must have empty prev_hash")
	}
	if b.PrevHash != "" && b.Index == 0 {
		return fmt.Errorf("non-genesis block must have prev_hash")
	}

	// Validate all posts in the block
	for i, post := range b.Posts {
		if err := post.ValidatePost(); err != nil {
			return fmt.Errorf("invalid post at index %d: %v", i, err)
		}
	}

	// Validate all transfers in the block
	for i, transfer := range b.Transfers {
		if err := transfer.Validate(); err != nil {
			return fmt.Errorf("invalid transfer at index %d: %v", i, err)
		}
	}

	// Validate state root if present
	if b.StateRoot != nil {
		if b.StateRoot.BlockIndex != b.Index {
			return fmt.Errorf("state root block index mismatch: expected %d, got %d", b.Index, b.StateRoot.BlockIndex)
		}

		// Verify state root hash
		calculatedHash := b.StateRoot.CalculateHash()
		if b.StateRoot.Hash != calculatedHash {
			return fmt.Errorf("state root hash mismatch: expected %s, got %s", calculatedHash, b.StateRoot.Hash)
		}
	}

	// Validate character count
	calculatedCharCount := 0
	for _, post := range b.Posts {
		calculatedCharCount += post.GetCharacterCount()
	}
	if calculatedCharCount != b.CharCount {
		return fmt.Errorf("block char_count mismatch: expected %d, got %d", calculatedCharCount, b.CharCount)
	}

	return nil
}

// ValidateBlockWithThreshold validates a block structure with post count threshold rules
func (b *Block) ValidateBlockWithThreshold(postThreshold int) error {
	// First run basic validation
	if err := b.ValidateBlock(); err != nil {
		return err
	}

	// Genesis block is always valid (no posts)
	if b.Index == 0 {
		return nil
	}

	// Enforce post count threshold rules
	postCount := len(b.Posts)

	// Block must have exactly the threshold number of posts (unless it's a forced block)
	if postCount != postThreshold {
		return fmt.Errorf("block %d has invalid post count: expected %d, got %d (fork protection)",
			b.Index, postThreshold, postCount)
	}

	// Additional security: ensure posts are not empty
	if postCount == 0 {
		return fmt.Errorf("block %d has no posts (fork protection)", b.Index)
	}

	// Validate that all posts have valid content
	for i, post := range b.Posts {
		if post.Content == "" {
			return fmt.Errorf("block %d has empty post at index %d (fork protection)", b.Index, i)
		}
		if post.Author == "" {
			return fmt.Errorf("block %d has post without author at index %d (fork protection)", b.Index, i)
		}
	}

	return nil
}

// GetCharacterCount returns the total number of characters in the block
func (b *Block) GetCharacterCount() int {
	count := 0
	for _, post := range b.Posts {
		count += post.GetCharacterCount()
	}
	return count
}

// GetPostCount returns the number of posts in the block
func (b *Block) GetPostCount() int {
	return len(b.Posts)
}

// GetTransferCount returns the number of transfers in the block
func (b *Block) GetTransferCount() int {
	return len(b.Transfers)
}

// AddPost adds a post to the block and updates the character count
func (b *Block) AddPost(post Post) error {
	if err := post.ValidatePost(); err != nil {
		return fmt.Errorf("invalid post: %w", err)
	}

	// Set the post hash if not already set
	if post.Hash == "" {
		post.SetHash()
	}

	b.Posts = append(b.Posts, post)
	b.CharCount = b.GetCharacterCount()

	return nil
}

// AddTransfer adds a transfer to the block
func (b *Block) AddTransfer(transfer Transfer) error {
	if err := transfer.Validate(); err != nil {
		return fmt.Errorf("invalid transfer: %w", err)
	}

	// Set the transfer hash if not already set
	if transfer.Hash == "" {
		hash, err := transfer.CalculateHash()
		if err != nil {
			return fmt.Errorf("failed to calculate transfer hash: %w", err)
		}
		transfer.Hash = hash
	}

	b.Transfers = append(b.Transfers, transfer)
	return nil
}

// ToJSON converts the block to JSON
func (b *Block) ToJSON() ([]byte, error) {
	return json.Marshal(b)
}

// FromJSON creates a block from JSON
func FromJSON(data []byte) (*Block, error) {
	var block Block
	err := json.Unmarshal(data, &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

// CreateGenesisBlock creates the first block in the chain
func CreateGenesisBlock() *Block {
	block := &Block{
		Index:     0,
		Timestamp: MainnetGenesisTimestamp,
		PrevHash:  "",
		Posts:     []Post{},
		Transfers: []Transfer{},
		StateRoot: &StateRoot{
			Wallets:    []WalletState{},
			BlockIndex: 0,
		},
		CharCount: 0,
	}
	block.StateRoot.SetHash()
	block.SetHash()
	return block
}

// CreateBlock creates a new block with the given posts and transfers
func CreateBlock(index int, prevHash string, posts []Post, transfers []Transfer, stateRoot *StateRoot) *Block {
	block := &Block{
		Index:     index,
		Timestamp: time.Now().Unix(),
		PrevHash:  prevHash,
		Posts:     posts,
		Transfers: transfers,
		StateRoot: stateRoot,
		CharCount: 0,
	}

	// Calculate character count
	for _, post := range posts {
		block.CharCount += post.GetCharacterCount()
	}

	// Set state root hash if provided
	if block.StateRoot != nil {
		block.StateRoot.SetHash()
	}

	block.SetHash()
	return block
}
