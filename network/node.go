package network

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/miner"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

// TrustNetwork represents a TruthChain node in the trust-based network
type TrustNetwork struct {
	NodeID        string
	Wallet        *wallet.Wallet
	Storage       *store.BoltDBStorage
	UptimeTracker *miner.UptimeTracker
	Blockchain    *blockchain.Blockchain

	// Network components
	TrustEngine      *TrustEngine
	Topology         *NetworkTopology
	MessageRouter    *MessageRouter
	PeerTable        *PeerTable        // Mesh management
	MeshManager      *MeshManager      // Mesh connection management
	BootstrapManager *BootstrapManager // Bootstrap node management

	// Configuration
	ListenPort    int
	MaxPeers      int
	MinTrustScore float64

	// State
	IsRunning bool
	mu        sync.RWMutex

	// Channels
	MessageChan chan NetworkMessage
	PeerChan    chan PeerEvent
	StopChan    chan struct{}

	// Maps for deduplication
	postSeen     map[string]bool
	transferSeen map[string]bool

	// Add recentMsgHashes for loop prevention
	recentMsgHashes map[string]int64 // hash -> timestamp
}

// NetworkMessage represents a message sent through the network
type NetworkMessage struct {
	Type      MessageType
	Source    string
	Payload   interface{}
	Timestamp int64
	TTL       int
}

// MessageType defines the type of network message
type MessageType int

const (
	MessageTypeGossip MessageType = iota
	MessageTypePost
	MessageTypeTransfer
	MessageTypeBlock
	MessageTypePing
	MessageTypePong
)

// PeerEvent represents peer-related events
type PeerEvent struct {
	Type   PeerEventType
	Peer   *Peer
	Reason string
}

// PeerEventType defines the type of peer event
type PeerEventType int

const (
	PeerEventConnected PeerEventType = iota
	PeerEventDisconnected
	PeerEventTrustUpdated
	PeerEventLatencyUpdated
)

// NewTrustNetwork creates a new trust-based network node
func NewTrustNetwork(
	nodeID string,
	wallet *wallet.Wallet,
	storage *store.BoltDBStorage,
	uptimeTracker *miner.UptimeTracker,
	blockchain *blockchain.Blockchain,
	listenPort int,
	bootstrapConfig string,
) *TrustNetwork {

	network := &TrustNetwork{
		NodeID:        nodeID,
		Wallet:        wallet,
		Storage:       storage,
		UptimeTracker: uptimeTracker,
		Blockchain:    blockchain,

		TrustEngine:      NewTrustEngine(),
		Topology:         NewNetworkTopology(nodeID),
		MessageRouter:    NewMessageRouter(),
		PeerTable:        NewPeerTable(32), // Default max 32 mesh peers
		MeshManager:      nil,              // Will be initialized after network is created
		BootstrapManager: NewBootstrapManager(bootstrapConfig),

		ListenPort:    listenPort,
		MaxPeers:      10,  // Default max 10 direct peers
		MinTrustScore: 0.3, // Minimum trust score for connections

		IsRunning:   false,
		MessageChan: make(chan NetworkMessage, 100),
		PeerChan:    make(chan PeerEvent, 50),
		StopChan:    make(chan struct{}),

		postSeen:     make(map[string]bool),
		transferSeen: make(map[string]bool),

		// Initialize recentMsgHashes
		recentMsgHashes: make(map[string]int64),
	}

	// Set up message router
	network.MessageRouter.Network = network

	return network
}

// Start begins the network node operation
func (tn *TrustNetwork) Start() error {
	tn.mu.Lock()
	defer tn.mu.Unlock()

	if tn.IsRunning {
		return fmt.Errorf("network is already running")
	}

	tn.IsRunning = true

	// Initialize mesh manager
	tn.MeshManager = NewMeshManager(tn)
	if err := tn.MeshManager.Start(); err != nil {
		return fmt.Errorf("failed to start mesh manager: %v", err)
	}

	// Start background goroutines
	go tn.gossipWorker()
	go tn.peerManager()
	go tn.messageProcessor()
	go tn.trustUpdater()
	go tn.meshListener() // Listen for inbound mesh connections
	go tn.cleanupMsgHashCache()

	// Perform bootstrap if we have bootstrap nodes
	if len(tn.BootstrapManager.GetNodes()) > 0 {
		go tn.performBootstrap()
	}

	log.Printf("TrustNetwork started on port %d", tn.ListenPort)

	// Start periodic status logger
	go tn.periodicStatusLogger()

	return nil
}

