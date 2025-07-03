package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/blindxfish/truthchain/api"
	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/miner"
	"github.com/blindxfish/truthchain/network"
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

// getLocalIP returns the local IP address for network binding
func getLocalIP() string {
	// For now, return localhost - in production this would detect the actual IP
	return "127.0.0.1"
}

func main() {
	// Define command line flags
	var (
		walletPath  = flag.String("wallet", "wallet.key", "Path to wallet file")
		showWallet  = flag.Bool("show-wallet", false, "Show wallet address and exit")
		debug       = flag.Bool("debug", false, "Show additional wallet information")
		networkType = flag.String("network", "mainnet", "Network type: mainnet, testnet, multisig")
		walletName  = flag.String("name", "", "Wallet name for new wallets")

		// Storage and blockchain commands
		dbPath           = flag.String("db", "truthchain.db", "Path to database file")
		postContent      = flag.String("post", "", "Post content to the blockchain")
		showPosts        = flag.Bool("posts", false, "Show recent posts")
		showBlocks       = flag.Bool("blocks", false, "Show recent blocks")
		showStatus       = flag.Bool("status", false, "Show blockchain status")
		showMempool      = flag.Bool("mempool", false, "Show mempool (pending posts)")
		forceBlock       = flag.Bool("force-block", false, "Force creation of a new block")
		postThreshold    = flag.Int("post-threshold", chain.MainnetMinPosts, "Number of posts needed for block creation")
		monitor          = flag.Bool("monitor", false, "Show live node/network stats (like top)")
		apiPort          = flag.Int("api-port", 0, "Start HTTP API server on port (0 = disabled)")
		sendTo           = flag.String("send", "", "Send characters to address")
		sendAmount       = flag.Int("amount", 0, "Amount of characters to send")
		showTransfers    = flag.Bool("show-transfers", false, "Show transfer pool information")
		processTransfers = flag.Bool("process-transfers", false, "Process all pending transfers")
		showState        = flag.Bool("show-state", false, "Show current blockchain state")
		showWallets      = flag.Bool("show-wallets", false, "Show all wallet states")
		addBalance       = flag.Int("add-balance", 0, "Add balance to current wallet (for testing)")

		// Network and sync flags
		syncPort   = flag.Int("sync-port", 0, "Start sync server on port (0 = disabled)")
		syncFrom   = flag.String("sync-from", "", "Sync blocks from peer address (e.g., 192.168.1.100:9876)")
		beaconMode = flag.Bool("beacon", false, "Enable beacon mode for peer discovery")
		beaconIP   = flag.String("beacon-ip", "", "Beacon IP address (required with --beacon)")
		beaconPort = flag.Int("beacon-port", 9876, "Beacon port (default: 9876)")
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
		switch *networkType {
		case "mainnet":
			w, err = wallet.NewWalletWithMetadata(*walletName, wallet.TruthChainMainnetVersion)
		case "testnet":
			w, err = wallet.NewTestnetWallet(*walletName)
		case "multisig":
			w, err = wallet.NewMultisigWallet(*walletName)
		default:
			log.Fatalf("Invalid network type: %s. Use mainnet, testnet, or multisig", *networkType)
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

	if *sendTo != "" && *sendAmount > 0 {
		// Create and add transfer
		transfer, err := blockchain.CreateTransfer(*sendTo, *sendAmount, w)
		if err != nil {
			log.Fatalf("Failed to create transfer: %v", err)
		}

		if err := blockchain.AddTransfer(*transfer); err != nil {
			log.Fatalf("Failed to add transfer: %v", err)
		}

		fmt.Printf("✅ Transfer created successfully!\n")
		fmt.Printf("From: %s\n", transfer.From)
		fmt.Printf("To: %s\n", transfer.To)
		fmt.Printf("Amount: %d characters\n", transfer.Amount)
		fmt.Printf("Gas Fee: %d character\n", transfer.GasFee)
		fmt.Printf("Total Cost: %d characters\n", transfer.GetTotalCost())
		fmt.Printf("Hash: %s\n", transfer.Hash)
		fmt.Printf("Nonce: %d\n", transfer.Nonce)
		return
	}

	if *showTransfers {
		// Show transfer pool information
		transferInfo := blockchain.GetTransferPoolInfo()
		fmt.Printf("TruthChain Transfer Pool:\n")
		fmt.Printf("  Pending Transfers: %v\n", transferInfo["transfer_count"])
		fmt.Printf("  Total Character Volume: %v\n", transferInfo["total_character_volume"])

		transfers := transferInfo["transfers"].([]map[string]interface{})
		if len(transfers) > 0 {
			fmt.Printf("\nPending Transfers:\n")
			for i, transfer := range transfers {
				fmt.Printf("  ⏳ Transfer %d:\n", i+1)
				fmt.Printf("    Hash: %s\n", transfer["hash"])
				fmt.Printf("    From: %s\n", transfer["from"])
				fmt.Printf("    To: %s\n", transfer["to"])
				fmt.Printf("    Amount: %v\n", transfer["amount"])
				fmt.Printf("    Gas Fee: %v\n", transfer["gas_fee"])
				fmt.Printf("    Timestamp: %v\n", transfer["timestamp"])
				fmt.Printf("    Nonce: %v\n\n", transfer["nonce"])
			}
		} else {
			fmt.Printf("\nNo pending transfers in pool.\n")
		}
		return
	}

	if *processTransfers {
		// Process pending transfers
		if err := blockchain.ProcessTransfers(); err != nil {
			log.Fatalf("Failed to process transfers: %v", err)
		}

		fmt.Printf("✅ Transfers processed successfully!\n")
		return
	}

	if *showState {
		stateInfo := blockchain.GetStateInfo()

		fmt.Printf("TruthChain State:\n")
		fmt.Printf("  Wallet Count: %v\n", stateInfo["wallet_count"])
		fmt.Printf("  Total Character Supply: %v\n", stateInfo["total_character_supply"])

		wallets := stateInfo["wallets"].([]map[string]interface{})
		if len(wallets) > 0 {
			fmt.Printf("\nWallets:\n")
			for i, wallet := range wallets {
				fmt.Printf("  %d. %s\n", i+1, wallet["address"])
				fmt.Printf("     Balance: %v characters\n", wallet["balance"])
				fmt.Printf("     Nonce: %v\n", wallet["nonce"])
				fmt.Printf("     Last TX: %v\n", wallet["last_tx_time"])
			}
		}
		return
	}

	if *showWallets {
		stateInfo := blockchain.GetStateInfo()
		wallets := stateInfo["wallets"].([]map[string]interface{})

		if len(wallets) == 0 {
			fmt.Printf("No wallets in state.\n")
			return
		}

		fmt.Printf("Wallet States (%d total):\n", len(wallets))
		fmt.Printf("%-50s %-15s %-10s %-20s\n", "Address", "Balance", "Nonce", "Last Transaction")
		fmt.Printf("%s\n", strings.Repeat("-", 95))

		for _, wallet := range wallets {
			address := wallet["address"].(string)
			balance := wallet["balance"].(int)
			nonce := wallet["nonce"].(int64)
			lastTx := wallet["last_tx_time"].(int64)

			// Format last transaction time
			lastTxStr := "Never"
			if lastTx > 0 {
				lastTxStr = time.Unix(lastTx, 0).Format("2006-01-02 15:04:05")
			}

			fmt.Printf("%-50s %-15d %-10d %-20s\n", address, balance, nonce, lastTxStr)
		}
		return
	}

	if *addBalance > 0 {
		// Get current balance from storage
		currentBalance, err := blockchain.GetCharacterBalance(w.GetAddress())
		if err != nil {
			// If wallet doesn't exist in storage, start with 0
			currentBalance = 0
		}

		// Add balance to current wallet
		if err := blockchain.UpdateCharacterBalance(w.GetAddress(), *addBalance); err != nil {
			log.Fatalf("Failed to add balance: %v", err)
		}

		// Calculate total balance
		totalBalance := currentBalance + *addBalance

		// Update state manager with total balance
		blockchain.UpdateWalletState(w.GetAddress(), totalBalance, 0)

		fmt.Printf("✅ Added %d characters to wallet %s\n", *addBalance, w.GetAddress())
		fmt.Printf("Previous balance: %d characters\n", currentBalance)
		fmt.Printf("New total balance: %d characters\n", totalBalance)
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

	// Start API server if requested
	if *apiPort > 0 {
		// Initialize uptime tracker
		uptimeTracker := miner.NewUptimeTracker(w, storage)
		uptimeTracker.LoadHeartbeats()

		// Create and start API server
		server := api.NewServer(blockchain, uptimeTracker, w, storage, *apiPort)

		fmt.Printf("Starting TruthChain API server on port %d...\n", *apiPort)
		fmt.Printf("API endpoints:\n")
		fmt.Printf("  GET  http://127.0.0.1:%d/status\n", *apiPort)
		fmt.Printf("  GET  http://127.0.0.1:%d/wallet\n", *apiPort)
		fmt.Printf("  POST http://127.0.0.1:%d/post\n", *apiPort)
		fmt.Printf("  GET  http://127.0.0.1:%d/posts/latest\n", *apiPort)
		fmt.Printf("  POST http://127.0.0.1:%d/characters/send\n", *apiPort)
		fmt.Printf("  GET  http://127.0.0.1:%d/uptime\n", *apiPort)
		fmt.Printf("  GET  http://127.0.0.1:%d/balance\n", *apiPort)
		fmt.Println()

		if err := server.Start(); err != nil {
			log.Fatalf("API server failed: %v", err)
		}
		return
	}

	// Handle sync operations
	if *syncFrom != "" {
		fmt.Printf("Syncing blocks from peer: %s\n", *syncFrom)

		// Get current chain length
		currentLength, err := blockchain.GetChainLength()
		if err != nil {
			log.Fatalf("Failed to get chain length: %v", err)
		}

		// Request blocks from current length onwards
		resp, err := network.SyncFromPeerTCP(*syncFrom, currentLength, -1, w.GetAddress())
		if err != nil {
			log.Fatalf("Failed to sync from peer: %v", err)
		}

		fmt.Printf("Received %d blocks from peer\n", len(resp.Blocks))
		fmt.Printf("Blocks range: %d to %d\n", resp.FromIndex, resp.ToIndex)

		// TODO: Validate and integrate received blocks
		// For now, just show what we received
		for _, block := range resp.Blocks {
			fmt.Printf("  Block %d: %d posts, %d characters\n",
				block.Index, len(block.Posts), block.CharCount)
		}
		return
	}

	// Start sync server if requested
	if *syncPort > 0 {
		fmt.Printf("Starting sync server on port %d...\n", *syncPort)
		fmt.Printf("Other nodes can sync from: %s:%d\n",
			getLocalIP(), *syncPort)

		// Start sync server in background
		go func() {
			if err := network.StartSyncServer(fmt.Sprintf(":%d", *syncPort), blockchain, w.GetAddress()); err != nil {
				log.Printf("Sync server failed: %v", err)
			}
		}()
	}

	// Handle beacon mode
	if *beaconMode {
		if *beaconIP == "" {
			log.Fatalf("Beacon IP address required when using --beacon flag")
		}

		fmt.Printf("Starting in beacon mode...\n")
		fmt.Printf("Beacon Address: %s:%d\n", *beaconIP, *beaconPort)
		fmt.Printf("Other nodes can discover this beacon from the blockchain\n")

		// TODO: Create beacon announcement and add to next block
		// For now, just indicate beacon mode is enabled
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

	// Show available features
	fmt.Println("\nAvailable Features:")
	fmt.Println("  ✅ Wallet Management")
	fmt.Println("  ✅ Blockchain Operations")
	fmt.Println("  ✅ Post Creation & Management")
	fmt.Println("  ✅ Character Transfer System")
	fmt.Println("  ✅ Uptime Mining & Rewards")
	fmt.Println("  ✅ HTTP API Server")
	fmt.Println("  ✅ Live Monitoring Dashboard")

	fmt.Println("\nTo start the HTTP API server:")
	fmt.Printf("  go run cmd/main.go --api-port 8080\n")
	fmt.Println("\nTo view live node stats:")
	fmt.Printf("  go run cmd/main.go --monitor\n")
	fmt.Println("\nUse --help to see all available commands.")
}
