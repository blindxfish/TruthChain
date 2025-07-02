package wallet

import (
	"testing"
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
	if !ValidateAddress(wallet.Address) {
		t.Errorf("Generated address is invalid: %s", wallet.Address)
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
	if !ValidateAddress(wallet1.Address) {
		t.Errorf("Wallet 1 address is invalid: %s", wallet1.Address)
	}

	if !ValidateAddress(wallet2.Address) {
		t.Errorf("Wallet 2 address is invalid: %s", wallet2.Address)
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
	if err != nil {
		t.Fatalf("Failed to verify signature with invalid data: %v", err)
	}

	if valid {
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
	// Test valid address
	wallet, _ := NewWallet()
	if !ValidateAddress(wallet.Address) {
		t.Errorf("Valid address failed validation: %s", wallet.Address)
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