// Stop gracefully shuts down the network node
func (tn *TrustNetwork) Stop() error {
	tn.mu.Lock()
	defer tn.mu.Unlock()

	if !tn.IsRunning {
		return fmt.Errorf("network is not running")
	}

	tn.IsRunning = false

	// Stop mesh manager
	if tn.MeshManager != nil {
		tn.MeshManager.Stop()
	}

	close(tn.StopChan)

	log.Printf("TrustNetwork stopped")
	return nil
}

// AddPeer adds a new peer to the mesh and topology
func (tn *TrustNetwork) AddPeer(address string) (*MeshPeer, error) {
	// Add to mesh peer table
	tn.PeerTable.AddPeer(address, 1, "", 0.5)
	peer, _ := tn.PeerTable.GetPeer(address)
	peer.IsConnected = true
	peer.LastSeen = time.Now()
	// Add to topology for routing
	p := &Peer{
		Address:      address,
		FirstSeen:    time.Now().Unix(),
		LastSeen:     time.Now().Unix(),
		UptimeScore:  0.0,
		AgeScore:     0.0,
		TrustScore:   0.0,
		Latency:      0,
		HopDistance:  0,
		Path:         []string{address},
		IsConnected:  true,
		ConnectionID: fmt.Sprintf("%s-%d", address, time.Now().Unix()),
	}
	tn.TrustEngine.CalculateTrustScore(p)
	tn.Topology.AddPeer(p)
	// Send peer event
	tn.PeerChan <- PeerEvent{
		Type: PeerEventConnected,
		Peer: p,
	}
	log.Printf("Added peer: %s (Trust: %.2f)", address, p.TrustScore)
	return peer, nil
}

// RemovePeer removes a peer from the mesh and topology
func (tn *TrustNetwork) RemovePeer(address string) error {
	// Remove from mesh peer table
	tn.PeerTable.MarkDisconnected(address)
	// Remove from topology
	tn.Topology.RemovePeer(address)
	log.Printf("Removed peer: %s", address)
	return nil
}

// BroadcastPost broadcasts a post to all mesh peers
func (tn *TrustNetwork) BroadcastPost(post *chain.Post) error {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	if !tn.IsRunning {
		return fmt.Errorf("network is not running")
	}

	// Create network message
	msg := NetworkMessage{
		Type:      MessageTypePost,
		Source:    tn.NodeID,
		Payload:   post,
		Timestamp: time.Now().Unix(),
		TTL:       10, // Allow up to 10 hops
	}

	// Send to message channel for processing
	tn.MessageChan <- msg

	// Also broadcast to mesh peers
	if tn.MeshManager != nil {
		_ = tn.MeshManager.SendNetworkMessage(&msg)
	}

	log.Printf("Broadcasting post: %s", post.Hash)
	return nil
}

// BroadcastTransfer broadcasts a transfer to all mesh peers
func (tn *TrustNetwork) BroadcastTransfer(transfer *chain.Transfer) error {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	if !tn.IsRunning {
		return fmt.Errorf("network is not running")
	}

	// Create network message
	msg := NetworkMessage{
		Type:      MessageTypeTransfer,
		Source:    tn.NodeID,
		Payload:   transfer,
		Timestamp: time.Now().Unix(),
		TTL:       10, // Allow up to 10 hops
	}

	// Send to message channel for processing
	tn.MessageChan <- msg

	// Also broadcast to mesh peers
	if tn.MeshManager != nil {
		_ = tn.MeshManager.SendNetworkMessage(&msg)
	}

	log.Printf("Broadcasting transfer: %s", transfer.Hash)
	return nil
}

