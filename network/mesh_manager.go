package network

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// MeshConnection represents an active connection to a mesh peer
type MeshConnection struct {
	Address     string
	Conn        net.Conn
	IsConnected bool
	LastPing    time.Time
	Latency     time.Duration
	TrustScore  float64
	HopDistance int
	mu          sync.RWMutex
}

// MeshManager handles mesh peer connections and selection
type MeshManager struct {
	network     *TrustNetwork
	connections map[string]*MeshConnection
	targetCount int // Target number of mesh connections
	mu          sync.RWMutex

	// Connection management
	connChan chan ConnectionEvent
	stopChan chan struct{}

	// Configuration
	selectionInterval time.Duration
	pingInterval      time.Duration
	connectionTimeout time.Duration
}

// ConnectionEvent represents connection-related events
type ConnectionEvent struct {
	Type    ConnectionEventType
	Address string
	Conn    *MeshConnection
	Error   error
	Latency time.Duration
}

// ConnectionEventType defines the type of connection event
type ConnectionEventType int

const (
	ConnectionEventConnected ConnectionEventType = iota
	ConnectionEventDisconnected
	ConnectionEventFailed
	ConnectionEventLatencyUpdated
	ConnectionEventTrustUpdated
)

// NewMeshManager creates a new mesh connection manager
func NewMeshManager(network *TrustNetwork) *MeshManager {
	return &MeshManager{
		network:           network,
		connections:       make(map[string]*MeshConnection),
		targetCount:       3, // Default: maintain 3 mesh connections
		connChan:          make(chan ConnectionEvent, 100),
		stopChan:          make(chan struct{}),
		selectionInterval: 30 * time.Second, // Re-select peers every 30 seconds
		pingInterval:      10 * time.Second, // Ping peers every 10 seconds
		connectionTimeout: 5 * time.Second,  // Connection timeout
	}
}

// Start begins the mesh connection management
func (mm *MeshManager) Start() error {
	log.Printf("Starting mesh manager with target %d connections", mm.targetCount)

	// Start background goroutines
	go mm.connectionSelector()
	go mm.connectionManager()
	go mm.pingManager()

	return nil
}

// Stop gracefully shuts down the mesh manager
func (mm *MeshManager) Stop() error {
	log.Printf("Stopping mesh manager")

	close(mm.stopChan)

	// Close all connections
	mm.mu.Lock()
	for _, conn := range mm.connections {
		if conn.Conn != nil {
			conn.Conn.Close()
		}
	}
	mm.mu.Unlock()

	return nil
}

// connectionSelector periodically selects and maintains mesh connections
func (mm *MeshManager) connectionSelector() {
	ticker := time.NewTicker(mm.selectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mm.selectAndMaintainConnections()
		case <-mm.stopChan:
			return
		}
	}
}

// selectAndMaintainConnections selects peers and maintains connections
func (mm *MeshManager) selectAndMaintainConnections() {
	// Get current mesh peer selection
	selectedPeers := mm.network.PeerTable.SelectPeers(mm.targetCount)

	// Get currently connected peers
	mm.mu.RLock()
	currentConnections := make(map[string]bool)
	for addr := range mm.connections {
		currentConnections[addr] = true
	}
	mm.mu.RUnlock()

	// Determine which connections to maintain and which to drop
	selectedAddresses := make(map[string]bool)
	for _, peer := range selectedPeers {
		selectedAddresses[peer.Address] = true

		// If not currently connected, establish connection
		if !currentConnections[peer.Address] {
			go mm.establishConnection(peer.Address)
		}
	}

	// Drop connections that are no longer selected
	for addr := range currentConnections {
		if !selectedAddresses[addr] {
			log.Printf("Dropping mesh connection to %s (no longer selected)", addr)
			mm.dropConnection(addr)
		}
	}
}

