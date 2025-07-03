package blockchain

import (
	"testing"

	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

func TestCreateTransfer(t *testing.T) {
	// Create test storage
	storage, err := store.NewBoltDBStorage("test_transfer.db")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain
	bc, err := NewBlockchain(storage, 5)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create test wallet
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create test wallet: %v", err)
	}

	// Give wallet some characters
	storage.UpdateCharacterBalance(w.GetAddress(), 1000)

	// Test valid transfer
	transfer, err := bc.CreateTransfer("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 100, w)
	if err != nil {
		t.Fatalf("Failed to create transfer: %v", err)
	}

	// Check transfer fields
	if transfer.From != w.GetAddress() {
		t.Errorf("Expected from address %s, got %s", w.GetAddress(), transfer.From)
	}

	if transfer.To != "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" {
		t.Errorf("Expected to address 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa, got %s", transfer.To)
	}

	if transfer.Amount != 100 {
		t.Errorf("Expected amount 100, got %d", transfer.Amount)
	}

	if transfer.GasFee != 1 {
		t.Errorf("Expected gas fee 1, got %d", transfer.GasFee)
	}

	if transfer.Hash == "" {
		t.Error("Transfer hash should not be empty")
	}

	if transfer.Signature == "" {
		t.Error("Transfer signature should not be empty")
	}
}

func TestAddTransfer(t *testing.T) {
	// Create test storage
	storage, err := store.NewBoltDBStorage("test_add_transfer.db")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain
	bc, err := NewBlockchain(storage, 5)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create test wallet
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create test wallet: %v", err)
	}

	// Give wallet some characters in storage
	storage.UpdateCharacterBalance(w.GetAddress(), 1000)

	// Add wallet to state manager
	bc.stateManager.UpdateWalletState(w.GetAddress(), 1000, 0)

	// Create transfer
	transfer, err := bc.CreateTransfer("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 100, w)
	if err != nil {
		t.Fatalf("Failed to create transfer: %v", err)
	}

	// Add transfer to pool
	if err := bc.AddTransfer(*transfer); err != nil {
		t.Fatalf("Failed to add transfer: %v", err)
	}

	// Check transfer pool
	poolInfo := bc.GetTransferPoolInfo()
	if poolInfo["transfer_count"].(int) != 1 {
		t.Errorf("Expected 1 transfer in pool, got %d", poolInfo["transfer_count"])
	}

	if poolInfo["total_character_volume"].(int) != 101 { // 100 + 1 gas fee
		t.Errorf("Expected 101 character volume, got %d", poolInfo["total_character_volume"])
	}
}

func TestAddTransferInsufficientBalance(t *testing.T) {
	// Create test storage
	storage, err := store.NewBoltDBStorage("test_insufficient_balance.db")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain
	bc, err := NewBlockchain(storage, 5)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create test wallet
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create test wallet: %v", err)
	}

	// Give wallet only 50 characters (need 101 for 100 + 1 gas fee)
	storage.UpdateCharacterBalance(w.GetAddress(), 50)

	// Add wallet to state manager with insufficient balance
	bc.stateManager.UpdateWalletState(w.GetAddress(), 50, 0)

	// Create transfer
	transfer, err := bc.CreateTransfer("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 100, w)
	if err != nil {
		t.Fatalf("Failed to create transfer: %v", err)
	}

	// Add transfer should fail due to insufficient balance
	if err := bc.AddTransfer(*transfer); err == nil {
		t.Error("Expected error for insufficient balance")
	}
}

func TestProcessTransfers(t *testing.T) {
	// Create test storage
	storage, err := store.NewBoltDBStorage("test_process_transfers.db")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain
	bc, err := NewBlockchain(storage, 5)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Create test wallet
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create test wallet: %v", err)
	}

	// Give wallet some characters in storage
	storage.UpdateCharacterBalance(w.GetAddress(), 1000)

	// Add wallet to state manager
	bc.stateManager.UpdateWalletState(w.GetAddress(), 1000, 0)

	// Create and add transfer
	transfer, err := bc.CreateTransfer("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 100, w)
	if err != nil {
		t.Fatalf("Failed to create transfer: %v", err)
	}

	if err := bc.AddTransfer(*transfer); err != nil {
		t.Fatalf("Failed to add transfer: %v", err)
	}

	// Check initial balances
	senderBalance, _ := storage.GetCharacterBalance(w.GetAddress())
	recipientBalance, _ := storage.GetCharacterBalance("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")

	// Process transfers
	if err := bc.ProcessTransfers(); err != nil {
		t.Fatalf("Failed to process transfers: %v", err)
	}

	// Check final balances
	newSenderBalance, _ := storage.GetCharacterBalance(w.GetAddress())
	newRecipientBalance, _ := storage.GetCharacterBalance("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")

	// Sender should have 101 less (100 + 1 gas fee)
	if newSenderBalance != senderBalance-101 {
		t.Errorf("Expected sender balance %d, got %d", senderBalance-101, newSenderBalance)
	}

	// Recipient should have 100 more
	if newRecipientBalance != recipientBalance+100 {
		t.Errorf("Expected recipient balance %d, got %d", recipientBalance+100, newRecipientBalance)
	}

	// Transfer pool should be empty
	poolInfo := bc.GetTransferPoolInfo()
	if poolInfo["transfer_count"].(int) != 0 {
		t.Errorf("Expected 0 transfers in pool after processing, got %d", poolInfo["transfer_count"])
	}
}

func TestGetTransferPoolInfo(t *testing.T) {
	// Create test storage
	storage, err := store.NewBoltDBStorage("test_pool_info.db")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create blockchain
	bc, err := NewBlockchain(storage, 5)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	// Get empty pool info
	poolInfo := bc.GetTransferPoolInfo()

	// Check required fields
	requiredFields := []string{"transfer_count", "total_character_volume", "transfers"}
	for _, field := range requiredFields {
		if _, exists := poolInfo[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Check initial values
	if poolInfo["transfer_count"].(int) != 0 {
		t.Errorf("Expected 0 transfers, got %d", poolInfo["transfer_count"])
	}

	if poolInfo["total_character_volume"].(int) != 0 {
		t.Errorf("Expected 0 character volume, got %d", poolInfo["total_character_volume"])
	}
}
