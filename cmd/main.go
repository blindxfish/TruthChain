package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
	DBPath            string
	APIPort           int
	MeshPort          int
	PostThreshold     int
	NetworkID         string
	BeaconMode        bool
	MeshMode          bool
	MiningMode        bool
	APIMode           bool
	Domain            string
	WalletPath        string
	ImportWallet      bool
	PrivateKey        string
	ConfigureFirewall bool
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
	// Check if user wants to skip interactive setup
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printHelp()
		return
	}

	// Run interactive setup
	config := runInteractiveSetup()
	if config == nil {
		log.Println("Setup cancelled by user")
		return
	}

	// Create and start node
	node, err := NewTruthChainNode(config)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	// Configure Windows Firewall if on Windows
	if (os.Getenv("OS") == "Windows_NT" && config.ConfigureFirewall) || (isLinux() && config.ConfigureFirewall) {
		exePath := ""
		if os.Getenv("OS") == "Windows_NT" {
			// Get the executable path for Windows
			var err error
			exePath, err = os.Executable()
			if err != nil {
				log.Printf("Warning: Could not get executable path for firewall configuration: %v", err)
			}
			exePath, err = filepath.Abs(exePath)
			if err != nil {
				log.Printf("Warning: Could not get absolute path for firewall configuration: %v", err)
			}
		}
		if os.Getenv("OS") == "Windows_NT" {
			if err := ConfigureFirewall(config.APIPort, config.MeshPort, exePath); err != nil {
				log.Printf("Warning: Failed to configure firewall rules: %v", err)
				log.Printf("You may need to manually allow TruthChain through Windows Firewall")
			}
		} else if isLinux() {
			if err := ConfigureLinuxFirewall(config.APIPort, config.MeshPort); err != nil {
				log.Printf("Warning: Failed to configure Linux firewall rules: %v", err)
				log.Printf("You may need to manually allow TruthChain through your Linux firewall")
			}
		}
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

	// Clean up firewall rules on Windows
	if (os.Getenv("OS") == "Windows_NT" && config.ConfigureFirewall) || (isLinux() && config.ConfigureFirewall) {
		if os.Getenv("OS") == "Windows_NT" {
			if err := RemoveFirewallRules(); err != nil {
				log.Printf("Warning: Failed to remove firewall rules: %v", err)
			}
		} else if isLinux() {
			if err := RemoveLinuxFirewallRules(config.APIPort, config.MeshPort); err != nil {
				log.Printf("Warning: Failed to remove Linux firewall rules: %v", err)
			}
		}
	}
}

// runInteractiveSetup guides the user through configuration
func runInteractiveSetup() *NodeConfig {
	clearScreen()
	fmt.Println("🌐 TruthChain Node Setup")
	fmt.Println("=========================")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Wallet Configuration
	walletConfig := configureWallet(reader)
	if walletConfig == nil {
		return nil
	}

	// Network Selection
	networkID := selectNetwork(reader)
	if networkID == "" {
		return nil
	}

	// Set post threshold based on network
	var postThreshold int
	switch networkID {
	case "truthchain-mainnet":
		postThreshold = 5
	case "truthchain-testnet":
		postThreshold = 3
	case "truthchain-local":
		postThreshold = 2
	default:
		postThreshold = 5
	}

	// Node Mode Selection
	modes := selectNodeModes(reader)
	if modes == nil {
		return nil
	}

	// Port Configuration
	ports := configurePorts(reader, modes)
	if ports == nil {
		return nil
	}

	// Domain Configuration (if beacon mode)
	var domain string
	if modes.BeaconMode {
		domain = configureDomain(reader)
		if domain == "" {
			return nil
		}
	}

	// Database Configuration
	dbPath := configureDatabase(reader)
	if dbPath == "" {
		return nil
	}

	// Show final configuration
	showFinalConfig(networkID, modes, ports, domain, dbPath, postThreshold, walletConfig)

	// Ask about firewall configuration
	firewallConfig := configureFirewall(reader)

	// Confirm configuration
	if !confirmConfiguration(reader) {
		return nil
	}

	return &NodeConfig{
		DBPath:            dbPath,
		APIPort:           ports.APIPort,
		MeshPort:          ports.MeshPort,
		PostThreshold:     postThreshold,
		NetworkID:         networkID,
		BeaconMode:        modes.BeaconMode,
		MeshMode:          modes.MeshMode,
		MiningMode:        modes.MiningMode,
		APIMode:           modes.APIMode,
		Domain:            domain,
		WalletPath:        walletConfig.Path,
		ImportWallet:      walletConfig.ImportWallet,
		PrivateKey:        walletConfig.PrivateKey,
		ConfigureFirewall: firewallConfig,
	}
}