// establishConnection attempts to establish a connection to a peer
func (mm *MeshManager) establishConnection(address string) {
	log.Printf("Attempting to connect to mesh peer: %s", address)

	// Check if already connected
	mm.mu.RLock()
	if _, exists := mm.connections[address]; exists {
		mm.mu.RUnlock()
		return
	}
	mm.mu.RUnlock()

	// Establish TCP connection
	conn, err := net.DialTimeout("tcp", address, mm.connectionTimeout)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", address, err)
		mm.connChan <- ConnectionEvent{
			Type:    ConnectionEventFailed,
			Address: address,
			Error:   err,
		}
		return
	}

	// Create mesh connection
	meshConn := &MeshConnection{
		Address:     address,
		Conn:        conn,
		IsConnected: true,
		LastPing:    time.Now(),
	}

	// Add to connections
	mm.mu.Lock()
	mm.connections[address] = meshConn
	mm.mu.Unlock()

	// Update peer table
	mm.network.PeerTable.MarkConnected(address)

	// Send connection event
	mm.connChan <- ConnectionEvent{
		Type:    ConnectionEventConnected,
		Address: address,
		Conn:    meshConn,
	}

	log.Printf("Successfully connected to mesh peer: %s", address)

	// Start connection handler
	go mm.handleConnection(meshConn)
}

// dropConnection drops a connection to a peer
func (mm *MeshManager) dropConnection(address string) {
	mm.mu.Lock()
	conn, exists := mm.connections[address]
	if exists {
		if conn.Conn != nil {
			conn.Conn.Close()
		}
		delete(mm.connections, address)
	}
	mm.mu.Unlock()

	if exists {
		// Update peer table
		mm.network.PeerTable.MarkDisconnected(address)

		// Send disconnection event
		mm.connChan <- ConnectionEvent{
			Type:    ConnectionEventDisconnected,
			Address: address,
			Conn:    conn,
		}
	}
}

// handleConnection handles an active connection
func (mm *MeshManager) handleConnection(meshConn *MeshConnection) {
	defer func() {
		mm.dropConnection(meshConn.Address)
	}()

	// Set up connection for reading
	buffer := make([]byte, 4096)

	for {
		// Set read deadline
		meshConn.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read data
		n, err := meshConn.Conn.Read(buffer)
		if err != nil {
			log.Printf("Connection read error from %s: %v", meshConn.Address, err)
			return
		}

		if n > 0 {
			// Process received data
			data := buffer[:n]
			mm.processReceivedData(meshConn.Address, data)
		}
	}
}

// processReceivedData processes data received from a mesh peer
func (mm *MeshManager) processReceivedData(address string, data []byte) {
	// TODO: Implement message parsing and routing
	// For now, just log the received data
	log.Printf("Received %d bytes from mesh peer %s", len(data), address)

	// Update last ping time
	mm.mu.Lock()
	if conn, exists := mm.connections[address]; exists {
		conn.LastPing = time.Now()
	}
	mm.mu.Unlock()
}

// connectionManager handles connection events
func (mm *MeshManager) connectionManager() {
	for {
		select {
		case event := <-mm.connChan:
			mm.handleConnectionEvent(event)
		case <-mm.stopChan:
			return
		}
	}
}

// handleConnectionEvent processes connection events
func (mm *MeshManager) handleConnectionEvent(event ConnectionEvent) {
	switch event.Type {
	case ConnectionEventConnected:
		log.Printf("Mesh peer connected: %s", event.Address)
	case ConnectionEventDisconnected:
		log.Printf("Mesh peer disconnected: %s", event.Address)
	case ConnectionEventFailed:
		log.Printf("Failed to connect to mesh peer: %s - %v", event.Address, event.Error)
	case ConnectionEventLatencyUpdated:
		log.Printf("Mesh peer latency updated: %s (%v)", event.Address, event.Latency)
	case ConnectionEventTrustUpdated:
		log.Printf("Mesh peer trust updated: %s (%.2f)", event.Address, event.Conn.TrustScore)
	}
}

