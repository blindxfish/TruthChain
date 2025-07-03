package chain

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// StateManager handles global state management and StateRoot calculation
type StateManager struct {
	mu sync.RWMutex
	// Current state
	wallets map[string]*WalletState
	// Nonce tracking per address
	nonces map[string]int64
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		wallets: make(map[string]*WalletState),
		nonces:  make(map[string]int64),
	}
}

// GetWalletState returns the current state of a wallet
func (sm *StateManager) GetWalletState(address string) (*WalletState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	wallet, exists := sm.wallets[address]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	walletCopy := *wallet
	return &walletCopy, true
}

// UpdateWalletState updates the state of a wallet
func (sm *StateManager) UpdateWalletState(address string, balance int, nonce int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now().Unix()

	wallet, exists := sm.wallets[address]
	if !exists {
		wallet = &WalletState{
			Address:    address,
			Balance:    balance,
			Nonce:      nonce,
			LastTxTime: now,
		}
		sm.wallets[address] = wallet
	} else {
		wallet.Balance = balance
		wallet.Nonce = nonce
		wallet.LastTxTime = now
	}

	// Update nonce tracking
	sm.nonces[address] = nonce
}

// GetNextNonce returns the next nonce for an address
func (sm *StateManager) GetNextNonce(address string) int64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	currentNonce := sm.nonces[address]
	nextNonce := currentNonce + 1
	sm.nonces[address] = nextNonce
	return nextNonce
}

// GetEffectiveBalance returns the effective balance considering pending transactions
func (sm *StateManager) GetEffectiveBalance(address string, pendingTransfers []Transfer) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	wallet, exists := sm.wallets[address]
	if !exists {
		return 0
	}

	// Calculate pending deductions
	pendingDeductions := 0
	for _, transfer := range pendingTransfers {
		if transfer.From == address {
			pendingDeductions += transfer.GetTotalCost()
		}
	}

	effectiveBalance := wallet.Balance - pendingDeductions
	if effectiveBalance < 0 {
		effectiveBalance = 0
	}

	return effectiveBalance
}

// ValidateTransfer validates a transfer against current state
func (sm *StateManager) ValidateTransfer(transfer Transfer, pendingTransfers []Transfer) error {
	// Get current wallet state
	wallet, exists := sm.GetWalletState(transfer.From)
	if !exists {
		return fmt.Errorf("wallet not found: %s", transfer.From)
	}

	// Check nonce
	if transfer.Nonce <= wallet.Nonce {
		return fmt.Errorf("invalid nonce: expected > %d, got %d", wallet.Nonce, transfer.Nonce)
	}

	// Check effective balance
	effectiveBalance := sm.GetEffectiveBalance(transfer.From, pendingTransfers)
	if effectiveBalance < transfer.GetTotalCost() {
		return fmt.Errorf("insufficient effective balance: %d, need %d", effectiveBalance, transfer.GetTotalCost())
	}

	return nil
}

// ApplyTransfer applies a transfer to the current state
func (sm *StateManager) ApplyTransfer(transfer Transfer) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Get sender wallet
	senderWallet, exists := sm.wallets[transfer.From]
	if !exists {
		senderWallet = &WalletState{
			Address:    transfer.From,
			Balance:    0,
			Nonce:      0,
			LastTxTime: time.Now().Unix(),
		}
		sm.wallets[transfer.From] = senderWallet
	}

	// Get recipient wallet
	recipientWallet, exists := sm.wallets[transfer.To]
	if !exists {
		recipientWallet = &WalletState{
			Address:    transfer.To,
			Balance:    0,
			Nonce:      0,
			LastTxTime: time.Now().Unix(),
		}
		sm.wallets[transfer.To] = recipientWallet
	}

	// Validate sender balance
	if senderWallet.Balance < transfer.GetTotalCost() {
		return fmt.Errorf("insufficient balance: %d, need %d", senderWallet.Balance, transfer.GetTotalCost())
	}

	// Validate nonce
	if transfer.Nonce <= senderWallet.Nonce {
		return fmt.Errorf("invalid nonce: expected > %d, got %d", senderWallet.Nonce, transfer.Nonce)
	}

	// Apply transfer
	senderWallet.Balance -= transfer.GetTotalCost()
	senderWallet.Nonce = transfer.Nonce
	senderWallet.LastTxTime = time.Now().Unix()

	recipientWallet.Balance += transfer.Amount
	recipientWallet.LastTxTime = time.Now().Unix()

	// Update nonce tracking
	sm.nonces[transfer.From] = transfer.Nonce

	return nil
}

// CalculateStateRoot calculates the StateRoot for the current state
func (sm *StateManager) CalculateStateRoot(blockIndex int) *StateRoot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Convert map to slice
	wallets := make([]WalletState, 0, len(sm.wallets))
	for _, wallet := range sm.wallets {
		wallets = append(wallets, *wallet)
	}

	// Sort wallets by address for deterministic ordering
	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].Address < wallets[j].Address
	})

	stateRoot := &StateRoot{
		Wallets:    wallets,
		BlockIndex: blockIndex,
	}

	stateRoot.SetHash()
	return stateRoot
}

// LoadStateFromStateRoot loads the state from a StateRoot
func (sm *StateManager) LoadStateFromStateRoot(stateRoot *StateRoot) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear current state
	sm.wallets = make(map[string]*WalletState)
	sm.nonces = make(map[string]int64)

	// Load wallets from state root
	for _, wallet := range stateRoot.Wallets {
		walletCopy := wallet
		sm.wallets[wallet.Address] = &walletCopy
		sm.nonces[wallet.Address] = wallet.Nonce
	}

	return nil
}

// GetAllWallets returns all wallet states
func (sm *StateManager) GetAllWallets() []WalletState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	wallets := make([]WalletState, 0, len(sm.wallets))
	for _, wallet := range sm.wallets {
		wallets = append(wallets, *wallet)
	}

	// Sort by address for consistent ordering
	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].Address < wallets[j].Address
	})

	return wallets
}

// GetWalletCount returns the number of wallets in the state
func (sm *StateManager) GetWalletCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.wallets)
}

// GetTotalCharacterSupply returns the total character supply
func (sm *StateManager) GetTotalCharacterSupply() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	total := 0
	for _, wallet := range sm.wallets {
		total += wallet.Balance
	}
	return total
}
