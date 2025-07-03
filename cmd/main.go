package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/miner"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

func clearScreen() {
	cmd := exec.Command("clear")
	if os.Getenv("OS") == "Windows_NT" {
		cmd = exec.Command("cmd", "/c", "cls")
	}
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

func main() {
	// Define command line flags
	var (
		walletPath = flag.String("wallet", "wallet.key", "Path to wallet file")
		showWallet = flag.Bool("show-wallet", false, "Show wallet address and exit")
		debug      = flag.Bool("debug", false, "Show additional wallet information")
		network    = flag.String("network", "mainnet", "Network type: mainnet, testnet, multisig")
		walletName = flag.String("name", "", "Wallet name for new wallets")

		// Storage and blockchain commands
		dbPath        = flag.String("db", "truthchain.db", "Path to database file")
		postContent   = flag.String("post", "", "Post content to the blockchain")
		showPosts     = flag.Bool("posts", false, "Show recent posts")
		showBlocks    = flag.Bool("blocks", false, "Show recent blocks")
		showStatus    = flag.Bool("status", false, "Show blockchain status")
		showMempool   = flag.Bool("mempool", false, "Show mempool (pending posts)")
		forceBlock    = flag.Bool("force-block", false, "Force creation of a new block")
		postThreshold = flag.Int("post-threshold", chain.MainnetMinPosts, "Number of posts needed for block creation")
		monitor       = flag.Bool("monitor", false, "Show live node/network stats (like top)")
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

	// Initialize storage
	storage, err := store.NewBoltDBStorage(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Initialize blockchain with storage
	blockchain, err := blockchain.NewBlockchain(storage, *postThreshold)
	if err != nil {
		log.Fatalf("Failed to initialize blockchain: %v", err)
	}

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
		fmt.Printf("Pending Posts: %d/%d\n", blockchain.GetPendingPostCount(), *postThreshold)

		if blockchain.GetPendingPostCount() >= *postThreshold {
			fmt.Printf("✅ New block created!\n")
		}
		return
	}

	if *showPosts {
		// Show recent posts from latest block
		latestBlock, err := blockchain.GetLatestBlock()
		if err != nil {
			log.Fatalf("Failed to get latest block: %v", err)
		}

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
			fmt.Printf("Pending posts: %d/%d\n", blockchain.GetPendingPostCount(), *postThreshold)

			// Show pending posts details
			pendingPosts := blockchain.GetPendingPosts()
			for i, post := range pendingPosts {
				fmt.Printf("  ⏳ Pending Post %d:\n", i+1)
				fmt.Printf("    Author: %s\n", post.Author)
				fmt.Printf("    Content: %s\n", post.Content)
				fmt.Printf("    Characters: %d\n", post.GetCharacterCount())
				fmt.Printf("    Hash: %s\n", post.Hash)
				fmt.Printf("    Timestamp: %d\n\n", post.Timestamp)
			}
		}
		return
	}

	if *showBlocks {
		// Show recent blocks
		chainLength, err := blockchain.GetChainLength()
		if err != nil {
			log.Fatalf("Failed to get chain length: %v", err)
		}
		fmt.Printf("Blockchain length: %d blocks\n\n", chainLength)

		// Show last 5 blocks (or all if less than 5)
		start := 0
		if chainLength > 5 {
			start = chainLength - 5
		}

		for i := start; i < chainLength; i++ {
			block, err := blockchain.GetBlockByIndex(i)
			if err != nil {
				continue
			}
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
		info, err := blockchain.GetBlockchainInfo()
		if err != nil {
			log.Fatalf("Failed to get blockchain info: %v", err)
		}

		fmt.Printf("TruthChain Status:\n")
		fmt.Printf("  Chain Length: %v\n", info["chain_length"])
		fmt.Printf("  Total Posts: %v\n", info["total_post_count"])
		fmt.Printf("  Total Characters: %v\n", info["total_character_count"])
		fmt.Printf("  Pending Posts: %v\n", info["pending_post_count"])
		fmt.Printf("  Pending Characters: %v/%v\n", info["pending_character_count"], info["character_threshold"])

		if latestBlock, err := blockchain.GetLatestBlock(); err == nil && latestBlock != nil {
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

	if *showMempool {
		// Show mempool information
		mempoolInfo := blockchain.GetMempoolInfo()
		fmt.Printf("TruthChain Mempool:\n")
		fmt.Printf("  Pending Posts: %v\n", mempoolInfo["pending_post_count"])
		fmt.Printf("  Pending Characters: %v/%v\n", mempoolInfo["pending_character_count"], mempoolInfo["character_threshold"])

		posts := mempoolInfo["posts"].([]map[string]interface{})
		if len(posts) > 0 {
			fmt.Printf("\nPending Posts:\n")
			for i, post := range posts {
				fmt.Printf("  ⏳ Post %d:\n", i+1)
				fmt.Printf("    Hash: %s\n", post["hash"])
				fmt.Printf("    Author: %s\n", post["author"])
				fmt.Printf("    Content: %s\n", post["content"])
				fmt.Printf("    Characters: %v\n", post["characters"])
				fmt.Printf("    Timestamp: %v\n\n", post["timestamp"])
			}
		} else {
			fmt.Printf("\nNo pending posts in mempool.\n")
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

		chainLength, err := blockchain.GetChainLength()
		if err != nil {
			log.Fatalf("Failed to get chain length: %v", err)
		}

		fmt.Printf("✅ Block created successfully!\n")
		fmt.Printf("New block index: %d\n", chainLength-1)
		return
	}

	if *monitor {
		// Initialize uptime tracker
		uptimeTracker := miner.NewUptimeTracker(w, storage)
		uptimeTracker.LoadHeartbeats() // Load heartbeats from storage

		for {
			clearScreen()
			fmt.Println("TruthChain Node Monitor (press Ctrl+C to exit)")
			fmt.Println("============================================")

			// Node stats
			uptimeInfo := uptimeTracker.GetUptimeInfo()
			fmt.Println("[Node Stats]")
			fmt.Printf("  Character Balance: %v\n", uptimeInfo["character_balance"])
			fmt.Printf("  Earning Rate: %v chars/10min, %v chars/day\n", "N/A", "N/A")
			fmt.Printf("  Uptime (24h): %.2f%%\n", uptimeInfo["uptime_24h_percent"])
			fmt.Printf("  Uptime (total): %.2f%%\n", uptimeInfo["uptime_total_percent"])
			fmt.Printf("  Heartbeats (24h): %v\n", uptimeInfo["heartbeat_count"])
			fmt.Printf("  Last Reward: %v\n", uptimeInfo["last_reward"])
			fmt.Println()

			// Blockchain stats
			chainInfo, _ := blockchain.GetBlockchainInfo()
			fmt.Println("[Blockchain Stats]")
			fmt.Printf("  Block Height: %v\n", chainInfo["chain_length"])
			fmt.Printf("  Total Posts: %v\n", chainInfo["total_post_count"])
			fmt.Printf("  Total Characters: %v\n", chainInfo["total_character_count"])
			fmt.Printf("  Pending Posts: %v\n", chainInfo["pending_post_count"])
			fmt.Printf("  Pending Characters: %v/%v\n", chainInfo["pending_character_count"], chainInfo["post_threshold"])
			fmt.Println()

			// Network stats (placeholder)
			fmt.Println("[Network Stats]")
			fmt.Printf("  Active Nodes: N/A\n")
			fmt.Printf("  Total Characters Minted: N/A\n")
			fmt.Printf("  Total Characters Burned: N/A\n")
			fmt.Printf("  Total Characters Used for Gas: N/A\n")
			fmt.Printf("  Total Posts (Network): N/A\n")
			fmt.Println()

			time.Sleep(5 * time.Second)
		}
	}

	// Normal node startup (no specific command)
	fmt.Printf("TruthChain node starting...\n")
	fmt.Printf("Wallet Address: %s\n", w.GetAddress())
	fmt.Printf("Wallet File: %s\n", *walletPath)
	fmt.Printf("Database File: %s\n", *dbPath)
	fmt.Printf("Network: %s\n", w.GetNetwork())
	fmt.Printf("Post Threshold: %d\n", *postThreshold)

	if *debug {
		fmt.Printf("Public Key (compressed): %s\n", w.ExportPublicKeyHex())
		fmt.Printf("Version Byte: 0x%02X\n", w.GetVersionByte())
		fmt.Printf("Address Valid: %t\n", wallet.ValidateAddressWithVersion(w.GetAddress(), w.GetVersionByte()))
	}

	// Show blockchain status
	info, err := blockchain.GetBlockchainInfo()
	if err != nil {
		log.Fatalf("Failed to get blockchain info: %v", err)
	}

	fmt.Printf("Blockchain Status:\n")
	fmt.Printf("  Chain Length: %v\n", info["chain_length"])
	fmt.Printf("  Pending Posts: %v\n", info["pending_post_count"])
	fmt.Printf("  Pending Characters: %v/%v\n", info["pending_character_count"], info["character_threshold"])

	// TODO: Start other node components (API, miner, etc.)
	fmt.Println("\nNode components not yet implemented - stopping.")
	fmt.Println("Use --help to see available commands.")
}
