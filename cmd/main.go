package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/blindxfish/truthchain/wallet"
)

func main() {
	// Define command line flags
	var (
		walletPath = flag.String("wallet", "wallet.key", "Path to wallet file")
		showWallet = flag.Bool("show-wallet", false, "Show wallet address and exit")
		debug      = flag.Bool("debug", false, "Show additional wallet information")
	)
	flag.Parse()

	// Load or create wallet
	w, err := wallet.LoadOrCreateWallet(*walletPath)
	if err != nil {
		log.Fatalf("Failed to load/create wallet: %v", err)
	}

	// If show-wallet flag is set, display address and exit
	if *showWallet {
		fmt.Printf("Wallet Address: %s\n", w.GetAddress())
		fmt.Printf("Wallet File: %s\n", *walletPath)

		if *debug {
			fmt.Printf("Public Key (compressed): %s\n", w.ExportPublicKeyHex())
			fmt.Printf("Public Key (uncompressed): %s\n", w.ExportPublicKeyUncompressedHex())
			fmt.Printf("Address Valid: %t\n", wallet.ValidateAddress(w.GetAddress()))
		}
		return
	}

	// Normal node startup
	fmt.Printf("TruthChain node starting...\n")
	fmt.Printf("Wallet Address: %s\n", w.GetAddress())
	fmt.Printf("Wallet File: %s\n", *walletPath)

	if *debug {
		fmt.Printf("Public Key (compressed): %s\n", w.ExportPublicKeyHex())
		fmt.Printf("Address Valid: %t\n", wallet.ValidateAddress(w.GetAddress()))
	}

	// TODO: Start other node components (API, chain, miner, etc.)
	fmt.Println("Node components not yet implemented - stopping.")
}
