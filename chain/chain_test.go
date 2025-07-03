package chain

import (
	"testing"
	"time"

	"github.com/blindxfish/truthchain/wallet"
)

func TestPostCreation(t *testing.T) {
	post := Post{
		Author:    "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		Signature: "test_signature",
		Content:   "Hello, TruthChain!",
		Timestamp: time.Now().Unix(),
	}

	// Test validation
	if err := post.ValidatePost(); err != nil {
		t.Errorf("Valid post failed validation: %v", err)
	}

	// Test hash calculation
	post.SetHash()
	if post.Hash == "" {
		t.Error("Post hash is empty")
	}

	// Test character count
	expectedCount := len("Hello, TruthChain!")
	if post.GetCharacterCount() != expectedCount {
		t.Errorf("Expected character count %d, got %d", expectedCount, post.GetCharacterCount())
	}
}

func TestPostValidation(t *testing.T) {
	// Test empty author
	post := Post{
		Author:    "",
		Signature: "test",
		Content:   "test",
		Timestamp: time.Now().Unix(),
	}
	if err := post.ValidatePost(); err == nil {
		t.Error("Post with empty author should fail validation")
	}

	// Test empty content
	post = Post{
		Author:    "test",
		Signature: "test",
		Content:   "",
		Timestamp: time.Now().Unix(),
	}
	if err := post.ValidatePost(); err == nil {
		t.Error("Post with empty content should fail validation")
	}

	// Test empty signature
	post = Post{
		Author:    "test",
		Signature: "",
		Content:   "test",
		Timestamp: time.Now().Unix(),
	}
	if err := post.ValidatePost(); err == nil {
		t.Error("Post with empty signature should fail validation")
	}

	// Test invalid timestamp
	post = Post{
		Author:    "test",
		Signature: "test",
		Content:   "test",
		Timestamp: 0,
	}
	if err := post.ValidatePost(); err == nil {
		t.Error("Post with invalid timestamp should fail validation")
	}
}

func TestBlockCreation(t *testing.T) {
	// Create test posts
	posts := []Post{
		{
			Author:    "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			Signature: "sig1",
			Content:   "First post",
			Timestamp: time.Now().Unix(),
		},
		{
			Author:    "1B2zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			Signature: "sig2",
			Content:   "Second post",
			Timestamp: time.Now().Unix(),
		},
	}

	// Set hashes for posts
	for i := range posts {
		posts[i].SetHash()
	}

	// Create block
	block := CreateBlock(1, "prev_hash", posts, []Transfer{}, nil)

	// Test block properties
	if block.Index != 1 {
		t.Errorf("Expected block index 1, got %d", block.Index)
	}
	if block.PrevHash != "prev_hash" {
		t.Errorf("Expected prev_hash 'prev_hash', got %s", block.PrevHash)
	}
	if len(block.Posts) != 2 {
		t.Errorf("Expected 2 posts, got %d", len(block.Posts))
	}

	// Test character count
	expectedCount := len("First post") + len("Second post")
	if block.GetCharacterCount() != expectedCount {
		t.Errorf("Expected character count %d, got %d", expectedCount, block.GetCharacterCount())
	}

	// Test validation
	if err := block.ValidateBlock(); err != nil {
		t.Errorf("Valid block failed validation: %v", err)
	}
}

func TestGenesisBlock(t *testing.T) {
	genesis := CreateGenesisBlock()

	// Test genesis block properties
	if genesis.Index != 0 {
		t.Errorf("Genesis block should have index 0, got %d", genesis.Index)
	}
	if genesis.PrevHash != "" {
		t.Errorf("Genesis block should have empty prev_hash, got %s", genesis.PrevHash)
	}
	if len(genesis.Posts) != 0 {
		t.Errorf("Genesis block should have no posts, got %d", len(genesis.Posts))
	}
	if genesis.CharCount != 0 {
		t.Errorf("Genesis block should have 0 characters, got %d", genesis.CharCount)
	}

	// Test validation
	if err := genesis.ValidateBlock(); err != nil {
		t.Errorf("Genesis block failed validation: %v", err)
	}
}

func TestBlockValidation(t *testing.T) {
	// Test negative index
	block := &Block{
		Index:     -1,
		Timestamp: time.Now().Unix(),
		PrevHash:  "",
		Posts:     []Post{},
		CharCount: 0,
	}
	if err := block.ValidateBlock(); err == nil {
		t.Error("Block with negative index should fail validation")
	}

	// Test invalid timestamp
	block = &Block{
		Index:     0,
		Timestamp: 0,
		PrevHash:  "",
		Posts:     []Post{},
		CharCount: 0,
	}
	if err := block.ValidateBlock(); err == nil {
		t.Error("Block with invalid timestamp should fail validation")
	}

	// Test non-genesis block without prev_hash
	block = &Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		PrevHash:  "",
		Posts:     []Post{},
		CharCount: 0,
	}
	if err := block.ValidateBlock(); err == nil {
		t.Error("Non-genesis block without prev_hash should fail validation")
	}
}

