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
		return
	}

	// Normal node startup
	fmt.Printf("TruthChain node starting...\n")
	fmt.Printf("Wallet Address: %s\n", w.GetAddress())
	fmt.Printf("Wallet File: %s\n", *walletPath)

	// TODO: Start other node components (API, chain, miner, etc.)
	fmt.Println("Node components not yet implemented - stopping.")
}
