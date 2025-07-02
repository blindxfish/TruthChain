package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/blindxfish/truthchain/chain"
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

		// Blockchain commands
		postContent   = flag.String("post", "", "Post content to the blockchain")
		showPosts     = flag.Bool("posts", false, "Show recent posts")
		showBlocks    = flag.Bool("blocks", false, "Show recent blocks")
		showStatus    = flag.Bool("status", false, "Show blockchain status")
		forceBlock    = flag.Bool("force-block", false, "Force creation of a new block")
		charThreshold = flag.Int("char-threshold", 1000, "Character threshold for block creation")
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

	// Initialize blockchain
	blockchain := chain.NewBlockchain(*charThreshold)

	// Handle wallet-only commands
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

	// Handle blockchain commands
	if *postContent != "" {
		// Create and add post
		post, err := blockchain.CreatePost(*postContent, w)
		if err != nil {
			log.Fatalf("Failed to create post: %v", err)
		}

		err = blockchain.AddPost(*post)
		if err != nil {
			log.Fatalf("Failed to add post to blockchain: %v", err)
		}

		fmt.Printf("Post created successfully!\n")
		fmt.Printf("Post Hash: %s\n", post.Hash)
		fmt.Printf("Author: %s\n", post.Author)
		fmt.Printf("Characters: %d\n", post.GetCharacterCount())
		fmt.Printf("Pending Characters: %d/%d\n", blockchain.GetPendingCharacterCount(), *charThreshold)

		if blockchain.GetPendingCharacterCount() >= *charThreshold {
			fmt.Printf("✅ New block created!\n")
		}
		return
	}

	if *showPosts {
		// Show recent posts from latest block
		latestBlock := blockchain.GetLatestBlock()
		if latestBlock != nil && len(latestBlock.Posts) > 0 {
			fmt.Printf("Recent posts from block %d:\n", latestBlock.Index)
			fmt.Printf("Block Hash: %s\n\n", latestBlock.Hash)

			for i, post := range latestBlock.Posts {
				fmt.Printf("Post %d:\n", i+1)
				fmt.Printf("  Author: %s\n", post.Author)
				fmt.Printf("  Content: %s\n", post.Content)
				fmt.Printf("  Characters: %d\n", post.GetCharacterCount())
				fmt.Printf("  Hash: %s\n", post.Hash)
				fmt.Printf("  Timestamp: %d\n\n", post.Timestamp)
			}
		} else {
			fmt.Println("No posts found in the latest block.")
		}

		// Show pending posts
		pendingCount := blockchain.GetPendingPostCount()
		if pendingCount > 0 {
			fmt.Printf("Pending posts (%d):\n", pendingCount)
			fmt.Printf("Pending characters: %d/%d\n", blockchain.GetPendingCharacterCount(), *charThreshold)
		}
		return
	}

	if *showBlocks {
		// Show recent blocks
		chainLength := blockchain.GetChainLength()
		fmt.Printf("Blockchain length: %d blocks\n\n", chainLength)

		// Show last 5 blocks (or all if less than 5)
		start := 0
		if chainLength > 5 {
			start = chainLength - 5
		}

		for i := start; i < chainLength; i++ {
			block := blockchain.GetBlockByIndex(i)
			if block != nil {
				fmt.Printf("Block %d:\n", block.Index)
				fmt.Printf("  Hash: %s\n", block.Hash)
				fmt.Printf("  Previous Hash: %s\n", block.PrevHash)
				fmt.Printf("  Posts: %d\n", block.GetPostCount())
				fmt.Printf("  Characters: %d\n", block.GetCharacterCount())
				fmt.Printf("  Timestamp: %d\n\n", block.Timestamp)
			}
		}
		return
	}

	if *showStatus {
		// Show blockchain status
		info := blockchain.GetBlockchainInfo()
		fmt.Printf("TruthChain Status:\n")
		fmt.Printf("  Chain Length: %v\n", info["chain_length"])
		fmt.Printf("  Total Posts: %v\n", info["total_post_count"])
		fmt.Printf("  Total Characters: %v\n", info["total_character_count"])
		fmt.Printf("  Pending Posts: %v\n", info["pending_post_count"])
		fmt.Printf("  Pending Characters: %v/%v\n", info["pending_character_count"], info["character_threshold"])

		if latestBlock := blockchain.GetLatestBlock(); latestBlock != nil {
			fmt.Printf("  Latest Block: %d\n", latestBlock.Index)
			fmt.Printf("  Latest Block Hash: %s\n", latestBlock.Hash)
		}

		// Validate chain
		if err := blockchain.ValidateChain(); err != nil {
			fmt.Printf("  Chain Validation: ❌ %v\n", err)
		} else {
			fmt.Printf("  Chain Validation: ✅ Valid\n")
		}
		return
	}

	if *forceBlock {
		// Force creation of a new block
		pendingCount := blockchain.GetPendingPostCount()
		if pendingCount == 0 {
			fmt.Println("No pending posts to create a block from.")
			return
		}

		err := blockchain.ForceCreateBlock()
		if err != nil {
			log.Fatalf("Failed to force create block: %v", err)
		}

		fmt.Printf("✅ Block created successfully!\n")
		fmt.Printf("New block index: %d\n", blockchain.GetChainLength()-1)
		return
	}

	// Normal node startup (no specific command)
	fmt.Printf("TruthChain node starting...\n")
	fmt.Printf("Wallet Address: %s\n", w.GetAddress())
	fmt.Printf("Wallet File: %s\n", *walletPath)
	fmt.Printf("Network: %s\n", w.GetNetwork())
	fmt.Printf("Character Threshold: %d\n", *charThreshold)

	if *debug {
		fmt.Printf("Public Key (compressed): %s\n", w.ExportPublicKeyHex())
		fmt.Printf("Version Byte: 0x%02X\n", w.GetVersionByte())
		fmt.Printf("Address Valid: %t\n", wallet.ValidateAddressWithVersion(w.GetAddress(), w.GetVersionByte()))
	}

	// Show blockchain status
	info := blockchain.GetBlockchainInfo()
	fmt.Printf("Blockchain Status:\n")
	fmt.Printf("  Chain Length: %v\n", info["chain_length"])
	fmt.Printf("  Pending Posts: %v\n", info["pending_post_count"])
	fmt.Printf("  Pending Characters: %v/%v\n", info["pending_character_count"], info["character_threshold"])

	// TODO: Start other node components (API, miner, etc.)
	fmt.Println("\nNode components not yet implemented - stopping.")
	fmt.Println("Use --help to see available commands.")
}