type NodeModes struct {
	APIMode    bool
	MeshMode   bool
	BeaconMode bool
	MiningMode bool
}

type PortConfig struct {
	APIPort  int
	MeshPort int
}

func selectNetwork(reader *bufio.Reader) string {
	for {
		fmt.Println("🌍 Select Network:")
		fmt.Println("1. Mainnet (Production - Real TruthChain network)")
		fmt.Println("2. Testnet (Development - Testing environment)")
		fmt.Println("3. Local (Isolated - Your own private network)")
		fmt.Println()
		fmt.Print("Enter your choice (1-3): ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			fmt.Println("✅ Selected: Mainnet")
			fmt.Println("ℹ️  Using mainnet consensus rules (post threshold: 5)")
			return "truthchain-mainnet"
		case "2":
			fmt.Println("✅ Selected: Testnet")
			fmt.Println("ℹ️  Using testnet consensus rules (post threshold: 3)")
			return "truthchain-testnet"
		case "3":
			fmt.Println("✅ Selected: Local")
			fmt.Println("ℹ️  Using local consensus rules (post threshold: 2)")
			return "truthchain-local"
		default:
			fmt.Println("❌ Invalid choice. Please enter 1, 2, or 3.")
			fmt.Println()
		}
	}
}

func selectNodeModes(reader *bufio.Reader) *NodeModes {
	fmt.Println()
	fmt.Println("🔧 Select Node Modes:")
	fmt.Println("Choose which features to enable:")
	fmt.Println()

	modes := &NodeModes{}

	// API Mode (always recommended)
	fmt.Println("📡 API Server (Required for creating posts and checking balances)")
	modes.APIMode = getYesNo(reader, "Enable API server?", true)

	// Mesh Mode
	fmt.Println()
	fmt.Println("🌐 Mesh Network (Connect to other nodes for data sharing)")
	modes.MeshMode = getYesNo(reader, "Enable mesh network?", false)

	// Beacon Mode
	fmt.Println()
	fmt.Println("📡 Beacon Mode (Announce your node to the network for discovery)")
	if modes.MeshMode {
		modes.BeaconMode = getYesNo(reader, "Enable beacon mode?", false)
	} else {
		fmt.Println("⚠️  Beacon mode requires mesh mode. Skipping...")
		modes.BeaconMode = false
	}

	// Mining Mode
	fmt.Println()
	fmt.Println("⛏️  Uptime Mining (Earn characters by keeping your node online)")
	fmt.Println("ℹ️  Requirements: 80% uptime over 24 hours to receive rewards")
	fmt.Println("ℹ️  Rewards: Distributed every 10 minutes when requirements are met")
	modes.MiningMode = getYesNo(reader, "Enable uptime mining?", true)

	return modes
}

func configurePorts(reader *bufio.Reader, modes *NodeModes) *PortConfig {
	fmt.Println()
	fmt.Println("🔌 Port Configuration:")
	fmt.Println()

	ports := &PortConfig{}

	// API Port
	if modes.APIMode {
		ports.APIPort = getPort(reader, "API Server Port", 8080)
	}

	// Mesh Port (handles both mesh communication and chain sync)
	if modes.MeshMode {
		ports.MeshPort = getPort(reader, "Mesh Network Port", 9876)
		fmt.Println("ℹ️  Note: Chain sync is handled through the mesh network on the same port")
	}

	return ports
}

