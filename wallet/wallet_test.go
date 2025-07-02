package wallet

import (
	"testing"
	"time"
)

func TestNewWallet(t *testing.T) {
	wallet, err := NewWallet()
	if err != nil {
		t.Fatalf("Failed to create new wallet: %v", err)
	}

	if wallet.PrivateKey == nil {
		t.Error("Private key is nil")
	}

	if wallet.PublicKey == nil {
		t.Error("Public key is nil")
	}

	if wallet.Address == "" {
		t.Error("Address is empty")
	}

	// Test address validation
	if !ValidateAddressWithVersion(wallet.Address, wallet.GetVersionByte()) {
		t.Errorf("Generated address is invalid: %s", wallet.Address)
	}

	// Test metadata
	if wallet.Metadata == nil {
		t.Error("Metadata is nil")
	} else {
		if wallet.Metadata.Network != "mainnet" {
			t.Errorf("Expected network 'mainnet', got '%s'", wallet.Metadata.Network)
		}
		if wallet.Metadata.VersionByte != TruthChainMainnetVersion {
			t.Errorf("Expected version byte 0x%02X, got 0x%02X", TruthChainMainnetVersion, wallet.Metadata.VersionByte)
		}
	}
}

func TestNewWalletWithMetadata(t *testing.T) {
	wallet, err := NewWalletWithMetadata("Test Wallet", TruthChainTestnetVersion)
	if err != nil {
		t.Fatalf("Failed to create new wallet with metadata: %v", err)
	}

	if wallet.Metadata == nil {
		t.Error("Metadata is nil")
	} else {
		if wallet.Metadata.Name != "Test Wallet" {
			t.Errorf("Expected name 'Test Wallet', got '%s'", wallet.Metadata.Name)
		}
		if wallet.Metadata.Network != "testnet" {
			t.Errorf("Expected network 'testnet', got '%s'", wallet.Metadata.Network)
		}
		if wallet.Metadata.VersionByte != TruthChainTestnetVersion {
			t.Errorf("Expected version byte 0x%02X, got 0x%02X", TruthChainTestnetVersion, wallet.Metadata.VersionByte)
		}
	}

	// Test address validation with correct version
	if !ValidateAddressWithVersion(wallet.Address, TruthChainTestnetVersion) {
		t.Errorf("Generated testnet address is invalid: %s", wallet.Address)
	}
}

func TestAddressGeneration(t *testing.T) {
	wallet1, _ := NewWallet()
	wallet2, _ := NewWallet()

	// Addresses should be different for different wallets
	if wallet1.Address == wallet2.Address {
		t.Error("Different wallets generated the same address")
	}

	// Addresses should be valid
	if !ValidateAddressWithVersion(wallet1.Address, wallet1.GetVersionByte()) {
		t.Errorf("Wallet 1 address is invalid: %s", wallet1.Address)
	}

	if !ValidateAddressWithVersion(wallet2.Address, wallet2.GetVersionByte()) {
		t.Errorf("Wallet 2 address is invalid: %s", wallet2.Address)
	}
}

func TestNetworkSpecificWallets(t *testing.T) {
	// Test mainnet wallet
	mainnetWallet, err := NewWallet()
	if err != nil {
		t.Fatalf("Failed to create mainnet wallet: %v", err)
	}
	if mainnetWallet.GetNetwork() != "mainnet" {
		t.Errorf("Expected mainnet, got %s", mainnetWallet.GetNetwork())
	}

	// Test testnet wallet
	testnetWallet, err := NewTestnetWallet("Testnet Wallet")
	if err != nil {
		t.Fatalf("Failed to create testnet wallet: %v", err)
	}
	if testnetWallet.GetNetwork() != "testnet" {
		t.Errorf("Expected testnet, got %s", testnetWallet.GetNetwork())
	}

	// Test multisig wallet
	multisigWallet, err := NewMultisigWallet("Multisig Wallet")
	if err != nil {
		t.Fatalf("Failed to create multisig wallet: %v", err)
	}
	if multisigWallet.GetNetwork() != "multisig" {
		t.Errorf("Expected multisig, got %s", multisigWallet.GetNetwork())
	}

	// Addresses should be different due to different version bytes
	if mainnetWallet.Address == testnetWallet.Address {
		t.Error("Mainnet and testnet wallets generated the same address")
	}
}

