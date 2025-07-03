package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/miner"
	"github.com/blindxfish/truthchain/network"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
	"github.com/gorilla/mux"
)

// TruthChainNode represents the main TruthChain node
type TruthChainNode struct {
	blockchain   *blockchain.Blockchain
	storage      *store.BoltDBStorage
	wallet       *wallet.Wallet
	trustNetwork *network.TrustNetwork
	beacon       *network.BeaconManager
	miner        *miner.UptimeTracker
	apiServer    *http.Server
	router       *mux.Router
	config       *NodeConfig
	isRunning    bool
	stopChan     chan struct{}
}

// NodeConfig holds the node configuration
type NodeConfig struct {
	DBPath        string
	APIPort       int
	MeshPort      int
	SyncPort      int
	PostThreshold int
	NetworkID     string
	BeaconMode    bool
	MeshMode      bool
	MiningMode    bool
	APIMode       bool
	Domain        string
}

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
	// Parse command line flags
	var (
		dbPath        = flag.String("db", "truthchain.db", "Path to database file")
		apiPort       = flag.Int("api-port", 8080, "API server port")
		meshPort      = flag.Int("mesh-port", 9876, "Mesh network port")
		syncPort      = flag.Int("sync-port", 9877, "Chain sync port")
		postThreshold = flag.Int("post-threshold", 5, "Posts needed to create a block")
		networkID     = flag.String("network", "truthchain-mainnet", "Network identifier")
		beaconMode    = flag.Bool("beacon", false, "Run in beacon mode")
		meshMode      = flag.Bool("mesh", false, "Run in mesh mode")
		miningMode    = flag.Bool("mining", false, "Enable uptime mining")
		apiMode       = flag.Bool("api", true, "Enable API server")
		domain        = flag.String("domain", "", "Domain for beacon announcements")
		help          = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// Create node configuration
	config := &NodeConfig{
		DBPath:        *dbPath,
		APIPort:       *apiPort,
		MeshPort:      *meshPort,
		SyncPort:      *syncPort,
		PostThreshold: *postThreshold,
		NetworkID:     *networkID,
		BeaconMode:    *beaconMode,
		MeshMode:      *meshMode,
		MiningMode:    *miningMode,
		APIMode:       *apiMode,
		Domain:        *domain,
	}

	// Create and start node
	node, err := NewTruthChainNode(config)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	// Start the node
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Printf("Shutting down TruthChain node...")
	if err := node.Stop(); err != nil {
		log.Printf("Error stopping node: %v", err)
	}
}

// NewTruthChainNode creates a new TruthChain node
func NewTruthChainNode(config *NodeConfig) (*TruthChainNode, error) {
	// Initialize storage with better options for concurrent access
	storage, err := store.NewBoltDBStorage(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize blockchain
	blockchain, err := blockchain.NewBlockchain(storage, config.PostThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
	}

	// Initialize wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize wallet: %w", err)
	}

	// Create TrustNetwork (mesh manager is handled inside it)
	trustNet := network.NewTrustNetwork(
		wallet.GetAddress(),
		wallet,
		storage,
		nil, // UptimeTracker (set later if mining enabled)
		blockchain,
		config.MeshPort,
		"", // Bootstrap config (can be a file or string)
	)

	// Create router for API
	router := mux.NewRouter()

	// Create API server
	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.APIPort),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	node := &TruthChainNode{
		blockchain:   blockchain,
		storage:      storage,
		wallet:       wallet,
		trustNetwork: trustNet,
		apiServer:    apiServer,
		router:       router,
		config:       config,
		stopChan:     make(chan struct{}),
	}

	// Initialize network components if enabled
	if config.MeshMode {
		if err := node.initializeMesh(); err != nil {
			return nil, fmt.Errorf("failed to initialize mesh: %w", err)
		}
	}

	if config.BeaconMode {
		if err := node.initializeBeacon(); err != nil {
			return nil, fmt.Errorf("failed to initialize beacon: %w", err)
		}
	}

	if config.MiningMode {
		if err := node.initializeMiner(); err != nil {
			return nil, fmt.Errorf("failed to initialize miner: %w", err)
		}
	}

	// Setup API routes if enabled
	if config.APIMode {
		node.setupAPIRoutes()
	}

	return node, nil
}

// initializeMesh sets up the mesh network manager
func (n *TruthChainNode) initializeMesh() error {
	if n.trustNetwork == nil {
		return fmt.Errorf("trust network not initialized")
	}
	return n.trustNetwork.Start()
}

