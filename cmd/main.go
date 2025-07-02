package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/blindxfish/truthchain/wallet"
)

func main() {
	// Define command line flags
	var (
		walletPath = flag.String("wallet", "wallet.key", "Path to wallet file")
		showWallet = flag.Bool("show-wallet", false, "Show wallet address and exit")
		debug      = flag.Bool("debug", false, "Show additional wallet information")
		network    = flag.String("network", "mainnet", "Network type: mainnet, testnet, multisig")
		walletName = flag.String("name", "", "Wallet name for new wallets")
	)
	flag.Parse()

	// Load or create wallet
	var w *wallet.Wallet
	var err error

	// Try to load existing wallet first
	if _, statErr := os.Stat(*walletPath); statErr == nil {
		w, err = wallet.LoadWallet(*walletPath)
		if err != nil {
			log.Fatalf("Failed to load wallet: %v", err)
		}
	} else {
		// Create new wallet based on network type
		switch *network {
		case "mainnet":
			w, err = wallet.NewWalletWithMetadata(*walletName, wallet.TruthChainMainnetVersion)
		case "testnet":
			w, err = wallet.NewTestnetWallet(*walletName)
		case "multisig":
			w, err = wallet.NewMultisigWallet(*walletName)
		default:
			log.Fatalf("Invalid network type: %s. Use mainnet, testnet, or multisig", *network)
		}

		if err != nil {
			log.Fatalf("Failed to create wallet: %v", err)
		}

		// Save the new wallet
		if err := w.SaveWallet(*walletPath); err != nil {
			log.Fatalf("Failed to save wallet: %v", err)
		}
	}

	// If show-wallet flag is set, display address and exit
	if *showWallet {
		fmt.Printf("Wallet Address: %s\n", w.GetAddress())
		fmt.Printf("Wallet File: %s\n", *walletPath)
		fmt.Printf("Network: %s\n", w.GetNetwork())

		if *debug {
			fmt.Printf("Public Key (compressed): %s\n", w.ExportPublicKeyHex())
			fmt.Printf("Public Key (uncompressed): %s\n", w.ExportPublicKeyUncompressedHex())
			fmt.Printf("Version Byte: 0x%02X\n", w.GetVersionByte())
			fmt.Printf("Address Valid: %t\n", wallet.ValidateAddressWithVersion(w.GetAddress(), w.GetVersionByte()))

			if w.Metadata != nil {
				fmt.Printf("Wallet Name: %s\n", w.Metadata.Name)
				fmt.Printf("Created: %s\n", w.Metadata.Created.Format("2006-01-02 15:04:05"))
				fmt.Printf("Last Used: %s\n", w.Metadata.LastUsed.Format("2006-01-02 15:04:05"))
				if w.Metadata.Notes != "" {
					fmt.Printf("Notes: %s\n", w.Metadata.Notes)
				}
			}
		}
		return
	}

	// Normal node startup
	fmt.Printf("TruthChain node starting...\n")
	fmt.Printf("Wallet Address: %s\n", w.GetAddress())
	fmt.Printf("Wallet File: %s\n", *walletPath)
	fmt.Printf("Network: %s\n", w.GetNetwork())

	if *debug {
		fmt.Printf("Public Key (compressed): %s\n", w.ExportPublicKeyHex())
		fmt.Printf("Version Byte: 0x%02X\n", w.GetVersionByte())
		fmt.Printf("Address Valid: %t\n", wallet.ValidateAddressWithVersion(w.GetAddress(), w.GetVersionByte()))
	}

	// TODO: Start other node components (API, chain, miner, etc.)
	fmt.Println("Node components not yet implemented - stopping.")
}