// GetNetworkStats returns comprehensive network statistics
func (tn *TrustNetwork) GetNetworkStats() map[string]interface{} {
	tn.mu.RLock()
	defer tn.mu.RUnlock()

	topologyStats := tn.Topology.GetNetworkStats()

	// Add trust engine stats
	trustStats := map[string]interface{}{
		"uptime_weight": tn.TrustEngine.UptimeWeight,
		"age_weight":    tn.TrustEngine.AgeWeight,
		"max_age":       tn.TrustEngine.MaxAge,
	}

	// Add peer details
	peers := make([]map[string]interface{}, 0, len(tn.Topology.Peers))
	for _, peer := range tn.Topology.Peers {
		peerInfo := map[string]interface{}{
			"address":        peer.Address,
			"trust_score":    peer.TrustScore,
			"uptime_score":   peer.UptimeScore,
			"age_score":      peer.AgeScore,
			"latency":        peer.Latency,
			"hop_distance":   peer.HopDistance,
			"is_connected":   peer.IsConnected,
			"last_seen":      peer.LastSeen,
			"connection_age": tn.TrustEngine.GetPeerAge(peer),
		}
		peers = append(peers, peerInfo)
	}

	// Combine all stats
	stats := map[string]interface{}{
		"node_id":         tn.NodeID,
		"is_running":      tn.IsRunning,
		"listen_port":     tn.ListenPort,
		"max_peers":       tn.MaxPeers,
		"min_trust_score": tn.MinTrustScore,
		"peers":           peers,
	}

	// Merge topology and trust stats
	for k, v := range topologyStats {
		stats[k] = v
	}
	for k, v := range trustStats {
		stats[k] = v
	}

	return stats
}

// gossipWorker periodically sends gossip messages to mesh peers
func (tn *TrustNetwork) gossipWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tn.sendGossip()
		case <-tn.StopChan:
			return
		}
	}
}

// sendGossip sends a gossip message to selected mesh peers
func (tn *TrustNetwork) sendGossip() {
	tn.mu.RLock()
	if !tn.IsRunning || len(tn.PeerTable.peers) == 0 {
		tn.mu.RUnlock()
		return
	}
	gossipMsg := tn.PeerTable.CreateGossipMessage()
	msg := NetworkMessage{
		Type:      MessageTypeGossip,
		Source:    tn.NodeID,
		Payload:   gossipMsg,
		Timestamp: time.Now().Unix(),
		TTL:       10,
	}
	tn.mu.RUnlock()
	tn.MessageChan <- msg
}

// peerManager handles peer-related events
func (tn *TrustNetwork) peerManager() {
	for {
		select {
		case event := <-tn.PeerChan:
			tn.handlePeerEvent(event)
		case <-tn.StopChan:
			return
		}
	}
}

// handlePeerEvent processes peer events
func (tn *TrustNetwork) handlePeerEvent(event PeerEvent) {
	switch event.Type {
	case PeerEventConnected:
		log.Printf("\033[32m🌱 New node connected: %s (Trust: %.2f)\033[0m", event.Peer.Address, event.Peer.TrustScore)
	case PeerEventDisconnected:
		log.Printf("Peer disconnected: %s - %s", event.Peer.Address, event.Reason)
	case PeerEventTrustUpdated:
		log.Printf("Peer trust updated: %s (Trust: %.2f)", event.Peer.Address, event.Peer.TrustScore)
	case PeerEventLatencyUpdated:
		log.Printf("Peer latency updated: %s (%dms)", event.Peer.Address, event.Peer.Latency)
	}
}

// messageProcessor handles incoming network messages
func (tn *TrustNetwork) messageProcessor() {
	for {
		select {
		case msg := <-tn.MessageChan:
			tn.handleMessage(msg)
		case <-tn.StopChan:
			return
		}
	}
}

// handleMessage processes incoming network messages
func (tn *TrustNetwork) handleMessage(msg NetworkMessage) {
	switch msg.Type {
	case MessageTypeGossip:
		tn.handleGossipMessage(msg)
	case MessageTypePost:
		tn.handlePostMessage(msg)
	case MessageTypeTransfer:
		tn.handleTransferMessage(msg)
	case MessageTypePing:
		tn.handlePingMessage(msg)
	case MessageTypePong:
		tn.handlePongMessage(msg)
	default:
		log.Printf("Unknown message type: %d", msg.Type)
	}
}