// initializeBeacon sets up the beacon manager
func (n *TruthChainNode) initializeBeacon() error {
	// Convert btcec keys to ecdsa keys
	privateKey := n.wallet.PrivateKey.ToECDSA()
	publicKey := n.wallet.PublicKey.ToECDSA()

	beacon := network.NewBeaconManager(privateKey, publicKey)

	// Enable beacon mode if domain is provided
	if n.config.Domain != "" {
		beacon.EnableBeacon(n.config.Domain, n.config.MeshPort)
	}

	n.beacon = beacon
	return nil
}

// initializeMiner sets up the uptime miner
func (n *TruthChainNode) initializeMiner() error {
	beaconChecker := &beaconCheckerAdapter{beacon: n.beacon}
	miner := miner.NewUptimeTracker(n.wallet, n.storage, beaconChecker)
	n.miner = miner
	// Attach miner to trust network for uptime tracking
	if n.trustNetwork != nil {
		n.trustNetwork.UptimeTracker = miner
	}
	return nil
}

// beaconCheckerAdapter adapts BeaconManager to BeaconChecker interface
type beaconCheckerAdapter struct {
	beacon *network.BeaconManager
}

func (bca *beaconCheckerAdapter) IsBeaconMode() bool {
	if bca.beacon == nil {
		return false
	}
	return bca.beacon.IsBeaconMode()
}

func (bca *beaconCheckerAdapter) GetBeaconUptime() float64 {
	if bca.beacon == nil {
		return 0.0
	}
	return bca.beacon.GetBeaconUptime()
}

// setupAPIRoutes configures the API endpoints
func (n *TruthChainNode) setupAPIRoutes() {
	// Health and status endpoints
	n.router.HandleFunc("/status", n.handleStatus).Methods("GET")
	n.router.HandleFunc("/health", n.handleHealth).Methods("GET")
	n.router.HandleFunc("/info", n.handleInfo).Methods("GET")

	// Blockchain endpoints
	n.router.HandleFunc("/blockchain/latest", n.handleLatestBlock).Methods("GET")
	n.router.HandleFunc("/blockchain/length", n.handleChainLength).Methods("GET")

	// Post endpoints
	n.router.HandleFunc("/posts", n.handleCreatePost).Methods("POST")
	n.router.HandleFunc("/posts/pending", n.handleGetPendingPosts).Methods("GET")

	// Transfer endpoints
	n.router.HandleFunc("/transfers", n.handleCreateTransfer).Methods("POST")
	n.router.HandleFunc("/transfers/pending", n.handleGetPendingTransfers).Methods("GET")

	// Wallet endpoints
	n.router.HandleFunc("/wallets", n.handleGetWallets).Methods("GET")
	n.router.HandleFunc("/wallets/{address}", n.handleGetWallet).Methods("GET")
	n.router.HandleFunc("/wallets/{address}/balance", n.handleGetBalance).Methods("GET")

	// Network endpoints
	n.router.HandleFunc("/network/stats", n.handleNetworkStats).Methods("GET")
	n.router.HandleFunc("/network/peers", n.handleGetPeers).Methods("GET")

	// Add CORS headers
	n.router.Use(n.corsMiddleware)
}

// Start begins the TruthChain node
func (n *TruthChainNode) Start() error {
	if n.isRunning {
		return fmt.Errorf("node is already running")
	}

	n.isRunning = true
	log.Printf("Starting TruthChain node...")

	// Start mesh network if enabled
	if n.trustNetwork != nil {
		if err := n.trustNetwork.Start(); err != nil {
			return fmt.Errorf("failed to start trust network: %w", err)
		}
		log.Printf("Trust network started")
	}

	// Start miner if enabled
	if n.miner != nil {
		if err := n.miner.Start(); err != nil {
			return fmt.Errorf("failed to start miner: %w", err)
		}
		log.Printf("Uptime miner started")
	}

	// Start API server if enabled
	if n.config.APIMode {
		go func() {
			if err := n.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("API server error: %v", err)
			}
		}()
		log.Printf("API server started on port %d", n.config.APIPort)
	}

	log.Printf("TruthChain node started successfully")
	log.Printf("Wallet address: %s", n.wallet.GetAddress())
	log.Printf("Network: %s", n.config.NetworkID)
	log.Printf("Post threshold: %d", n.config.PostThreshold)

	return nil
}

