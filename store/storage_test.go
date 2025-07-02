package store

import (
	"os"
	"testing"
	"time"

	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/wallet"
)

func TestNewBoltDBStorage(t *testing.T) {
	// Create temporary database file
	dbPath := "test_storage.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test that storage is not nil
	if storage == nil {
		t.Fatal("Storage is nil")
	}
}

func TestSaveAndGetBlock(t *testing.T) {
	// Create temporary database file
	dbPath := "test_blocks.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test block
	block := &chain.Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		PrevHash:  "0000000000000000000000000000000000000000000000000000000000000000",
		Posts:     []chain.Post{},
		CharCount: 0,
	}
	block.SetHash()

	// Save block
	if err := storage.SaveBlock(block); err != nil {
		t.Fatalf("Failed to save block: %v", err)
	}

	// Retrieve block
	retrievedBlock, err := storage.GetBlock(1)
	if err != nil {
		t.Fatalf("Failed to get block: %v", err)
	}

	// Verify block data
	if retrievedBlock.Index != block.Index {
		t.Errorf("Block index mismatch: expected %d, got %d", block.Index, retrievedBlock.Index)
	}
	if retrievedBlock.Hash != block.Hash {
		t.Errorf("Block hash mismatch: expected %s, got %s", block.Hash, retrievedBlock.Hash)
	}
	if retrievedBlock.PrevHash != block.PrevHash {
		t.Errorf("Block prev hash mismatch: expected %s, got %s", block.PrevHash, retrievedBlock.PrevHash)
	}
}

func TestGetLatestBlock(t *testing.T) {
	// Create temporary database file
	dbPath := "test_latest.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test getting latest block when no blocks exist
	_, err = storage.GetLatestBlock()
	if err == nil {
		t.Error("Expected error when no blocks exist")
	}

	// Create and save multiple blocks
	blocks := []*chain.Block{
		{
			Index:     0,
			Timestamp: time.Now().Unix(),
			PrevHash:  "",
			Posts:     []chain.Post{},
			CharCount: 0,
		},
		{
			Index:     1,
			Timestamp: time.Now().Unix(),
			PrevHash:  "0000000000000000000000000000000000000000000000000000000000000000",
			Posts:     []chain.Post{},
			CharCount: 0,
		},
		{
			Index:     2,
			Timestamp: time.Now().Unix(),
			PrevHash:  "1111111111111111111111111111111111111111111111111111111111111111",
			Posts:     []chain.Post{},
			CharCount: 0,
		},
	}

	for _, block := range blocks {
		block.SetHash()
		if err := storage.SaveBlock(block); err != nil {
			t.Fatalf("Failed to save block %d: %v", block.Index, err)
		}
	}

	// Get latest block
	latestBlock, err := storage.GetLatestBlock()
	if err != nil {
		t.Fatalf("Failed to get latest block: %v", err)
	}

	// Verify it's the last block we saved
	if latestBlock.Index != 2 {
		t.Errorf("Expected latest block index 2, got %d", latestBlock.Index)
	}
}

func TestGetBlockCount(t *testing.T) {
	// Create temporary database file
	dbPath := "test_count.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test count when no blocks exist
	count, err := storage.GetBlockCount()
	if err != nil {
		t.Fatalf("Failed to get block count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 blocks, got %d", count)
	}

	// Create and save blocks
	for i := 0; i < 5; i++ {
		block := &chain.Block{
			Index:     i,
			Timestamp: time.Now().Unix(),
			PrevHash:  "",
			Posts:     []chain.Post{},
			CharCount: 0,
		}
		block.SetHash()
		if err := storage.SaveBlock(block); err != nil {
			t.Fatalf("Failed to save block %d: %v", i, err)
		}
	}

	// Get count
	count, err = storage.GetBlockCount()
	if err != nil {
		t.Fatalf("Failed to get block count: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 blocks, got %d", count)
	}
}

