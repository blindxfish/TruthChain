package blockchain

import (
	"fmt"
	"os"
	"testing"

	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

func TestNewBlockchain(t *testing.T) {
	// Create temporary database file
	dbPath := "test_new_blockchain.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain with mainnet threshold
	bc, err := NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Test that blockchain is not nil
	if bc == nil {
		t.Fatal("Blockchain is nil")
	}

	// Test that genesis block was created
	latestBlock, err := bc.GetLatestBlock()
	if err != nil {
		t.Fatalf("Failed to get latest block: %v", err)
	}

	if latestBlock.Index != 0 {
		t.Errorf("Expected genesis block index 0, got %d", latestBlock.Index)
	}
}

func TestAddPost(t *testing.T) {
	// Create temporary database file
	dbPath := "test_add_post.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain with mainnet threshold
	bc, err := NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create a test wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create a post
	post, err := bc.CreatePost("Test post content", wallet)
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	// Add post to blockchain
	if err := bc.AddPost(*post); err != nil {
		t.Fatalf("Failed to add post: %v", err)
	}

	// Verify post was added to pending posts
	pendingCount := bc.GetPendingPostCount()
	if pendingCount != 1 {
		t.Errorf("Expected 1 pending post, got %d", pendingCount)
	}

	// Verify post was saved to storage
	exists, err := storage.PostExists(post.Hash)
	if err != nil {
		t.Fatalf("Failed to check post existence: %v", err)
	}
	if !exists {
		t.Error("Post was not saved to storage")
	}
}

func TestCreateBlockFromPending(t *testing.T) {
	// Create temporary database file
	dbPath := "test_create_block.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain with mainnet threshold
	bc, err := NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create a test wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create and add posts until block is created (need 5 posts for mainnet)
	for i := 0; i < 5; i++ {
		content := fmt.Sprintf("Test post %d with enough characters to trigger block creation", i)
		post, err := bc.CreatePost(content, wallet)
		if err != nil {
			t.Fatalf("Failed to create post %d: %v", i, err)
		}

		if err := bc.AddPost(*post); err != nil {
			t.Fatalf("Failed to add post %d: %v", i, err)
		}
	}

	// Verify block was created
	chainLength, err := bc.GetChainLength()
	if err != nil {
		t.Fatalf("Failed to get chain length: %v", err)
	}

	// With 5 posts, we expect exactly 2 blocks (genesis + 1 new block)
	if chainLength != 2 {
		t.Errorf("Expected exactly 2 blocks, got %d", chainLength)
	}

	// Verify pending posts were cleared
	pendingCount := bc.GetPendingPostCount()
	if pendingCount != 0 {
		t.Errorf("Expected 0 pending posts, got %d", pendingCount)
	}
}

func TestCharacterBalance(t *testing.T) {
	// Create temporary database file
	dbPath := "test_balance.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain with mainnet threshold
	bc, err := NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	testAddress := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"

	// Test initial balance
	balance, err := bc.GetCharacterBalance(testAddress)
	if err != nil {
		t.Fatalf("Failed to get initial balance: %v", err)
	}
	if balance != 0 {
		t.Errorf("Expected initial balance 0, got %d", balance)
	}

	// Add characters
	if err := bc.UpdateCharacterBalance(testAddress, 1000); err != nil {
		t.Fatalf("Failed to add characters: %v", err)
	}

	// Check balance
	balance, err = bc.GetCharacterBalance(testAddress)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}
	if balance != 1000 {
		t.Errorf("Expected balance 1000, got %d", balance)
	}
}

func TestValidateChain(t *testing.T) {
	// Create temporary database file
	dbPath := "test_validate.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain with mainnet threshold
	bc, err := NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create a test wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Add some posts to create blocks
	for i := 0; i < 2; i++ {
		content := fmt.Sprintf("Test post %d with enough characters", i)
		post, err := bc.CreatePost(content, wallet)
		if err != nil {
			t.Fatalf("Failed to create post %d: %v", i, err)
		}

		if err := bc.AddPost(*post); err != nil {
			t.Fatalf("Failed to add post %d: %v", i, err)
		}
	}

	// Validate the chain
	if err := bc.ValidateChain(); err != nil {
		t.Fatalf("Chain validation failed: %v", err)
	}
}

func TestGetBlockchainInfo(t *testing.T) {
	// Create temporary database file
	dbPath := "test_info.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain
	bc, err := NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Get blockchain info
	info, err := bc.GetBlockchainInfo()
	if err != nil {
		t.Fatalf("Failed to get blockchain info: %v", err)
	}

	// Verify required fields
	requiredFields := []string{
		"chain_length",
		"total_character_count",
		"total_post_count",
		"pending_post_count",
		"pending_character_count",
		"post_threshold",
		"latest_block_index",
		"latest_block_hash",
		"latest_block_timestamp",
	}

	for _, field := range requiredFields {
		if _, exists := info[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Verify initial values
	if info["chain_length"].(int) != 1 { // Genesis block
		t.Errorf("Expected chain length 1, got %d", info["chain_length"])
	}

	if info["latest_block_index"].(int) != 0 { // Genesis block
		t.Errorf("Expected latest block index 0, got %d", info["latest_block_index"])
	}
}

func TestDuplicatePostRejection(t *testing.T) {
	// Create temporary database file
	dbPath := "test_duplicate.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain
	bc, err := NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create a test wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create a post
	post, err := bc.CreatePost("Test post content", wallet)
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	// Add post first time
	if err := bc.AddPost(*post); err != nil {
		t.Fatalf("Failed to add post first time: %v", err)
	}

	// Try to add the same post again
	if err := bc.AddPost(*post); err == nil {
		t.Error("Expected error when adding duplicate post")
	}
}
