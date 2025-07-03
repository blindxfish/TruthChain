package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/store"
	"github.com/gorilla/mux"
)

// APIServer represents a standalone HTTP API server for TruthChain
type APIServer struct {
	blockchain *blockchain.Blockchain
	storage    *store.BoltDBStorage
	router     *mux.Router
	server     *http.Server
	port       int
	isRunning  bool
	stopChan   chan struct{}
}

// NewAPIServer creates a new API server instance
func NewAPIServer(dbPath string, port int) (*APIServer, error) {
	// Initialize storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize blockchain (read-only mode)
	bc, err := blockchain.NewBlockchain(storage, 5, "truthchain-mainnet") // Default post threshold and network
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
	}

	// Create router
	router := mux.NewRouter()

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	api := &APIServer{
		blockchain: bc,
		storage:    storage,
		router:     router,
		server:     server,
		port:       port,
		stopChan:   make(chan struct{}),
	}

	// Setup routes
	api.setupRoutes()

	return api, nil
}

// setupRoutes configures all API endpoints
func (api *APIServer) setupRoutes() {
	// Health and status endpoints
	api.router.HandleFunc("/status", api.handleStatus).Methods("GET")
	api.router.HandleFunc("/health", api.handleHealth).Methods("GET")
	api.router.HandleFunc("/info", api.handleInfo).Methods("GET")

	// Blockchain endpoints
	api.router.HandleFunc("/blockchain/latest", api.handleLatestBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks", api.handleGetBlocks).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}", api.handleGetBlockByIndex).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/hash/{hash}", api.handleGetBlockByHash).Methods("GET")
	api.router.HandleFunc("/blockchain/length", api.handleChainLength).Methods("GET")

	// Post endpoints
	api.router.HandleFunc("/posts", api.handleGetPosts).Methods("GET")
	api.router.HandleFunc("/posts/pending", api.handleGetPendingPosts).Methods("GET")
	api.router.HandleFunc("/posts/{hash}", api.handleGetPostByHash).Methods("GET")

	// Transfer endpoints
	api.router.HandleFunc("/transfers", api.handleGetTransfers).Methods("GET")
	api.router.HandleFunc("/transfers/pending", api.handleGetPendingTransfers).Methods("GET")

	// Wallet endpoints
	api.router.HandleFunc("/wallets", api.handleGetWallets).Methods("GET")
	api.router.HandleFunc("/wallets/{address}", api.handleGetWallet).Methods("GET")
	api.router.HandleFunc("/wallets/{address}/balance", api.handleGetBalance).Methods("GET")

	// Network endpoints
	api.router.HandleFunc("/network/stats", api.handleNetworkStats).Methods("GET")
	api.router.HandleFunc("/network/peers", api.handleGetPeers).Methods("GET")

	// Add CORS headers
	api.router.Use(api.corsMiddleware)
}

// corsMiddleware adds CORS headers to all responses
func (api *APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Start begins the API server
func (api *APIServer) Start() error {
	if api.isRunning {
		return fmt.Errorf("API server is already running")
	}

	api.isRunning = true
	log.Printf("Starting TruthChain API server on port %d", api.port)

	// Start server in goroutine
	go func() {
		if err := api.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()

	// Setup graceful shutdown
	go api.handleShutdown()

	return nil
}

// Stop gracefully shuts down the API server
func (api *APIServer) Stop() error {
	if !api.isRunning {
		return fmt.Errorf("API server is not running")
	}

	log.Printf("Stopping TruthChain API server...")
	api.isRunning = false

	// Close stop channel
	close(api.stopChan)

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := api.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	// Close storage
	if err := api.storage.Close(); err != nil {
		log.Printf("Warning: failed to close storage: %v", err)
	}

	log.Printf("TruthChain API server stopped")
	return nil
}

// handleShutdown sets up graceful shutdown handling
func (api *APIServer) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Printf("Received shutdown signal")
	case <-api.stopChan:
		log.Printf("Received stop request")
	}

	api.Stop()
}

// handleStatus returns the overall status of the TruthChain node
func (api *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	info, err := api.blockchain.GetBlockchainInfo()
	if err != nil {
		api.sendError(w, "Failed to get blockchain info", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":     "running",
		"timestamp":  time.Now().Unix(),
		"blockchain": info,
		"api": map[string]interface{}{
			"port":    api.port,
			"version": "1.0.0",
			"uptime":  time.Since(time.Now()).String(), // TODO: track actual uptime
		},
	}

	api.sendJSON(w, response)
}

// handleHealth returns a simple health check
func (api *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}
	api.sendJSON(w, response)
}

