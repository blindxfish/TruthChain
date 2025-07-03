package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/wallet"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/ripemd160"
)

// Blockchain represents the TruthChain blockchain
type Blockchain struct {
	Blocks             []*Block     `json:"blocks"`
	PendingPosts       []Post       `json:"pending_posts"`
	CharacterThreshold int          `json:"character_threshold"` // Characters needed to create a block
	mu                 sync.RWMutex `json:"-"`
}

// NewBlockchain creates a new blockchain with genesis block
func NewBlockchain(characterThreshold int) *Blockchain {
	bc := &Blockchain{
		Blocks:             []*Block{},
		PendingPosts:       []Post{},
		CharacterThreshold: characterThreshold,
	}

	// Create genesis block
	genesis := CreateGenesisBlock()
	bc.Blocks = append(bc.Blocks, genesis)

	return bc
}

// GetLatestBlock returns the most recent block
func (bc *Blockchain) GetLatestBlock() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if len(bc.Blocks) == 0 {
		return nil
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

// GetBlockByIndex returns a block by its index
func (bc *Blockchain) GetBlockByIndex(index int) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if index < 0 || index >= len(bc.Blocks) {
		return nil
	}
	return bc.Blocks[index]
}

// GetBlockByHash returns a block by its hash
func (bc *Blockchain) GetBlockByHash(hash string) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for _, block := range bc.Blocks {
		if block.Hash == hash {
			return block
		}
	}
	return nil
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

// AddPost adds a new post to the pending posts
func (bc *Blockchain) AddPost(post Post) error {
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

	// Check for duplicate posts
	for _, existingPost := range bc.PendingPosts {
		if existingPost.Hash == post.Hash {
			return fmt.Errorf("duplicate post: %s", post.Hash)
		}
	}

	// Add to pending posts
	bc.PendingPosts = append(bc.PendingPosts, post)

	// Calculate pending character count directly (avoid lock contention)
	pendingCharCount := 0
	for _, pendingPost := range bc.PendingPosts {
		pendingCharCount += pendingPost.GetCharacterCount()
	}

	// Check if we should create a new block
	if pendingCharCount >= bc.CharacterThreshold {
		return bc.createBlockFromPending()
	}

	return nil
}

// CreatePost creates a new post from content and wallet
func (bc *Blockchain) CreatePost(content string, w *wallet.Wallet) (*Post, error) {
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
	post := Post{
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
func (bc *Blockchain) VerifyPostSignature(post Post) (bool, error) {
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

// deriveAddressFromPubKey derives a TruthChain address from a public key
func deriveAddressFromPubKey(pubKey *btcec.PublicKey) string {
	// Get compressed public key bytes
	pubBytes := pubKey.SerializeCompressed()

	// SHA256 hash
	sha := sha256.Sum256(pubBytes)

	// RIPEMD160 hash
	ripemd := ripemd160.New()
	ripemd.Write(sha[:])
	hashed := ripemd.Sum(nil)

	// Create versioned payload (0x00 for TruthChain mainnet)
	versionedPayload := append([]byte{0x00}, hashed...)

	// Double SHA256 for checksum
	checksum := sha256.Sum256(versionedPayload)
	checksum = sha256.Sum256(checksum[:])

	// Append first 4 bytes of checksum
	finalPayload := append(versionedPayload, checksum[:4]...)

	// Encode as Base58Check
	return base58Encode(finalPayload)
}

// base58Encode encodes bytes to Base58 (simplified version)
func base58Encode(data []byte) string {
	// This is a simplified Base58 implementation
	// In production, you'd use a proper Base58 library
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	var result string
	value := new(big.Int).SetBytes(data)
	base := big.NewInt(58)

	for value.Cmp(big.NewInt(0)) > 0 {
		mod := new(big.Int)
		value.DivMod(value, base, mod)
		result = string(alphabet[mod.Int64()]) + result
	}

	// Handle leading zeros
	for _, b := range data {
		if b == 0 {
			result = "1" + result
		} else {
			break
		}
	}

	return result
}

// createBlockFromPending creates a new block from pending posts
func (bc *Blockchain) createBlockFromPending() error {
	if len(bc.PendingPosts) == 0 {
		return fmt.Errorf("no pending posts to create block")
	}

	// Get the latest block directly (we already have write lock)
	if len(bc.Blocks) == 0 {
		return fmt.Errorf("no latest block found")
	}
	latestBlock := bc.Blocks[len(bc.Blocks)-1]

	// Create new block
	newBlock := CreateBlock(
		latestBlock.Index+1,
		latestBlock.Hash,
		bc.PendingPosts,
		[]Transfer{}, // No transfers in this simple implementation
		nil,          // No state root in this simple implementation
	)

	// Validate the new block
	if err := newBlock.ValidateBlock(); err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}

	// Add block to chain
	bc.Blocks = append(bc.Blocks, newBlock)

	// Clear pending posts
	bc.PendingPosts = []Post{}

	return nil
}

// ForceCreateBlock forces the creation of a block from pending posts
func (bc *Blockchain) ForceCreateBlock() error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.createBlockFromPending()
}

// GetChainLength returns the number of blocks in the chain
func (bc *Blockchain) GetChainLength() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return len(bc.Blocks)
}