func configureDomain(reader *bufio.Reader) string {
	fmt.Println()
	fmt.Println("🌐 Domain Configuration:")
	fmt.Println("For beacon mode, you need a domain that points to your IP address.")
	fmt.Println("This allows other nodes to discover and connect to your node.")
	fmt.Println()

	for {
		fmt.Print("Enter your domain (e.g., mynode.truth-chain.org): ")
		domain, _ := reader.ReadString('\n')
		domain = strings.TrimSpace(domain)

		if domain == "" {
			fmt.Println("❌ Domain cannot be empty for beacon mode.")
			continue
		}

		if !strings.Contains(domain, ".") {
			fmt.Println("❌ Please enter a valid domain (e.g., mynode.truth-chain.org)")
			continue
		}

		fmt.Printf("✅ Domain set to: %s\n", domain)
		return domain
	}
}

func configureDatabase(reader *bufio.Reader) string {
	fmt.Println()
	fmt.Println("💾 Database Configuration:")
	fmt.Println()

	for {
		fmt.Print("Enter database file path (default: truthchain.db): ")
		dbPath, _ := reader.ReadString('\n')
		dbPath = strings.TrimSpace(dbPath)

		if dbPath == "" {
			dbPath = "truthchain.db"
		}

		fmt.Printf("✅ Database path: %s\n", dbPath)
		return dbPath
	}
}

func configureFirewall(reader *bufio.Reader) bool {
	fmt.Println()
	fmt.Println("🔧 Firewall Configuration:")
	fmt.Println("TruthChain needs to communicate on ports for API and mesh networking.")
	fmt.Println("On Windows, we can automatically configure Windows Firewall rules.")
	fmt.Println()

	if os.Getenv("OS") == "Windows_NT" {
		return getYesNo(reader, "Automatically configure Windows Firewall rules?", true)
	} else {
		fmt.Println("ℹ️  Firewall configuration is only available on Windows")
		return false
	}
}

func configurePostThreshold(reader *bufio.Reader) int {
	fmt.Println()
	fmt.Println("📝 Post Threshold Configuration:")
	fmt.Println("This determines how many posts are needed before creating a new block.")
	fmt.Println("⚠️  IMPORTANT: This affects blockchain consensus. Use default values for network compatibility.")
	fmt.Println()

	// Use fixed values based on network type
	// This will be set based on the network selection, not user input
	return 5 // Default for mainnet
}

func getYesNo(reader *bufio.Reader, question string, defaultYes bool) bool {
	defaultStr := "Y"
	if !defaultYes {
		defaultStr = "N"
	}

	for {
		fmt.Printf("%s (y/n, default: %s): ", question, defaultStr)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response == "" {
			return defaultYes
		}
		if response == "y" || response == "yes" {
			return true
		}
		if response == "n" || response == "no" {
			return false
		}

		fmt.Println("❌ Please enter 'y' for yes or 'n' for no.")
	}
}

func getPort(reader *bufio.Reader, name string, defaultPort int) int {
	for {
		fmt.Printf("Enter %s port (default: %d): ", name, defaultPort)
		portStr, _ := reader.ReadString('\n')
		portStr = strings.TrimSpace(portStr)

		if portStr == "" {
			fmt.Printf("✅ %s port: %d\n", name, defaultPort)
			return defaultPort
		}

		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1024 || port > 65535 {
			fmt.Println("❌ Please enter a valid port number between 1024 and 65535.")
			continue
		}

		fmt.Printf("✅ %s port: %d\n", name, port)
		return port
	}
}