// handleGossipMessage processes mesh gossip messages
func (tn *TrustNetwork) handleGossipMessage(msg NetworkMessage) {
	gossipMsg, ok := msg.Payload.([]*MeshPeer)
	if !ok {
		log.Printf("Invalid mesh gossip message payload")
		return
	}
	tn.PeerTable.ProcessGossipMessage(msg.Source, gossipMsg)
	log.Printf("Processed mesh gossip message from %s", msg.Source)
}

// handlePostMessage processes post messages
func (tn *TrustNetwork) handlePostMessage(msg NetworkMessage) {
	post, ok := msg.Payload.(*chain.Post)
	if !ok {
		log.Printf("Invalid post message payload")
		return
	}

	// Check TTL
	if msg.TTL <= 0 {
		return // Drop message
	}

	// Check hash in recentMsgHashes
	hash := post.Hash
	if tn.recentMsgHashes == nil {
		tn.recentMsgHashes = make(map[string]int64)
	}
	tn.mu.Lock()
	if _, seen := tn.recentMsgHashes[hash]; seen {
		tn.mu.Unlock()
		return // Already seen, drop
	}
	// Add to cache
	tn.recentMsgHashes[hash] = time.Now().Unix()
	tn.mu.Unlock()

	// Validate post (signature, etc)
	if err := post.ValidatePost(); err != nil {
		log.Printf("Invalid post received: %v", err)
		return
	}
	// Add to pending posts (if not present)
	if err := tn.Storage.SavePendingPost(*post); err != nil {
		log.Printf("Failed to add post to pending: %v", err)
		return
	}
	// Gossip to selected peers
	if msg.TTL > 1 {
		msg.TTL--
		tn.gossipToPeers(&msg, msg.Source)
	}

	log.Printf("Received post from %s: %s", msg.Source, post.Hash)
}

// handleTransferMessage processes transfer messages
func (tn *TrustNetwork) handleTransferMessage(msg NetworkMessage) {
	transfer, ok := msg.Payload.(*chain.Transfer)
	if !ok {
		log.Printf("Invalid transfer message payload")
		return
	}

	if tn.transferSeen == nil {
		tn.transferSeen = make(map[string]bool)
	}
	if tn.transferSeen[transfer.Hash] {
		return // Already seen
	}
	tn.transferSeen[transfer.Hash] = true
	// Validate transfer
	if err := transfer.Validate(); err != nil {
		log.Printf("Invalid transfer received: %v", err)
		return
	}
	// Add to transfer pool (if not present)
	if err := tn.Blockchain.AddTransfer(*transfer); err != nil {
		log.Printf("Failed to add transfer to pool: %v", err)
		return
	}
	// Gossip to selected peers
	if msg.TTL > 1 {
		msg.TTL--
		tn.gossipToPeers(&msg, msg.Source)
	}

	log.Printf("Received transfer from %s: %s", msg.Source, transfer.Hash)
}

// handlePingMessage processes ping messages
func (tn *TrustNetwork) handlePingMessage(msg NetworkMessage) {
	// Respond with pong (implementation will be added)
	log.Printf("Received ping from %s", msg.Source)
}

// handlePongMessage processes pong messages
func (tn *TrustNetwork) handlePongMessage(msg NetworkMessage) {
	// Update peer latency (implementation will be added)
	log.Printf("Received pong from %s", msg.Source)
}

// trustUpdater periodically updates trust scores
func (tn *TrustNetwork) trustUpdater() {
	ticker := time.NewTicker(5 * time.Minute) // Update every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tn.updateTrustScores()
		case <-tn.StopChan:
			return
		}
	}
}

// updateTrustScores updates trust scores for all peers
func (tn *TrustNetwork) updateTrustScores() {
	tn.mu.Lock()
	defer tn.mu.Unlock()

	for _, peer := range tn.Topology.Peers {
		// Update trust score
		oldTrust := peer.TrustScore
		tn.TrustEngine.CalculateTrustScore(peer)

		// Send event if trust changed significantly
		if abs(peer.TrustScore-oldTrust) > 0.1 {
			tn.PeerChan <- PeerEvent{
				Type: PeerEventTrustUpdated,
				Peer: peer,
			}
		}
	}
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// meshListener listens for inbound mesh connections
func (tn *TrustNetwork) meshListener() {
	// Listen on the configured port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", tn.ListenPort))
	if err != nil {
		log.Printf("Failed to start mesh listener on port %d: %v", tn.ListenPort, err)
		return
	}
	defer listener.Close()

	log.Printf("Mesh listener started on port %d", tn.ListenPort)

	for {
		select {
		case <-tn.StopChan:
			return
		default:
			// Accept connections with timeout
			listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout, try again
				}
				log.Printf("Error accepting connection: %v", err)
				continue
			}

			// Handle the connection
			remoteAddr := conn.RemoteAddr().String()
			go tn.MeshManager.AcceptInboundConnection(conn, remoteAddr)
		}
	}
}