// handleInfo returns detailed information about the TruthChain node
func (api *APIServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	info, err := api.blockchain.GetBlockchainInfo()
	if err != nil {
		api.sendError(w, "Failed to get blockchain info", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"name":        "TruthChain",
		"version":     "1.0.0",
		"description": "Decentralized Truth Network",
		"blockchain":  info,
		"features": []string{
			"Immutable Posts",
			"Character Currency",
			"Uptime Mining",
			"Mesh Network",
			"Beacon Discovery",
			"Transfer System",
		},
	}

	api.sendJSON(w, response)
}

// handleLatestBlock returns the latest block
func (api *APIServer) handleLatestBlock(w http.ResponseWriter, r *http.Request) {
	block, err := api.blockchain.GetLatestBlock()
	if err != nil {
		api.sendError(w, "Failed to get latest block", http.StatusInternalServerError)
		return
	}

	api.sendJSON(w, block)
}

// handleGetBlocks returns a range of blocks
func (api *APIServer) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement pagination and range queries
	api.sendError(w, "Not implemented", http.StatusNotImplemented)
}

// handleGetBlockByIndex returns a block by its index
func (api *APIServer) handleGetBlockByIndex(w http.ResponseWriter, r *http.Request) {
	_ = mux.Vars(r)["index"]

	// TODO: Parse index and return block
	api.sendError(w, "Not implemented", http.StatusNotImplemented)
}

// handleGetBlockByHash returns a block by its hash
func (api *APIServer) handleGetBlockByHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["hash"]

	block, err := api.blockchain.GetBlockByHash(hash)
	if err != nil {
		api.sendError(w, "Block not found", http.StatusNotFound)
		return
	}

	api.sendJSON(w, block)
}

// handleChainLength returns the current chain length
func (api *APIServer) handleChainLength(w http.ResponseWriter, r *http.Request) {
	length, err := api.blockchain.GetChainLength()
	if err != nil {
		api.sendError(w, "Failed to get chain length", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"length": length,
	}
	api.sendJSON(w, response)
}

// handleGetPosts returns recent posts
func (api *APIServer) handleGetPosts(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement post retrieval with pagination
	api.sendError(w, "Not implemented", http.StatusNotImplemented)
}

// handleGetPendingPosts returns pending posts
func (api *APIServer) handleGetPendingPosts(w http.ResponseWriter, r *http.Request) {
	posts := api.blockchain.GetPendingPosts()
	api.sendJSON(w, posts)
}

// handleGetPostByHash returns a post by its hash
func (api *APIServer) handleGetPostByHash(w http.ResponseWriter, r *http.Request) {
	_ = mux.Vars(r)["hash"]

	// TODO: Implement post retrieval by hash
	api.sendError(w, "Not implemented", http.StatusNotImplemented)
}

// handleGetTransfers returns recent transfers
func (api *APIServer) handleGetTransfers(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement transfer retrieval
	api.sendError(w, "Not implemented", http.StatusNotImplemented)
}

// handleGetPendingTransfers returns pending transfers
func (api *APIServer) handleGetPendingTransfers(w http.ResponseWriter, r *http.Request) {
	poolInfo := api.blockchain.GetTransferPoolInfo()
	api.sendJSON(w, poolInfo)
}

// handleGetWallets returns all wallet states
func (api *APIServer) handleGetWallets(w http.ResponseWriter, r *http.Request) {
	stateInfo := api.blockchain.GetStateInfo()
	api.sendJSON(w, stateInfo)
}

// handleGetWallet returns a specific wallet state
func (api *APIServer) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	balance, err := api.blockchain.GetCharacterBalance(address)
	if err != nil {
		api.sendError(w, "Wallet not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"address": address,
		"balance": balance,
	}
	api.sendJSON(w, response)
}

// handleGetBalance returns a wallet's character balance
func (api *APIServer) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	balance, err := api.blockchain.GetCharacterBalance(address)
	if err != nil {
		api.sendError(w, "Wallet not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"address": address,
		"balance": balance,
	}
	api.sendJSON(w, response)
}

// handleNetworkStats returns network statistics
func (api *APIServer) handleNetworkStats(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement network stats when network is available
	response := map[string]interface{}{
		"status": "network_stats_not_available",
		"note":   "Network stats require active mesh network connection",
	}
	api.sendJSON(w, response)
}

// handleGetPeers returns connected peers
func (api *APIServer) handleGetPeers(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement peer list when network is available
	response := map[string]interface{}{
		"status": "peers_not_available",
		"note":   "Peer list require active mesh network connection",
	}
	api.sendJSON(w, response)
}

// sendJSON sends a JSON response
func (api *APIServer) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// sendError sends an error response
func (api *APIServer) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error":   message,
		"status":  statusCode,
		"success": false,
	}

	json.NewEncoder(w).Encode(response)
}