func showFinalConfig(networkID string, modes *NodeModes, ports *PortConfig, domain, dbPath string, postThreshold int, walletConfig *WalletConfig) {
	fmt.Println()
	fmt.Println("📋 Final Configuration:")
	fmt.Println("=======================")
	fmt.Printf("Network: %s\n", networkID)
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Post Threshold: %d\n", postThreshold)
	fmt.Printf("Wallet: %s\n", walletConfig.Path)
	if walletConfig.ImportWallet {
		fmt.Printf("Wallet Type: Imported\n")
	} else {
		fmt.Printf("Wallet Type: New\n")
	}

	// Show firewall configuration
	if os.Getenv("OS") == "Windows_NT" {
		fmt.Printf("Firewall: Auto-configure Windows Firewall\n")
	} else {
		fmt.Printf("Firewall: Manual configuration required\n")
	}

	fmt.Println()
	fmt.Println("Enabled Features:")
	if modes.APIMode {
		fmt.Printf("  ✅ API Server (Port: %d)\n", ports.APIPort)
	}
	if modes.MeshMode {
		fmt.Printf("  ✅ Mesh Network (Port: %d) - handles mesh + chain sync\n", ports.MeshPort)
	}
	if modes.BeaconMode {
		fmt.Printf("  ✅ Beacon Mode (Domain: %s)\n", domain)
	}
	if modes.MiningMode {
		fmt.Println("  ✅ Uptime Mining")
	}
	fmt.Println()
}

func confirmConfiguration(reader *bufio.Reader) bool {
	for {
		fmt.Print("Start TruthChain node with this configuration? (y/n): ")
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			fmt.Println()
			fmt.Println("🚀 Starting TruthChain node...")
			fmt.Println()
			return true
		}
		if response == "n" || response == "no" {
			return false
		}

		fmt.Println("❌ Please enter 'y' for yes or 'n' for no.")
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
	var myWallet *wallet.Wallet
	if config.ImportWallet && config.PrivateKey != "" {
		// Import existing wallet from private key
		myWallet, err = wallet.ImportFromPrivateKey(config.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to import wallet: %w", err)
		}

		// Save imported wallet to disk
		if err := myWallet.SaveWallet(config.WalletPath); err != nil {
			return nil, fmt.Errorf("failed to save imported wallet: %w", err)
		}

		// Generate wallet info file
		if err := generateWalletInfoFile(myWallet, config.WalletPath); err != nil {
			log.Printf("Warning: failed to generate wallet info file: %v", err)
		}

		log.Printf("Wallet imported and saved successfully: %s", myWallet.GetAddress())
	} else {
		// Create new wallet
		myWallet, err = wallet.NewWallet()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize wallet: %w", err)
		}

		// Save wallet to disk
		if err := myWallet.SaveWallet(config.WalletPath); err != nil {
			return nil, fmt.Errorf("failed to save wallet: %w", err)
		}

		// Generate wallet info file
		if err := generateWalletInfoFile(myWallet, config.WalletPath); err != nil {
			log.Printf("Warning: failed to generate wallet info file: %v", err)
		}

		log.Printf("New wallet created and saved: %s", myWallet.GetAddress())
	}

	// Create TrustNetwork (mesh manager is handled inside it)
	trustNet := network.NewTrustNetwork(
		myWallet.GetAddress(),
		myWallet,
		storage,
		nil, // UptimeTracker (set later if mining enabled)
		blockchain,
		config.MeshPort,
		"bootstrap.json", // Bootstrap config file
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
		wallet:       myWallet,
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
	// TrustNetwork will be started in the Start() method, not here
	return nil
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
	n.router.HandleFunc("/wallets/{address}/backup", n.handleWalletBackup).Methods("GET")

	// Network endpoints
	n.router.HandleFunc("/network/stats", n.handleNetworkStats).Methods("GET")
	n.router.HandleFunc("/network/peers", n.handleGetPeers).Methods("GET")
	n.router.HandleFunc("/network/firewall", n.handleFirewallStatus).Methods("GET")

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

func (n *TruthChainNode) handleWalletBackup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	// Only allow backup of the node's own wallet
	if address != n.wallet.GetAddress() {
		http.Error(w, "Unauthorized: can only backup own wallet", http.StatusForbidden)
		return
	}

	// Create wallet backup
	backup, err := n.wallet.ExportBackup()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create wallet backup: %v", err), http.StatusInternalServerError)
		return
	}

	// Set filename for download
	filename := fmt.Sprintf("truthchain-wallet-backup-%s.json", address[:8])
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	json.NewEncoder(w).Encode(backup)
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