// performBootstrap attempts to connect to bootstrap nodes
func (tn *TrustNetwork) performBootstrap() {
	// Wait a bit for the network to start up
	time.Sleep(2 * time.Second)

	log.Printf("Starting bootstrap process...")

	// Attempt to bootstrap with up to 5 nodes
	if err := tn.BootstrapManager.Bootstrap(tn, 5); err != nil {
		log.Printf("Bootstrap failed: %v", err)
	} else {
		log.Printf("Bootstrap completed successfully")
	}
}

// Add cleanupMsgHashCache method to TrustNetwork
func (tn *TrustNetwork) cleanupMsgHashCache() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now().Unix()
			tn.mu.Lock()
			for hash, ts := range tn.recentMsgHashes {
				if now-ts > 3600 { // 1 hour
					delete(tn.recentMsgHashes, hash)
				}
			}
			tn.mu.Unlock()
		case <-tn.StopChan:
			return
		}
	}
}

// Add gossipToPeers method to TrustNetwork
func (tn *TrustNetwork) gossipToPeers(msg *NetworkMessage, excludePeer string) {
	if tn.MeshManager == nil {
		return
	}

	// Get connected peers
	peers := tn.PeerTable.GetConnectedPeers()
	if len(peers) == 0 {
		return
	}

	// Select subset for forwarding (3-5 peers, diverse selection)
	targetCount := 3
	if len(peers) < targetCount {
		targetCount = len(peers)
	}

	// Use peer selection logic for diverse forwarding
	selectedPeers := tn.PeerTable.SelectPeers(targetCount)

	// Forward to selected peers (excluding source)
	for _, peer := range selectedPeers {
		if peer.Address != excludePeer {
			// TODO: Implement peer-specific forwarding
			// For now, use the mesh manager's broadcast
			log.Printf("Gossiping to peer: %s", peer.Address)
		}
	}

	// Use mesh manager to send to selected peers
	_ = tn.MeshManager.SendNetworkMessage(msg)
}

// Add periodicStatusLogger method
func (tn *TrustNetwork) periodicStatusLogger() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Gather blockchain info
			if tn.Blockchain == nil {
				continue
			}
			info, err := tn.Blockchain.GetBlockchainInfo()
			if err != nil {
				log.Printf("[Status] Failed to get blockchain info: %v", err)
				continue
			}
			// Gather network stats
			netStats := tn.GetNetworkStats()
			peerCount := 0
			if peers, ok := netStats["peers"].([]map[string]interface{}); ok {
				peerCount = len(peers)
			}
			// Prepare peer summary
			nearest := "-"
			trusted := "-"
			furthest := "-"
			if peers, ok := netStats["peers"].([]map[string]interface{}); ok && len(peers) > 0 {
				// Sort by hop_distance, trust_score
				minHop, maxHop := 9999, -1
				maxTrust := -1.0
				for _, p := range peers {
					hop, _ := p["hop_distance"].(int)
					trust, _ := p["trust_score"].(float64)
					addr, _ := p["address"].(string)
					if hop < minHop {
						minHop = hop
						nearest = addr
					}
					if hop > maxHop {
						maxHop = hop
						furthest = addr
					}
					if trust > maxTrust {
						maxTrust = trust
						trusted = addr
					}
				}
			}
			log.Printf("[Status] Chain: %d blocks, %d posts, %d chars | Peers: %d [nearest: %s, trusted: %s, furthest: %s] | Minted: %d | Pending posts: %d",
				info["chain_length"], info["total_post_count"], info["total_character_count"],
				peerCount, nearest, trusted, furthest, info["total_character_supply"], info["pending_post_count"])
		case <-tn.StopChan:
			return
		}
	}
}
