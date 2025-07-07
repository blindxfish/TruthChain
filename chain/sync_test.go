package chain

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// MockBlockchain implements BlockchainInterface for testing
type MockBlockchain struct {
	blocks []*Block
}

func NewMockBlockchain() *MockBlockchain {
	return &MockBlockchain{
		blocks: make([]*Block, 0),
	}
}

func (mb *MockBlockchain) GetLatestBlock() (*Block, error) {
	if len(mb.blocks) == 0 {
		return nil, nil
	}
	return mb.blocks[len(mb.blocks)-1], nil
}

func (mb *MockBlockchain) GetBlockByIndex(index int) (*Block, error) {
	if index < 0 || index >= len(mb.blocks) {
		return nil, nil
	}
	return mb.blocks[index], nil
}

func (mb *MockBlockchain) GetChainLength() (int, error) {
	return len(mb.blocks), nil
}

func (mb *MockBlockchain) ValidateBlock(block *Block) error {
	// Simple validation for testing
	if block.Index < 0 {
		return fmt.Errorf("invalid block index")
	}
	return nil
}

func (mb *MockBlockchain) SaveBlock(block *Block) error {
	// Ensure we have space for this block
	for len(mb.blocks) <= block.Index {
		mb.blocks = append(mb.blocks, nil)
	}
	mb.blocks[block.Index] = block
	return nil
}

func TestSyncManagerCreation(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	if syncManager == nil {
		t.Fatal("SyncManager should not be nil")
	}

	if syncManager.nodeID != "test-node" {
		t.Errorf("Expected nodeID 'test-node', got '%s'", syncManager.nodeID)
	}
}

func TestSyncManagerStartStop(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Test start
	if err := syncManager.Start(); err != nil {
		t.Fatalf("Failed to start sync manager: %v", err)
	}

	// Test that it's running
	if !syncManager.isRunning {
		t.Error("SyncManager should be running after Start()")
	}

	// Test stop
	if err := syncManager.Stop(); err != nil {
		t.Fatalf("Failed to stop sync manager: %v", err)
	}

	// Test that it's stopped
	if syncManager.isRunning {
		t.Error("SyncManager should not be running after Stop()")
	}
}

func TestSyncManagerGetMyChainTip(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Test with empty blockchain
	tip, err := syncManager.getMyChainTip()
	if err != nil {
		t.Fatalf("Failed to get chain tip: %v", err)
	}
	if tip != -1 {
		t.Errorf("Expected tip -1 for empty blockchain, got %d", tip)
	}

	// Add a block and test again
	block := &Block{Index: 0, Hash: "genesis"}
	blockchain.SaveBlock(block)

	tip, err = syncManager.getMyChainTip()
	if err != nil {
		t.Fatalf("Failed to get chain tip: %v", err)
	}
	if tip != 0 {
		t.Errorf("Expected tip 0, got %d", tip)
	}
}

func TestSyncManagerValidateAndIntegrateBlock(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Test with valid block
	block := &Block{
		Index:     0,
		Hash:      "genesis",
		PrevHash:  "0000000000000000000000000000000000000000000000000000000000000000",
		Timestamp: time.Now().Unix(),
	}

	if err := syncManager.validateAndIntegrateBlock(block); err != nil {
		t.Fatalf("Failed to validate and integrate block: %v", err)
	}

	// Verify block was saved
	savedBlock, err := blockchain.GetBlockByIndex(0)
	if err != nil {
		t.Fatalf("Failed to get saved block: %v", err)
	}
	if savedBlock == nil {
		t.Fatal("Block should have been saved")
	}
	if savedBlock.Hash != "genesis" {
		t.Errorf("Expected hash 'genesis', got '%s'", savedBlock.Hash)
	}
}

func TestSyncManagerHandleBlockRequest(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Add a block to the blockchain
	block := &Block{Index: 0, Hash: "genesis"}
	blockchain.SaveBlock(block)

	// Create a block request
	request := &BlockRequest{
		Index:     0,
		NodeID:    "peer-node",
		Timestamp: time.Now().Unix(),
	}

	// Test handling the request
	if err := syncManager.HandleBlockRequest(request); err != nil {
		t.Fatalf("Failed to handle block request: %v", err)
	}
}

func TestSyncManagerGetSyncStatus(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Get status before starting
	status := syncManager.GetSyncStatus()
	if status["is_running"].(bool) {
		t.Error("SyncManager should not be running initially")
	}

	// Start and get status
	syncManager.Start()
	status = syncManager.GetSyncStatus()
	if !status["is_running"].(bool) {
		t.Error("SyncManager should be running after Start()")
	}

	// Stop
	syncManager.Stop()
}

func TestSyncManagerRequestResponseFlow(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Start the sync manager
	if err := syncManager.Start(); err != nil {
		t.Fatalf("Failed to start sync manager: %v", err)
	}
	defer syncManager.Stop()

	// Create a test block
	testBlock := &Block{
		Index:     1,
		Hash:      "test-block-1",
		PrevHash:  "genesis",
		Timestamp: time.Now().Unix(),
	}

	// Start a goroutine to simulate a response
	go func() {
		time.Sleep(100 * time.Millisecond) // Simulate network delay
		response := &BlockResponse{
			Index:     1,
			Block:     testBlock,
			NodeID:    "peer-node",
			Timestamp: time.Now().Unix(),
		}
		syncManager.HandleBlockResponse(response)
	}()

	// Request the block
	block, err := syncManager.requestBlock(1)
	if err != nil {
		t.Fatalf("Failed to request block: %v", err)
	}

	if block == nil {
		t.Fatal("Expected block, got nil")
	}

	if block.Hash != "test-block-1" {
		t.Errorf("Expected hash 'test-block-1', got '%s'", block.Hash)
	}

	if block.Index != 1 {
		t.Errorf("Expected index 1, got %d", block.Index)
	}
}

func TestSyncManagerRequestTimeout(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Start the sync manager
	if err := syncManager.Start(); err != nil {
		t.Fatalf("Failed to start sync manager: %v", err)
	}
	defer syncManager.Stop()

	// Request a block that won't be responded to
	// This should timeout after 30 seconds, but we'll use a shorter timeout for testing
	syncManager.responseTimeout = 100 * time.Millisecond

	_, err := syncManager.requestBlock(999)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestSyncManagerNoPeersAvailable(t *testing.T) {
	blockchain := NewMockBlockchain()
	config := DefaultConsensusConfig()

	syncManager := NewSyncManager(blockchain, "test-node", config)

	// Start the sync manager
	if err := syncManager.Start(); err != nil {
		t.Fatalf("Failed to start sync manager: %v", err)
	}
	defer syncManager.Stop()

	// Test CheckAndSyncIfNeeded when no peers are available
	// This should not fail, just log that no peers are available
	if err := syncManager.CheckAndSyncIfNeeded(); err != nil {
		t.Fatalf("CheckAndSyncIfNeeded should not fail when no peers available: %v", err)
	}

	// Verify the sync manager is not stuck in syncing state
	status := syncManager.GetSyncStatus()
	if status["is_syncing"].(bool) {
		t.Error("SyncManager should not be in syncing state when no peers available")
	}
}