func TestSignAndVerify(t *testing.T) {
	wallet, _ := NewWallet()
	testData := []byte("Hello, TruthChain!")

	// Sign the data
	signature, err := wallet.Sign(testData)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}

	// Verify the signature
	valid, err := wallet.Verify(testData, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}

	if !valid {
		t.Error("Signature verification failed")
	}

	// Test with different data (should fail)
	invalidData := []byte("Different data")
	valid, err = wallet.Verify(invalidData, signature)
	if err == nil && valid {
		t.Error("Signature verification should have failed with different data")
	}
}

func TestExportMethods(t *testing.T) {
	wallet, _ := NewWallet()

	// Test public key export
	compressedHex := wallet.ExportPublicKeyHex()
	if compressedHex == "" {
		t.Error("Compressed public key hex is empty")
	}

	uncompressedHex := wallet.ExportPublicKeyUncompressedHex()
	if uncompressedHex == "" {
		t.Error("Uncompressed public key hex is empty")
	}

	// Compressed should be shorter than uncompressed
	if len(compressedHex) >= len(uncompressedHex) {
		t.Error("Compressed public key should be shorter than uncompressed")
	}

	// Test private key export (for debugging)
	privateHex := wallet.ExportPrivateKeyHex()
	if privateHex == "" {
		t.Error("Private key hex is empty")
	}
}

func TestAddressValidation(t *testing.T) {
	// Test valid mainnet address
	mainnetWallet, _ := NewWallet()
	if !ValidateAddressWithVersion(mainnetWallet.Address, TruthChainMainnetVersion) {
		t.Errorf("Valid mainnet address failed validation: %s", mainnetWallet.Address)
	}

	// Test valid testnet address
	testnetWallet, _ := NewTestnetWallet("Test")
	if !ValidateAddressWithVersion(testnetWallet.Address, TruthChainTestnetVersion) {
		t.Errorf("Valid testnet address failed validation: %s", testnetWallet.Address)
	}

	// Test cross-validation (should fail)
	if ValidateAddressWithVersion(mainnetWallet.Address, TruthChainTestnetVersion) {
		t.Error("Mainnet address should not validate as testnet")
	}

	if ValidateAddressWithVersion(testnetWallet.Address, TruthChainMainnetVersion) {
		t.Error("Testnet address should not validate as mainnet")
	}

	// Test invalid addresses
	invalidAddresses := []string{
		"",
		"invalid",
		"1234567890",
		"TCinvalid",
		"1invalidaddress", // Invalid Base58Check
	}

	for _, addr := range invalidAddresses {
		if ValidateAddress(addr) {
			t.Errorf("Invalid address passed validation: %s", addr)
		}
	}
}

func TestVersionConstants(t *testing.T) {
	// Test that version constants are different
	if TruthChainMainnetVersion == TruthChainTestnetVersion {
		t.Error("Mainnet and testnet version bytes should be different")
	}
	if TruthChainMainnetVersion == TruthChainMultisigVersion {
		t.Error("Mainnet and multisig version bytes should be different")
	}
	if TruthChainTestnetVersion == TruthChainMultisigVersion {
		t.Error("Testnet and multisig version bytes should be different")
	}
}

func TestMetadataFields(t *testing.T) {
	wallet, _ := NewWalletWithMetadata("Test Wallet", TruthChainMainnetVersion)

	if wallet.Metadata == nil {
		t.Fatal("Metadata is nil")
	}

	// Test metadata fields
	if wallet.Metadata.Name != "Test Wallet" {
		t.Errorf("Expected name 'Test Wallet', got '%s'", wallet.Metadata.Name)
	}

	if wallet.Metadata.Network != "mainnet" {
		t.Errorf("Expected network 'mainnet', got '%s'", wallet.Metadata.Network)
	}

	if wallet.Metadata.VersionByte != TruthChainMainnetVersion {
		t.Errorf("Expected version byte 0x%02X, got 0x%02X", TruthChainMainnetVersion, wallet.Metadata.VersionByte)
	}

	// Test that timestamps are recent
	now := time.Now()
	if wallet.Metadata.Created.After(now) {
		t.Error("Created timestamp is in the future")
	}
	if wallet.Metadata.LastUsed.After(now) {
		t.Error("LastUsed timestamp is in the future")
	}
}
