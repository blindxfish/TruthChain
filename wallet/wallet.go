package wallet

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"golang.org/x/crypto/ripemd160"
)

// Wallet represents a TruthChain wallet with secp256k1 keypair
type Wallet struct {
	PrivateKey *secp.PrivateKey
	PublicKey  *secp.PublicKey
	Address    string
}

// NewWallet creates a new secp256k1 wallet
func NewWallet() (*Wallet, error) {
	privateKey, err := secp.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	wallet := &Wallet{
		PrivateKey: privateKey,
		PublicKey:  privateKey.PubKey(),
		Address:    generateAddress(privateKey.PubKey()),
	}

	return wallet, nil
}

// LoadWallet loads an existing wallet from file
func LoadWallet(walletPath string) (*Wallet, error) {
	data, err := os.ReadFile(walletPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey := secp.PrivKeyFromBytes(block.Bytes)

	wallet := &Wallet{
		PrivateKey: privateKey,
		PublicKey:  privateKey.PubKey(),
		Address:    generateAddress(privateKey.PubKey()),
	}

	return wallet, nil
}

// SaveWallet saves the wallet to a file
func (w *Wallet) SaveWallet(walletPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(walletPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create wallet directory: %w", err)
	}

	// Get private key bytes
	keyBytes := w.PrivateKey.Serialize()

	// Create PEM block
	block := &pem.Block{
		Type:  "SECP256K1 PRIVATE KEY",
		Bytes: keyBytes,
	}

	// Write to file
	if err := os.WriteFile(walletPath, pem.EncodeToMemory(block), 0600); err != nil {
		return fmt.Errorf("failed to write wallet file: %w", err)
	}

	return nil
}

// GetAddress returns the wallet's public address
func (w *Wallet) GetAddress() string {
	return w.Address
}

// Sign signs data with the wallet's private key
func (w *Wallet) Sign(data []byte) ([]byte, error) {
	// Hash the data first (best practice for ECDSA)
	hash := sha256.Sum256(data)

	// Sign the hash using ECDSA
	signature, err := ecdsa.SignASN1(rand.Reader, w.PrivateKey.ToECDSA(), hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

// Verify verifies a signature against data and public key
func (w *Wallet) Verify(data []byte, signature []byte) (bool, error) {
	// Hash the data first
	hash := sha256.Sum256(data)

	// Verify the signature using ECDSA
	return ecdsa.VerifyASN1(w.PublicKey.ToECDSA(), hash[:], signature), nil
}

// VerifySignature verifies a signature against data and a given public key
func VerifySignature(data []byte, signature []byte, publicKey *secp.PublicKey) (bool, error) {
	// Hash the data first
	hash := sha256.Sum256(data)

	// Verify the signature using ECDSA
	return ecdsa.VerifyASN1(publicKey.ToECDSA(), hash[:], signature), nil
}

// generateAddress creates a Bitcoin-style address from the public key
func generateAddress(publicKey *secp.PublicKey) string {
	// Get compressed public key bytes
	pubBytes := publicKey.SerializeCompressed()

	// SHA256 hash
	sha := sha256.Sum256(pubBytes)

	// RIPEMD160 hash
	ripemd := ripemd160.New()
	ripemd.Write(sha[:])
	hashed := ripemd.Sum(nil)

	// Add TruthChain prefix and format as hex
	return fmt.Sprintf("TC%x", hashed)
}

// LoadOrCreateWallet loads an existing wallet or creates a new one
func LoadOrCreateWallet(walletPath string) (*Wallet, error) {
	// Try to load existing wallet
	if _, err := os.Stat(walletPath); err == nil {
		return LoadWallet(walletPath)
	}

	// Create new wallet if file doesn't exist
	wallet, err := NewWallet()
	if err != nil {
		return nil, err
	}

	// Save the new wallet
	if err := wallet.SaveWallet(walletPath); err != nil {
		return nil, err
	}

	return wallet, nil
}
