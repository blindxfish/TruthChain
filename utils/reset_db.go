package main

import (
	"fmt"
	"log"
	"os"

	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/reset_db.go <database_path>")
		fmt.Println("Example: go run cmd/reset_db.go truthchain.db")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	fmt.Printf("=== TruthChain Database Reset Utility ===\n")
	fmt.Printf("Database: %s\n\n", dbPath)

	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("❌ Database file not found: %s\n", dbPath)
		os.Exit(1)
	}

	// Close any existing connections and remove the file
	fmt.Printf("Removing existing database...\n")
	if err := os.Remove(dbPath); err != nil {
		log.Fatalf("Failed to remove database: %v", err)
	}
	fmt.Printf("✅ Removed existing database\n")

	// Create new storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to create new storage: %v", err)
	}
	defer storage.Close()

	// Create and save genesis block
	fmt.Printf("Creating new genesis block...\n")
	genesis := chain.CreateGenesisBlock()
	if err := storage.SaveBlock(genesis); err != nil {
		log.Fatalf("Failed to save genesis block: %v", err)
	}
	fmt.Printf("✅ Created new genesis block\n")

	// Verify the genesis block
	if !chain.IsMainnetGenesis(genesis) {
		fmt.Printf("❌ Genesis block doesn't match mainnet!\n")
		os.Exit(1)
	}

	// Verify final state
	blockCount, err := storage.GetBlockCount()
	if err != nil {
		log.Fatalf("Failed to get block count: %v", err)
	}

	pendingPosts, err := storage.GetPendingPosts()
	if err != nil {
		log.Fatalf("Failed to get pending posts: %v", err)
	}

	fmt.Printf("\n✅ Database reset completed!\n")
	fmt.Printf("Final state:\n")
	fmt.Printf("  Total blocks: %d (genesis only)\n", blockCount)
	fmt.Printf("  Pending posts: %d\n", len(pendingPosts))
	fmt.Printf("  Genesis hash: %s\n", genesis.Hash)
	fmt.Printf("  Genesis timestamp: %d\n", genesis.Timestamp)

	fmt.Printf("\n🎉 Database is now ready for mainnet rules!\n")
	fmt.Printf("You can now use the CLI with 5-post threshold:\n")
	fmt.Printf("  go run cmd/main.go --post \"Your first mainnet post\"\n")
}
