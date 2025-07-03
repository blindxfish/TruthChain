package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/blindxfish/truthchain/api"
)

func main() {
	// Parse command line flags
	var (
		dbPath = flag.String("db", "truthchain.db", "Path to TruthChain database file")
		port   = flag.Int("port", 8080, "Port to run the API server on")
		help   = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		fmt.Println("TruthChain Standalone API Server")
		fmt.Println("Usage: api-server [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  api-server -db truthchain.db -port 8080")
		fmt.Println("  api-server -port 9090")
		return
	}

	// Check if database file exists
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("Database file not found: %s", *dbPath)
	}

	// Create API server
	server, err := api.NewStandaloneAPIServer(*dbPath, *port)
	if err != nil {
		log.Fatalf("Failed to create API server: %v", err)
	}

	// Start the server
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}

	log.Printf("TruthChain API server started on port %d", *port)
	log.Printf("Database: %s", *dbPath)
	log.Printf("API endpoints:")
	log.Printf("  GET  /status")
	log.Printf("  GET  /health")
	log.Printf("  GET  /info")
	log.Printf("  GET  /blockchain/latest")
	log.Printf("  GET  /blockchain/length")
	log.Printf("  GET  /posts/pending")
	log.Printf("  GET  /transfers/pending")
	log.Printf("  GET  /wallets")
	log.Printf("  GET  /wallets/{address}")
	log.Printf("  GET  /wallets/{address}/balance")
	log.Printf("")
	log.Printf("Press Ctrl+C to stop the server")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("Shutting down API server...")
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
}