func TestSaveAndGetPost(t *testing.T) {
	// Create temporary database file
	dbPath := "test_posts.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create a test post
	post := chain.Post{
		Author:    wallet.GetAddress(),
		Content:   "Test post content",
		Timestamp: time.Now().Unix(),
	}
	post.SetHash()

	// Sign the post
	postData := post.Author + post.Content + string(rune(post.Timestamp))
	signature, err := wallet.Sign([]byte(postData))
	if err != nil {
		t.Fatalf("Failed to sign post: %v", err)
	}
	post.Signature = string(signature)

	// Save post
	if err := storage.SavePost(post); err != nil {
		t.Fatalf("Failed to save post: %v", err)
	}

	// Retrieve post
	retrievedPost, err := storage.GetPost(post.Hash)
	if err != nil {
		t.Fatalf("Failed to get post: %v", err)
	}

	// Verify post data
	if retrievedPost.Author != post.Author {
		t.Errorf("Post author mismatch: expected %s, got %s", post.Author, retrievedPost.Author)
	}
	if retrievedPost.Content != post.Content {
		t.Errorf("Post content mismatch: expected %s, got %s", post.Content, retrievedPost.Content)
	}
	if retrievedPost.Hash != post.Hash {
		t.Errorf("Post hash mismatch: expected %s, got %s", post.Hash, retrievedPost.Hash)
	}
}

func TestPostExists(t *testing.T) {
	// Create temporary database file
	dbPath := "test_exists.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test non-existent post
	exists, err := storage.PostExists("nonexistent")
	if err != nil {
		t.Fatalf("Failed to check post existence: %v", err)
	}
	if exists {
		t.Error("Expected post to not exist")
	}

	// Create and save a post
	post := chain.Post{
		Author:    "test_address",
		Content:   "Test content",
		Timestamp: time.Now().Unix(),
	}
	post.SetHash()

	if err := storage.SavePost(post); err != nil {
		t.Fatalf("Failed to save post: %v", err)
	}

	// Test existing post
	exists, err = storage.PostExists(post.Hash)
	if err != nil {
		t.Fatalf("Failed to check post existence: %v", err)
	}
	if !exists {
		t.Error("Expected post to exist")
	}
}

func TestCharacterBalance(t *testing.T) {
	// Create temporary database file
	dbPath := "test_balance.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	testAddress := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"

	// Test initial balance (should be 0)
	balance, err := storage.GetCharacterBalance(testAddress)
	if err != nil {
		t.Fatalf("Failed to get initial balance: %v", err)
	}
	if balance != 0 {
		t.Errorf("Expected initial balance 0, got %d", balance)
	}

	// Add characters
	if err := storage.UpdateCharacterBalance(testAddress, 1000); err != nil {
		t.Fatalf("Failed to add characters: %v", err)
	}

	// Check balance
	balance, err = storage.GetCharacterBalance(testAddress)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}
	if balance != 1000 {
		t.Errorf("Expected balance 1000, got %d", balance)
	}

	// Add more characters
	if err := storage.UpdateCharacterBalance(testAddress, 500); err != nil {
		t.Fatalf("Failed to add more characters: %v", err)
	}

	// Check updated balance
	balance, err = storage.GetCharacterBalance(testAddress)
	if err != nil {
		t.Fatalf("Failed to get updated balance: %v", err)
	}
	if balance != 1500 {
		t.Errorf("Expected balance 1500, got %d", balance)
	}

	// Subtract characters
	if err := storage.UpdateCharacterBalance(testAddress, -300); err != nil {
		t.Fatalf("Failed to subtract characters: %v", err)
	}

	// Check final balance
	balance, err = storage.GetCharacterBalance(testAddress)
	if err != nil {
		t.Fatalf("Failed to get final balance: %v", err)
	}
	if balance != 1200 {
		t.Errorf("Expected balance 1200, got %d", balance)
	}

	// Test insufficient balance
	err = storage.UpdateCharacterBalance(testAddress, -2000)
	if err == nil {
		t.Error("Expected error for insufficient balance")
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Create temporary database file
	dbPath := "test_concurrent.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Test concurrent balance updates
	testAddress := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(amount int) {
			if err := storage.UpdateCharacterBalance(testAddress, amount); err != nil {
				t.Errorf("Failed to update balance by %d: %v", amount, err)
			}
			done <- true
		}(i + 1) // Add 1-10 characters
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final balance (should be sum of 1+2+...+10 = 55)
	balance, err := storage.GetCharacterBalance(testAddress)
	if err != nil {
		t.Fatalf("Failed to get final balance: %v", err)
	}
	expectedBalance := 55 // 1+2+3+...+10
	if balance != expectedBalance {
		t.Errorf("Expected balance %d, got %d", expectedBalance, balance)
	}
}
