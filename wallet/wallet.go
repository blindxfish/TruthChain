package wallet

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcutil/base58"
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
	PrivateKey *btcec.PrivateKey
	PublicKey  *btcec.PublicKey
	Address    string
	Metadata   *WalletMetadata
}

// WalletBackup represents a complete wallet backup
type WalletBackup struct {
	Version    string          `json:"version"`
	Created    time.Time       `json:"created"`
	Metadata   *WalletMetadata `json:"metadata"`
	PrivateKey string          `json:"private_key"` // Hex encoded
	PublicKey  string          `json:"public_key"`  // Hex encoded compressed
	Address    string          `json:"address"`
	BackupHash string          `json:"backup_hash"` // SHA256 of backup for verification
}

// NewWallet creates a new secp256k1 wallet
func NewWallet() (*Wallet, error) {
	return NewWalletWithMetadata("", TruthChainMainnetVersion)
}

// NewWalletWithMetadata creates a new wallet with custom metadata
func NewWalletWithMetadata(name string, versionByte byte) (*Wallet, error) {
	privateKey, err := btcec.NewPrivateKey()
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
	if len(block.Bytes) != btcec.PrivKeyBytesLen {
		return nil, fmt.Errorf("invalid private key length: expected %d bytes, got %d", btcec.PrivKeyBytesLen, len(block.Bytes))
	}

	privateKey, _ := btcec.PrivKeyFromBytes(block.Bytes)

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

// Sign signs data with the wallet's private key using compact signature format
func (w *Wallet) Sign(data []byte) ([]byte, error) {
	// Hash the data first (best practice for ECDSA)
	hash := sha256.Sum256(data)

	// Sign using compact signature format for public key recovery
	signature := btcecdsa.SignCompact(w.PrivateKey, hash[:], true)
	return signature, nil
}

// Verify verifies a signature against data and public key
func (w *Wallet) Verify(data []byte, signature []byte) (bool, error) {
	// Hash the data first
	hash := sha256.Sum256(data)

	// Recover public key from compact signature
	recoveredPubKey, _, err := btcecdsa.RecoverCompact(signature, hash[:])
	if err != nil {
		return false, fmt.Errorf("failed to recover public key: %w", err)
	}

	// Compare with our public key
	if !recoveredPubKey.IsEqual(w.PublicKey) {
		return false, fmt.Errorf("recovered public key does not match")
	}

	return true, nil
}

// VerifySignature verifies a signature against data and a given public key
func VerifySignature(data []byte, signature []byte, publicKey *btcec.PublicKey) (bool, error) {
	// Hash the data first
	hash := sha256.Sum256(data)

	// Verify the signature using ECDSA
	return ecdsa.VerifyASN1(publicKey.ToECDSA(), hash[:], signature), nil
}

// generateAddress creates a Bitcoin-style Base58Check address from the public key
func generateAddress(publicKey *btcec.PublicKey) string {
	return generateAddressWithVersion(publicKey, TruthChainMainnetVersion)
}

// generateAddressWithVersion creates a Bitcoin-style Base58Check address with custom version byte
func generateAddressWithVersion(publicKey *btcec.PublicKey, versionByte byte) string {
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

// DeriveAddress derives a TruthChain address from a btcec public key
func DeriveAddress(pub *btcec.PublicKey) string {
	pubBytes := pub.SerializeCompressed()
	sha := sha256.Sum256(pubBytes)
	ripemd := ripemd160.New()
	ripemd.Write(sha[:])
	hashed := ripemd.Sum(nil)

	payload := append([]byte{0x00}, hashed...) // Version byte for mainnet
	checksum := sha256.Sum256(payload)
	checksum = sha256.Sum256(checksum[:])
	full := append(payload, checksum[:4]...)
	return base58.Encode(full)
}

// NewMultisigWallet creates a new wallet for multisig (placeholder for future implementation)
func NewMultisigWallet(name string) (*Wallet, error) {
	return NewWalletWithMetadata(name, TruthChainMultisigVersion)
}

// SignMessage signs a message with a private key and returns hex-encoded signature
func SignMessage(message []byte, privateKey *ecdsa.PrivateKey) (string, error) {
	// Create hash of the message
	hash := sha256.Sum256(message)

	// Sign the hash
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign message: %w", err)
	}

	// Return hex-encoded signature
	return hex.EncodeToString(signature), nil
}

// RecoverPublicKeyFromSignature recovers the public key from a compact signature and message hash (hex)
func RecoverPublicKeyFromSignature(messageHash string, signatureHex string) (*btcec.PublicKey, error) {
	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	hashBytes, err := hex.DecodeString(messageHash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash encoding: %w", err)
	}

	// Use btcecdsa.RecoverCompact to recover the public key
	recoveredPubKey, _, err := btcecdsa.RecoverCompact(sigBytes, hashBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to recover public key: %w", err)
	}

	return recoveredPubKey, nil
}

// PublicKeyToAddress converts a public key to a TruthChain address
func PublicKeyToAddress(publicKey *btcec.PublicKey) string {
	// Use the same logic as the wallet's address generation
	return generateAddressWithVersion(publicKey, TruthChainMainnetVersion)
}

// ExportBackup creates a complete wallet backup
func (w *Wallet) ExportBackup() (*WalletBackup, error) {
	// Update last used time
	if w.Metadata != nil {
		w.Metadata.LastUsed = time.Now()
	}

	backup := &WalletBackup{
		Version:    "1.0",
		Created:    time.Now(),
		Metadata:   w.Metadata,
		PrivateKey: hex.EncodeToString(w.PrivateKey.Serialize()),
		PublicKey:  hex.EncodeToString(w.PublicKey.SerializeCompressed()),
		Address:    w.Address,
		BackupHash: "", // Exclude from hash
	}

	// Calculate backup hash for verification (excluding BackupHash field)
	hash, err := calculateBackupHash(backup)
	if err != nil {
		return nil, err
	}
	backup.BackupHash = hash

	return backup, nil
}

// calculateBackupHash marshals the backup with BackupHash set to "" and returns the SHA256 hex
func calculateBackupHash(backup *WalletBackup) (string, error) {
	copy := *backup
	copy.BackupHash = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// SaveBackup saves a wallet backup to a file
func (w *Wallet) SaveBackup(backupPath string) error {
	backup, err := w.ExportBackup()
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(backupPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Marshal backup to JSON
	backupData, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal backup: %w", err)
	}

	// Write backup file
	if err := os.WriteFile(backupPath, backupData, 0600); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// ImportBackup restores a wallet from a backup
func ImportBackup(backupPath string) (*Wallet, error) {
	// Read backup file
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup file: %w", err)
	}

	// Unmarshal backup
	var backup WalletBackup
	if err := json.Unmarshal(backupData, &backup); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup: %w", err)
	}

	// Verify backup hash (exclude BackupHash field)
	calculatedHash, err := calculateBackupHash(&backup)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate backup hash: %w", err)
	}
	if calculatedHash != backup.BackupHash {
		return nil, fmt.Errorf("backup hash verification failed")
	}

	// Decode private key
	privateKeyBytes, err := hex.DecodeString(backup.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	// Validate private key length
	if len(privateKeyBytes) != btcec.PrivKeyBytesLen {
		return nil, fmt.Errorf("invalid private key length")
	}

	// Create private key
	privateKey, _ := btcec.PrivKeyFromBytes(privateKeyBytes)

	// Verify public key matches
	decodedPubKey, err := hex.DecodeString(backup.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	publicKey, err := btcec.ParsePubKey(decodedPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Verify private key matches public key
	if !privateKey.PubKey().IsEqual(publicKey) {
		return nil, fmt.Errorf("private key does not match public key")
	}

	// Verify address matches
	expectedAddress := generateAddressWithVersion(publicKey, backup.Metadata.VersionByte)
	if expectedAddress != backup.Address {
		return nil, fmt.Errorf("address verification failed")
	}

	// Create wallet
	wallet := &Wallet{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    backup.Address,
		Metadata:   backup.Metadata,
	}

	// Update last used time
	if wallet.Metadata != nil {
		wallet.Metadata.LastUsed = time.Now()
	}

	return wallet, nil
}

// ValidateBackup validates a backup file without importing it
func ValidateBackup(backupPath string) error {
	// Read backup file
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Unmarshal backup
	var backup WalletBackup
	if err := json.Unmarshal(backupData, &backup); err != nil {
		return fmt.Errorf("failed to unmarshal backup: %w", err)
	}

	// Verify backup hash (exclude BackupHash field)
	calculatedHash, err := calculateBackupHash(&backup)
	if err != nil {
		return fmt.Errorf("failed to calculate backup hash: %w", err)
	}
	if calculatedHash != backup.BackupHash {
		return fmt.Errorf("backup hash verification failed")
	}

	// Verify private key format
	privateKeyBytes, err := hex.DecodeString(backup.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}

	if len(privateKeyBytes) != btcec.PrivKeyBytesLen {
		return fmt.Errorf("invalid private key length")
	}

	// Verify public key format
	_, err = hex.DecodeString(backup.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}

	// Verify address format
	if !ValidateAddressWithVersion(backup.Address, backup.Metadata.VersionByte) {
		return fmt.Errorf("invalid address format")
	}

	return nil
}
