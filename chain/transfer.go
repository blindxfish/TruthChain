package chain

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/wallet"
)

// Transfer represents a signed character transfer transaction
type Transfer struct {
	From      string `json:"from"`      // Sender address
	To        string `json:"to"`        // Recipient address
	Amount    int    `json:"amount"`    // Number of characters to transfer
	GasFee    int    `json:"gas_fee"`   // Gas fee (always 1 character)
	Timestamp int64  `json:"timestamp"` // Unix timestamp
	Nonce     int64  `json:"nonce"`     // Unique transaction number
	Hash      string `json:"hash"`      // Transaction hash
	Signature string `json:"signature"` // ECDSA signature
}

// TransferPool represents the mempool for pending transfers
type TransferPool struct {
	Transfers []Transfer `json:"transfers"`
	mu        sync.RWMutex
}

// NewTransfer creates a new transfer transaction
func NewTransfer(from, to string, amount int, nonce int64, privateKey *ecdsa.PrivateKey) (*Transfer, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("transfer amount must be positive")
	}

	if !wallet.ValidateAddress(from) {
		return nil, fmt.Errorf("invalid sender address: %s", from)
	}

	if !wallet.ValidateAddress(to) {
		return nil, fmt.Errorf("invalid recipient address: %s", to)
	}

	if from == to {
		return nil, fmt.Errorf("cannot transfer to self")
	}

	// Create transfer
	transfer := &Transfer{
		From:      from,
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

	// Sign the transfer
	signature, err := transfer.Sign(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transfer: %w", err)
	}
	transfer.Signature = signature

	return transfer, nil
}

// CalculateHash calculates the hash of the transfer (excluding signature)
func (t *Transfer) CalculateHash() (string, error) {
	// Create a copy without signature for hashing
	transferForHash := map[string]interface{}{
		"from":      t.From,
		"to":        t.To,
		"amount":    t.Amount,
		"gas_fee":   t.GasFee,
		"timestamp": t.Timestamp,
		"nonce":     t.Nonce,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(transferForHash)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transfer: %w", err)
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
}

// Sign signs the transfer with a private key
func (t *Transfer) Sign(privateKey *ecdsa.PrivateKey) (string, error) {
	hash, err := t.CalculateHash()
	if err != nil {
		return "", err
	}

	// Convert hash to bytes
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return "", fmt.Errorf("failed to decode hash: %w", err)
	}

	// Sign the hash
	signature, err := wallet.SignMessage(hashBytes, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transfer: %w", err)
	}

	return signature, nil
}

// VerifySignature verifies the transfer signature
func (t *Transfer) VerifySignature() (bool, error) {
	// Recalculate hash
	calculatedHash, err := t.CalculateHash()
	if err != nil {
		return false, fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Verify hash matches
	if calculatedHash != t.Hash {
		return false, fmt.Errorf("transfer hash mismatch")
	}

	// Use the same data format as signing
	transferData := fmt.Sprintf("%s:%s:%d:%d:%d:%d", t.From, t.To, t.Amount, t.GasFee, t.Timestamp, t.Nonce)

	// Hash the data (same as wallet.Sign does)
	hash := sha256.Sum256([]byte(transferData))
	hashHex := hex.EncodeToString(hash[:])

	// Use wallet package to recover public key
	recoveredPubKey, err := wallet.RecoverPublicKeyFromSignature(hashHex, t.Signature)
	if err != nil {
		return false, fmt.Errorf("signature recovery failed: %w", err)
	}

	// Derive address from recovered public key
	derivedAddress := wallet.PublicKeyToAddress(recoveredPubKey)

	// Compare with transfer.From
	if derivedAddress != t.From {
		return false, fmt.Errorf("address mismatch: expected %s, got %s", t.From, derivedAddress)
	}

	return true, nil
}

// GetTotalCost returns the total cost including gas fee
func (t *Transfer) GetTotalCost() int {
	return t.Amount + t.GasFee
}

// Validate validates the transfer transaction
func (t *Transfer) Validate() error {
	// Check basic fields
	if t.From == "" {
		return fmt.Errorf("sender address cannot be empty")
	}

	if t.To == "" {
		return fmt.Errorf("recipient address cannot be empty")
	}

	if t.Amount <= 0 {
		return fmt.Errorf("transfer amount must be positive")
	}

	if t.GasFee != 1 {
		return fmt.Errorf("gas fee must be exactly 1 character")
	}

	if t.Timestamp <= 0 {
		return fmt.Errorf("invalid timestamp")
	}

	if t.Nonce < 0 {
		return fmt.Errorf("nonce must be non-negative")
	}

	if t.Hash == "" {
		return fmt.Errorf("transfer hash cannot be empty")
	}

	if t.Signature == "" {
		return fmt.Errorf("transfer signature cannot be empty")
	}

	// Validate addresses
	if !wallet.ValidateAddress(t.From) {
		return fmt.Errorf("invalid sender address: %s", t.From)
	}

	if !wallet.ValidateAddress(t.To) {
		return fmt.Errorf("invalid recipient address: %s", t.To)
	}

	if t.From == t.To {
		return fmt.Errorf("cannot transfer to self")
	}

	// Verify signature
	valid, err := t.VerifySignature()
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	if !valid {
		return fmt.Errorf("invalid transfer signature")
	}

	return nil
}

// NewTransferPool creates a new transfer pool
func NewTransferPool() *TransferPool {
	return &TransferPool{
		Transfers: make([]Transfer, 0),
	}
}

// AddTransfer adds a transfer to the pool
func (tp *TransferPool) AddTransfer(transfer Transfer) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Validate transfer
	if err := transfer.Validate(); err != nil {
		return fmt.Errorf("invalid transfer: %w", err)
	}

	// Check for duplicate hash
	for _, existing := range tp.Transfers {
		if existing.Hash == transfer.Hash {
			return fmt.Errorf("transfer already exists in pool")
		}
	}

	// Add to pool
	tp.Transfers = append(tp.Transfers, transfer)
	return nil
}

// GetTransfers returns all transfers in the pool
func (tp *TransferPool) GetTransfers() []Transfer {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	// Return a copy to avoid race conditions
	transfers := make([]Transfer, len(tp.Transfers))
	copy(transfers, tp.Transfers)
	return transfers
}

// RemoveTransfer removes a transfer from the pool
func (tp *TransferPool) RemoveTransfer(hash string) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	for i, transfer := range tp.Transfers {
		if transfer.Hash == hash {
			tp.Transfers = append(tp.Transfers[:i], tp.Transfers[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("transfer not found in pool: %s", hash)
}

// ClearPool clears all transfers from the pool
func (tp *TransferPool) ClearPool() {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.Transfers = make([]Transfer, 0)
}

// GetTransferCount returns the number of transfers in the pool
func (tp *TransferPool) GetTransferCount() int {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return len(tp.Transfers)
}

// GetTotalCharacterVolume returns the total character volume in the pool
func (tp *TransferPool) GetTotalCharacterVolume() int {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	total := 0
	for _, transfer := range tp.Transfers {
		total += transfer.GetTotalCost()
	}
	return total
}