// pingManager periodically pings mesh peers
func (mm *MeshManager) pingManager() {
	ticker := time.NewTicker(mm.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mm.pingAllPeers()
		case <-mm.stopChan:
			return
		}
	}
}

// pingAllPeers pings all connected mesh peers
func (mm *MeshManager) pingAllPeers() {
	mm.mu.RLock()
	peers := make([]*MeshConnection, 0, len(mm.connections))
	for _, conn := range mm.connections {
		peers = append(peers, conn)
	}
	mm.mu.RUnlock()

	for _, peer := range peers {
		go mm.pingPeer(peer)
	}
}

// pingPeer pings a specific peer
func (mm *MeshManager) pingPeer(peer *MeshConnection) {
	start := time.Now()

	// Send ping message
	pingMsg := fmt.Sprintf("PING:%d", start.UnixNano())
	_, err := peer.Conn.Write([]byte(pingMsg))
	if err != nil {
		log.Printf("Failed to ping %s: %v", peer.Address, err)
		return
	}

	// Update latency (will be refined when we implement proper ping/pong)
	latency := time.Since(start)

	peer.mu.Lock()
	peer.Latency = latency
	peer.LastPing = time.Now()
	peer.mu.Unlock()

	// Update peer table
	mm.network.PeerTable.UpdatePeerLatency(peer.Address, latency.Milliseconds())

	// Send latency update event
	mm.connChan <- ConnectionEvent{
		Type:    ConnectionEventLatencyUpdated,
		Address: peer.Address,
		Conn:    peer,
		Latency: latency,
	}
}

// SendToMesh sends a message to all mesh peers
func (mm *MeshManager) SendToMesh(message []byte) error {
	mm.mu.RLock()
	peers := make([]*MeshConnection, 0, len(mm.connections))
	for _, conn := range mm.connections {
		peers = append(peers, conn)
	}
	mm.mu.RUnlock()

	var lastError error
	for _, peer := range peers {
		_, err := peer.Conn.Write(message)
		if err != nil {
			log.Printf("Failed to send to mesh peer %s: %v", peer.Address, err)
			lastError = err
		}
	}

	return lastError
}

// GetMeshStats returns statistics about mesh connections
func (mm *MeshManager) GetMeshStats() map[string]interface{} {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	connectedCount := 0
	totalLatency := time.Duration(0)

	for _, conn := range mm.connections {
		if conn.IsConnected {
			connectedCount++
			totalLatency += conn.Latency
		}
	}

	avgLatency := time.Duration(0)
	if connectedCount > 0 {
		avgLatency = totalLatency / time.Duration(connectedCount)
	}

	return map[string]interface{}{
		"target_connections":   mm.targetCount,
		"connected_peers":      connectedCount,
		"average_latency_ms":   avgLatency.Milliseconds(),
		"selection_interval_s": mm.selectionInterval.Seconds(),
		"ping_interval_s":      mm.pingInterval.Seconds(),
	}
}

// AcceptInboundConnection accepts an inbound connection and adds to mesh
func (mm *MeshManager) AcceptInboundConnection(conn net.Conn, remoteAddr string) {
	log.Printf("Accepting inbound connection from: %s", remoteAddr)

	// Create mesh connection
	meshConn := &MeshConnection{
		Address:     remoteAddr,
		Conn:        conn,
		IsConnected: true,
		LastPing:    time.Now(),
	}

	// Add to connections
	mm.mu.Lock()
	mm.connections[remoteAddr] = meshConn
	mm.mu.Unlock()

	// Add to peer table
	mm.network.PeerTable.AddPeer(remoteAddr, 1, "", 0.5)
	mm.network.PeerTable.MarkConnected(remoteAddr)

	// Send connection event
	mm.connChan <- ConnectionEvent{
		Type:    ConnectionEventConnected,
		Address: remoteAddr,
		Conn:    meshConn,
	}

	// Start connection handler
	go mm.handleConnection(meshConn)
}
