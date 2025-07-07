package chain

import (
	"fmt"
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