// AddBlock adds a block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate the block
	if err := block.ValidateBlock(); err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}

	// Check if block index is correct
	expectedIndex := len(bc.Blocks)
	if block.Index != expectedIndex {
		return fmt.Errorf("block index mismatch: expected %d, got %d", expectedIndex, block.Index)
	}

	// Check previous hash (except genesis)
	if block.Index > 0 {
		if block.PrevHash != bc.Blocks[block.Index-1].Hash {
			return fmt.Errorf("previous hash mismatch")
		}
	}

	// Add block to chain
	bc.Blocks = append(bc.Blocks, block)

	return nil
}

// GetTotalCharacterCount returns the total characters in all blocks
func (bc *Blockchain) GetTotalCharacterCount() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	count := 0
	for _, block := range bc.Blocks {
		count += block.GetCharacterCount()
	}
	return count
}

// GetTotalPostCount returns the total posts in all blocks
func (bc *Blockchain) GetTotalPostCount() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	count := 0
	for _, block := range bc.Blocks {
		count += block.GetPostCount()
	}
	return count
}

// ValidateChain validates the entire blockchain
func (bc *Blockchain) ValidateChain() error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if len(bc.Blocks) == 0 {
		return fmt.Errorf("blockchain is empty")
	}

	// Validate each block
	for i, block := range bc.Blocks {
		if err := block.ValidateBlock(); err != nil {
			return fmt.Errorf("invalid block at index %d: %w", i, err)
		}

		// Check block index
		if block.Index != i {
			return fmt.Errorf("block index mismatch at %d: expected %d, got %d", i, i, block.Index)
		}

		// Check previous hash (except genesis)
		if i > 0 {
			if block.PrevHash != bc.Blocks[i-1].Hash {
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

// GetBlockchainInfo returns information about the blockchain
func (bc *Blockchain) GetBlockchainInfo() map[string]interface{} {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	latestBlock := bc.GetLatestBlock()

	info := map[string]interface{}{
		"chain_length":            len(bc.Blocks),
		"total_character_count":   bc.GetTotalCharacterCount(),
		"total_post_count":        bc.GetTotalPostCount(),
		"pending_post_count":      len(bc.PendingPosts),
		"pending_character_count": bc.GetPendingCharacterCount(),
		"character_threshold":     bc.CharacterThreshold,
	}

	if latestBlock != nil {
		info["latest_block_index"] = latestBlock.Index
		info["latest_block_hash"] = latestBlock.Hash
		info["latest_block_timestamp"] = latestBlock.Timestamp
	}

	return info
}