func (n *TruthChainNode) handleFirewallStatus(w http.ResponseWriter, r *http.Request) {
	var response map[string]interface{}
	if os.Getenv("OS") == "Windows_NT" {
		status := CheckFirewallStatus(n.config.APIPort, n.config.MeshPort)
		response = status
	} else if isLinux() {
		status := CheckLinuxFirewallStatus(n.config.APIPort, n.config.MeshPort)
		response = status
	} else {
		response = map[string]interface{}{
			"status": "not_supported",
			"note":   "Firewall configuration is only available on Windows and Linux",
		}
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
	fmt.Println("🌐 TruthChain Node")
	fmt.Println("==================")
	fmt.Println()
	fmt.Println("TruthChain is a decentralized blockchain for immutable posts and character-based currency.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  TruthChain.exe                    # Interactive setup (recommended)")
	fmt.Println("  TruthChain.exe --help             # Show this help message")
	fmt.Println()
	fmt.Println("Interactive Setup:")
	fmt.Println("  When you run TruthChain.exe without arguments, you'll be guided through")
	fmt.Println("  a user-friendly setup process that helps you configure:")
	fmt.Println()
	fmt.Println("  • Network selection (Mainnet/Testnet/Local)")
	fmt.Println("  • Node modes (API/Mesh/Beacon/Mining)")
	fmt.Println("  • Port configuration")
	fmt.Println("  • Domain setup (for beacon mode)")
	fmt.Println("  • Database location")
	fmt.Println("  • Post threshold settings")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  📡 API Server     - Create posts and check balances")
	fmt.Println("  🌐 Mesh Network   - Connect to other nodes")
	fmt.Println("  📡 Beacon Mode    - Announce your node to the network")
	fmt.Println("  ⛏️  Uptime Mining  - Earn characters by keeping node online")
	fmt.Println()
	fmt.Println("Quick Start:")
	fmt.Println("  1. Run: TruthChain.exe")
	fmt.Println("  2. Follow the interactive setup")
	fmt.Println("  3. Your node will start automatically")
	fmt.Println()
	fmt.Println("For advanced users, you can still use command-line flags:")
	fmt.Println("  TruthChain.exe -mesh -beacon -mining -domain mynode.truth-chain.org")
}

type WalletConfig struct {
	Path         string
	ImportWallet bool
	PrivateKey   string
}

func configureWallet(reader *bufio.Reader) *WalletConfig {
	fmt.Println("💰 Wallet Configuration")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("Your wallet is your identity on TruthChain.")
	fmt.Println("It contains your private key and public address.")
	fmt.Println()

	config := &WalletConfig{}

	// Ask if user wants to import existing wallet
	fmt.Println("Do you have an existing wallet to import?")
	fmt.Println("1. Create new wallet (recommended for first time)")
	fmt.Println("2. Import existing wallet (if you have private key)")
	fmt.Println()

	for {
		fmt.Print("Enter your choice (1-2): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			fmt.Println("✅ Creating new wallet...")
			config.ImportWallet = false
			config.Path = "wallet.json"
			break
		case "2":
			fmt.Println("✅ Importing existing wallet...")
			config.ImportWallet = true
			config.Path = "wallet.json"
			break
		default:
			fmt.Println("❌ Invalid choice. Please enter 1 or 2.")
			fmt.Println()
			continue
		}
		break
	}

	// If importing, get private key
	if config.ImportWallet {
		fmt.Println()
		fmt.Println("🔑 Private Key Import")
		fmt.Println("Enter your private key (it will be stored securely):")
		fmt.Println("Format: A long string of letters and numbers")
		fmt.Println()

		for {
			fmt.Print("Private Key: ")
			privateKey, _ := reader.ReadString('\n')
			privateKey = strings.TrimSpace(privateKey)

			if privateKey == "" {
				fmt.Println("❌ Private key cannot be empty.")
				continue
			}

			// Basic validation - should be a long hex string
			if len(privateKey) < 64 {
				fmt.Println("❌ Private key seems too short. Please check your key.")
				continue
			}

			config.PrivateKey = privateKey
			fmt.Println("✅ Private key imported successfully!")
			break
		}
	}

	// Show wallet info
	if config.ImportWallet {
		fmt.Println()
		fmt.Println("📋 Wallet Information:")
		fmt.Printf("  Type: Imported wallet\n")
		fmt.Printf("  File: %s\n", config.Path)
		fmt.Println("  ⚠️  Keep your private key safe and secure!")
	} else {
		fmt.Println()
		fmt.Println("📋 Wallet Information:")
		fmt.Printf("  Type: New wallet\n")
		fmt.Printf("  File: %s\n", config.Path)
		fmt.Println("  ✅ A new wallet will be created for you")
	}

	fmt.Println()
	return config
}

// generateWalletInfoFile creates a comprehensive wallet information file
func generateWalletInfoFile(wallet *wallet.Wallet, walletPath string) error {
	info := fmt.Sprintf(`🌐 TruthChain Wallet Information
===============================

⚠️  SECURITY WARNING ⚠️
======================
This file contains sensitive information about your TruthChain wallet.
KEEP THIS FILE IN A SECURE LOCATION AND NEVER SHARE IT WITH ANYONE!

If you lose this information, you will permanently lose access to your wallet
and any characters stored in it. There is no way to recover a lost wallet.

📋 Wallet Details
================

Your Wallet Address: %s
- This is your public address (like a bank account number)
- You can safely share this with others
- Others can send characters to this address
- This address is derived from your public key

Your Private Key: %s
- This is your secret key (like your bank card PIN)
- NEVER share this with anyone
- Keep this in a secure location
- This is required to access your wallet
- If lost, you cannot recover your wallet

Your Public Key: %s
- This is your public key (derived from private key)
- Can be shared safely
- Used to verify your signatures

🔐 Security Best Practices
=========================

1. BACKUP YOUR WALLET
   - Save this file in multiple secure locations
   - Consider using encrypted storage
   - Keep offline backups (not connected to internet)

2. PROTECT YOUR PRIVATE KEY
   - Never share your private key
   - Don't store it in cloud services
   - Don't send it via email or messaging
   - Consider using a hardware wallet for large amounts

3. REGULAR BACKUPS
   - Backup your wallet after any changes
   - Test your backup by importing it on a test system
   - Keep multiple backup copies

4. SECURE ENVIRONMENT
   - Use a clean, secure computer
   - Keep your system updated
   - Use antivirus software
   - Be careful with browser extensions

💡 How to Use Your Wallet
=========================

To import this wallet on another device:
1. Copy the private key from this file
2. Run TruthChain.exe
3. Choose "Import existing wallet"
4. Paste your private key when prompted

To check your balance:
- Use the API: http://localhost:8080/wallets/%s/balance
- Or check the node logs for balance information

To send characters:
- Use the API: POST http://localhost:8080/transfers
- Include recipient address and amount

📅 Wallet Created: %s
📁 Wallet File: %s

🔗 TruthChain Resources
=======================

- Official Documentation: https://github.com/blindxfish/truthchain
- Network Status: Check your node logs
- Support: GitHub Issues

Remember: Your wallet is your responsibility. Keep it secure!
`,
		wallet.GetAddress(),
		wallet.ExportPrivateKeyHex(),
		wallet.ExportPublicKeyHex(),
		wallet.GetAddress(),
		time.Now().Format("2006-01-02 15:04:05"),
		walletPath,
	)

	// Create the info file
	infoPath := "YourWalletInfo.txt"
	if err := os.WriteFile(infoPath, []byte(info), 0600); err != nil {
		return fmt.Errorf("failed to create wallet info file: %w", err)
	}

	return nil
}

func isLinux() bool {
	return strings.Contains(strings.ToLower(runtime.GOOS), "linux")
}