// Stop gracefully shuts down the TruthChain node
func (n *TruthChainNode) Stop() error {
	if !n.isRunning {
		return fmt.Errorf("node is not running")
	}

	log.Printf("Stopping TruthChain node...")
	n.isRunning = false

	// Close stop channel
	close(n.stopChan)

	// Stop miner if running
	if n.miner != nil {
		n.miner.Stop()
	}

	// Stop trust network if running
	if n.trustNetwork != nil {
		n.trustNetwork.Stop()
	}

	// Shutdown API server if running
	if n.config.APIMode {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := n.apiServer.Shutdown(ctx); err != nil {
			log.Printf("Warning: failed to shutdown API server: %v", err)
		}
	}

	// Close blockchain and storage
	if err := n.blockchain.Close(); err != nil {
		log.Printf("Warning: failed to close blockchain: %v", err)
	}

	if err := n.storage.Close(); err != nil {
		log.Printf("Warning: failed to close storage: %v", err)
	}

	log.Printf("TruthChain node stopped")
	return nil
}

// API handlers
func (n *TruthChainNode) handleStatus(w http.ResponseWriter, r *http.Request) {
	info, err := n.blockchain.GetBlockchainInfo()
	if err != nil {
		http.Error(w, "Failed to get blockchain info", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":     "running",
		"timestamp":  time.Now().Unix(),
		"blockchain": info,
		"node": map[string]interface{}{
			"address":     n.wallet.GetAddress(),
			"network":     n.config.NetworkID,
			"beacon_mode": n.config.BeaconMode,
			"mesh_mode":   n.config.MeshMode,
			"mining_mode": n.config.MiningMode,
			"api_mode":    n.config.APIMode,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) handleInfo(w http.ResponseWriter, r *http.Request) {
	info, err := n.blockchain.GetBlockchainInfo()
	if err != nil {
		http.Error(w, "Failed to get blockchain info", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"name":        "TruthChain",
		"version":     "1.0.0",
		"description": "Decentralized Truth Network",
		"blockchain":  info,
		"node": map[string]interface{}{
			"address":     n.wallet.GetAddress(),
			"network":     n.config.NetworkID,
			"beacon_mode": n.config.BeaconMode,
			"mesh_mode":   n.config.MeshMode,
			"mining_mode": n.config.MiningMode,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) handleLatestBlock(w http.ResponseWriter, r *http.Request) {
	block, err := n.blockchain.GetLatestBlock()
	if err != nil {
		http.Error(w, "Failed to get latest block", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(block)
}

func (n *TruthChainNode) handleChainLength(w http.ResponseWriter, r *http.Request) {
	length, err := n.blockchain.GetChainLength()
	if err != nil {
		http.Error(w, "Failed to get chain length", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"length": length,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	post, err := n.blockchain.CreatePost(req.Content, n.wallet)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create post: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(post)
}

func (n *TruthChainNode) handleGetPendingPosts(w http.ResponseWriter, r *http.Request) {
	posts := n.blockchain.GetPendingPosts()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func (n *TruthChainNode) handleCreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To     string `json:"to"`
		Amount int    `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	transfer, err := n.blockchain.CreateTransfer(req.To, req.Amount, n.wallet)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create transfer: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transfer)
}

func (n *TruthChainNode) handleGetPendingTransfers(w http.ResponseWriter, r *http.Request) {
	poolInfo := n.blockchain.GetTransferPoolInfo()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(poolInfo)
}

func (n *TruthChainNode) handleGetWallets(w http.ResponseWriter, r *http.Request) {
	stateInfo := n.blockchain.GetStateInfo()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stateInfo)
}

func (n *TruthChainNode) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	balance, err := n.blockchain.GetCharacterBalance(address)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"address": address,
		"balance": balance,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	balance, err := n.blockchain.GetCharacterBalance(address)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"address": address,
		"balance": balance,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) handleNetworkStats(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"mesh_enabled":   n.trustNetwork != nil,
		"beacon_enabled": n.beacon != nil,
		"mining_enabled": n.miner != nil,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) handleGetPeers(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "peers_not_available",
		"note":   "Peer list requires active trust network connection",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (n *TruthChainNode) corsMiddleware(next http.Handler) http.Handler {
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

func printHelp() {
	fmt.Println("TruthChain Node")
	fmt.Println("Usage: truthchain [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Run full node with API")
	fmt.Println("  truthchain -beacon -mesh -mining -api")
	fmt.Println()
	fmt.Println("  # Run beacon node only")
	fmt.Println("  truthchain -beacon -domain mainnet.truth-chain.org")
	fmt.Println()
	fmt.Println("  # Run mesh node only")
	fmt.Println("  truthchain -mesh")
	fmt.Println()
	fmt.Println("  # Run with custom database")
	fmt.Println("  truthchain -db /path/to/truthchain.db")
}
