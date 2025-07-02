package wallet

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcutil/base58"
	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"golang.org/x/crypto/ripemd160"
)

// Version byte constants for different networks
const (
	TruthChainMainnetVersion  = 0x00
	TruthChainTestnetVersion  = 0x6F
	TruthChainMultisigVersion = 0x05
)

// WalletMetadata contains additional information about the wallet
type WalletMetadata struct {
	Name        string    `json:"name"`
	Created     time.Time `json:"created"`
	LastUsed    time.Time `json:"last_used"`
	Notes       string    `json:"notes"`
	Network     string    `json:"network"` // "mainnet", "testnet", etc.
	VersionByte byte      `json:"version_byte"`
}

// Wallet represents a TruthChain wallet with secp256k1 keypair
type Wallet struct {
	PrivateKey *secp.PrivateKey
	PublicKey  *secp.PublicKey
	Address    string
	Metadata   *WalletMetadata
}

// NewWallet creates a new secp256k1 wallet
func NewWallet() (*Wallet, error) {
	return NewWalletWithMetadata("", TruthChainMainnetVersion)
}

// NewWalletWithMetadata creates a new wallet with custom metadata
func NewWalletWithMetadata(name string, versionByte byte) (*Wallet, error) {
	privateKey, err := secp.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Determine network name based on version byte
	network := "mainnet"
	if versionByte == TruthChainTestnetVersion {
		network = "testnet"
	} else if versionByte == TruthChainMultisigVersion {
		network = "multisig"
	}

	metadata := &WalletMetadata{
		Name:        name,
		Created:     time.Now(),
		LastUsed:    time.Now(),
		Network:     network,
		VersionByte: versionByte,
	}

	wallet := &Wallet{
		PrivateKey: privateKey,
		PublicKey:  privateKey.PubKey(),
		Address:    generateAddressWithVersion(privateKey.PubKey(), versionByte),
		Metadata:   metadata,
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

	// Validate private key length
	if len(block.Bytes) != secp.PrivKeyBytesLen {
		return nil, fmt.Errorf("invalid private key length: expected %d bytes, got %d", secp.PrivKeyBytesLen, len(block.Bytes))
	}

	privateKey := secp.PrivKeyFromBytes(block.Bytes)

	// Try to load metadata from separate file
	metadataPath := walletPath + ".meta"
	metadata := &WalletMetadata{
		Name:        filepath.Base(walletPath),
		Created:     time.Now(),
		LastUsed:    time.Now(),
		Network:     "mainnet",
		VersionByte: TruthChainMainnetVersion,
	}

	if metaData, err := os.ReadFile(metadataPath); err == nil {
		// TODO: Implement JSON unmarshaling for metadata
		// For now, use default metadata
		_ = metaData
	}

	wallet := &Wallet{
		PrivateKey: privateKey,
		PublicKey:  privateKey.PubKey(),
		Address:    generateAddressWithVersion(privateKey.PubKey(), metadata.VersionByte),
		Metadata:   metadata,
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

	// Update last used time
	if w.Metadata != nil {
		w.Metadata.LastUsed = time.Now()
	}

	return nil
}

// GetAddress returns the wallet's public address
func (w *Wallet) GetAddress() string {
	return w.Address
}

// GetNetwork returns the wallet's network (mainnet, testnet, etc.)
func (w *Wallet) GetNetwork() string {
	if w.Metadata != nil {
		return w.Metadata.Network
	}
	return "mainnet"
}

// GetVersionByte returns the wallet's version byte
func (w *Wallet) GetVersionByte() byte {
	if w.Metadata != nil {
		return w.Metadata.VersionByte
	}
	return TruthChainMainnetVersion
}

// ExportPublicKeyHex returns the compressed public key as a hex string
func (w *Wallet) ExportPublicKeyHex() string {
	return hex.EncodeToString(w.PublicKey.SerializeCompressed())
}

// ExportPublicKeyUncompressedHex returns the uncompressed public key as a hex string
func (w *Wallet) ExportPublicKeyUncompressedHex() string {
	return hex.EncodeToString(w.PublicKey.SerializeUncompressed())
}

// ExportPrivateKeyHex returns the private key as a hex string (use with caution!)
func (w *Wallet) ExportPrivateKeyHex() string {
	return hex.EncodeToString(w.PrivateKey.Serialize())
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

// generateAddress creates a Bitcoin-style Base58Check address from the public key
func generateAddress(publicKey *secp.PublicKey) string {
	return generateAddressWithVersion(publicKey, TruthChainMainnetVersion)
}

// generateAddressWithVersion creates a Bitcoin-style Base58Check address with custom version byte
func generateAddressWithVersion(publicKey *secp.PublicKey, versionByte byte) string {
	// Get compressed public key bytes
	pubBytes := publicKey.SerializeCompressed()

	// SHA256 hash
	sha := sha256.Sum256(pubBytes)

	// RIPEMD160 hash
	ripemd := ripemd160.New()
	ripemd.Write(sha[:])
	hashed := ripemd.Sum(nil)

	// Create versioned payload
	versionedPayload := append([]byte{versionByte}, hashed...)

	// Double SHA256 for checksum
	checksum := sha256.Sum256(versionedPayload)
	checksum = sha256.Sum256(checksum[:])

	// Append first 4 bytes of checksum
	finalPayload := append(versionedPayload, checksum[:4]...)

	// Encode as Base58Check
	return base58.Encode(finalPayload)
}

// ValidateAddress checks if a given address is valid
func ValidateAddress(address string) bool {
	return ValidateAddressWithVersion(address, TruthChainMainnetVersion)
}

// ValidateAddressWithVersion checks if a given address is valid for a specific version
func ValidateAddressWithVersion(address string, expectedVersion byte) bool {
	// Decode Base58Check
	decoded := base58.Decode(address)
	if len(decoded) < 5 {
		return false
	}

	// Check version byte
	if decoded[0] != expectedVersion {
		return false
	}

	// Extract payload and checksum
	payload := decoded[:len(decoded)-4]
	checksum := decoded[len(decoded)-4:]

	// Verify checksum
	calculatedChecksum := sha256.Sum256(payload)
	calculatedChecksum = sha256.Sum256(calculatedChecksum[:])

	for i := 0; i < 4; i++ {
		if checksum[i] != calculatedChecksum[i] {
			return false
		}
	}

	return true
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

// NewTestnetWallet creates a new wallet for testnet
func NewTestnetWallet(name string) (*Wallet, error) {
	return NewWalletWithMetadata(name, TruthChainTestnetVersion)
}

// NewMultisigWallet creates a new wallet for multisig (placeholder for future implementation)
func NewMultisigWallet(name string) (*Wallet, error) {
	return NewWalletWithMetadata(name, TruthChainMultisigVersion)
}