func TestBlockchainCreation(t *testing.T) {
	bc := NewBlockchain(1000)

	// Test initial state
	if bc.GetChainLength() != 1 {
		t.Errorf("New blockchain should have 1 block (genesis), got %d", bc.GetChainLength())
	}
	if bc.GetPendingPostCount() != 0 {
		t.Errorf("New blockchain should have 0 pending posts, got %d", bc.GetPendingPostCount())
	}
	if bc.GetPendingCharacterCount() != 0 {
		t.Errorf("New blockchain should have 0 pending characters, got %d", bc.GetPendingCharacterCount())
	}

	// Test genesis block
	genesis := bc.GetLatestBlock()
	if genesis == nil {
		t.Fatal("Genesis block should exist")
	}
	if genesis.Index != 0 {
		t.Errorf("Genesis block should have index 0, got %d", genesis.Index)
	}
}

func TestBlockchainAddPost(t *testing.T) {
	bc := NewBlockchain(100) // Low threshold for testing

	// Create a test wallet
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create and add a post
	post, err := bc.CreatePost("Test post content", w)
	if err != nil {
		t.Fatalf("Failed to create post: %v", err)
	}

	err = bc.AddPost(*post)
	if err != nil {
		t.Fatalf("Failed to add post: %v", err)
	}

	// Test pending post count
	if bc.GetPendingPostCount() != 1 {
		t.Errorf("Expected 1 pending post, got %d", bc.GetPendingPostCount())
	}

	// Test pending character count
	expectedChars := len("Test post content")
	if bc.GetPendingCharacterCount() != expectedChars {
		t.Errorf("Expected %d pending characters, got %d", expectedChars, bc.GetPendingCharacterCount())
	}
}

func TestBlockchainBlockCreation(t *testing.T) {
	bc := NewBlockchain(50) // Low threshold for testing

	// Create a test wallet
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Add posts until block is created
	posts := []string{
		"This is the first post with some content.",
		"Second post with more content to reach threshold.",
	}

	for _, content := range posts {
		post, err := bc.CreatePost(content, w)
		if err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}

		err = bc.AddPost(*post)
		if err != nil {
			t.Fatalf("Failed to add post: %v", err)
		}
	}

	// Check if block was created
	if bc.GetChainLength() != 2 { // Genesis + new block
		t.Errorf("Expected 2 blocks (genesis + new), got %d", bc.GetChainLength())
	}

	// Check pending posts should be cleared
	if bc.GetPendingPostCount() != 0 {
		t.Errorf("Pending posts should be cleared after block creation, got %d", bc.GetPendingPostCount())
	}
}

func TestBlockchainValidation(t *testing.T) {
	bc := NewBlockchain(1000)

	// Test initial validation
	if err := bc.ValidateChain(); err != nil {
		t.Errorf("Valid blockchain failed validation: %v", err)
	}

	// Test with multiple blocks
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Add some posts and force block creation
	for i := 0; i < 3; i++ {
		post, err := bc.CreatePost("Test post", w)
		if err != nil {
			t.Fatalf("Failed to create post: %v", err)
		}
		bc.AddPost(*post)
	}

	// Force create block
	err = bc.ForceCreateBlock()
	if err != nil {
		t.Fatalf("Failed to force create block: %v", err)
	}

	// Validate chain
	if err := bc.ValidateChain(); err != nil {
		t.Errorf("Valid blockchain failed validation: %v", err)
	}
}

func TestBlockchainInfo(t *testing.T) {
	bc := NewBlockchain(1000)
	info := bc.GetBlockchainInfo()

	// Test required fields
	requiredFields := []string{
		"chain_length",
		"total_character_count",
		"total_post_count",
		"pending_post_count",
		"pending_character_count",
		"character_threshold",
	}

	for _, field := range requiredFields {
		if _, exists := info[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Test initial values
	if info["chain_length"] != 1 {
		t.Errorf("Expected chain_length 1, got %v", info["chain_length"])
	}
	if info["total_post_count"] != 0 {
		t.Errorf("Expected total_post_count 0, got %v", info["total_post_count"])
	}
	if info["character_threshold"] != 1000 {
		t.Errorf("Expected character_threshold 1000, got %v", info["character_threshold"])
	}
}
