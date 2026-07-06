package network

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// Mesh transport robustness limits (Track C hardening). These bound blocking
// I/O, per-peer buffering, connection count, and message rate so a hostile or
// broken peer cannot crash, hang, or exhaust the node.
const (
	// maxMeshFrameBytes caps how much unparsed data is buffered per connection.
	// A single message larger than this (or a peer that never completes one)
	// drops the connection instead of consuming unbounded memory.
	maxMeshFrameBytes = 2 * 1024 * 1024 // 2 MB (> MaxBlockSize 1 MB)
	// maxMeshConnections caps concurrent mesh connections.
	maxMeshConnections = 64
	// handshakeMaxBytes bounds the wallet-address handshake line.
	handshakeMaxBytes = 256
	// meshReadDeadline / meshWriteDeadline bound blocking reads/writes so a
	// stalled or slowloris peer cannot pin a goroutine or a broadcast forever.
	meshReadDeadline  = 120 * time.Second
	meshWriteDeadline = 15 * time.Second
	handshakeDeadline = 15 * time.Second
	// Receive-path rate limit: max messages processed per window per peer.
	meshRateWindow  = 10 * time.Second
	meshRateMaxMsgs = 500
)

// writeToConn writes data with a bounded write deadline so a stalled peer cannot
// block the caller indefinitely.
func writeToConn(conn net.Conn, data []byte) error {
	_ = conn.SetWriteDeadline(time.Now().Add(meshWriteDeadline))
	_, err := conn.Write(data)
	return err
}

// readHandshakeLine reads a single newline-terminated line (the peer's wallet
// address) with a deadline and a hard size cap. It reads one byte at a time so
// it does not over-read and silently discard pipelined post-handshake bytes.
func readHandshakeLine(conn net.Conn) (string, error) {
	_ = conn.SetReadDeadline(time.Now().Add(handshakeDeadline))
	defer conn.SetReadDeadline(time.Time{})

	buf := make([]byte, 0, 64)
	one := make([]byte, 1)
	for len(buf) < handshakeMaxBytes {
		n, err := conn.Read(one)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		if one[0] == '\n' {
			return strings.TrimSpace(string(buf)), nil
		}
		buf = append(buf, one[0])
	}
	return "", fmt.Errorf("handshake line exceeded %d bytes", handshakeMaxBytes)
}

// jsonObjectEnd returns the index just past the end of the JSON object/array
// that begins at start, or -1 if the buffer does not yet contain the complete
// object (so the caller should retain the tail and wait for more data).
func jsonObjectEnd(data []byte, start int) int {
	braceCount, bracketCount := 0, 0
	inString, escapeNext := false, false
	for i := start; i < len(data); i++ {
		c := data[i]
		if escapeNext {
			escapeNext = false
			continue
		}
		if c == '\\' {
			escapeNext = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 && bracketCount == 0 {
				return i + 1
			}
		case '[':
			bracketCount++
		case ']':
			bracketCount--
			if braceCount == 0 && bracketCount == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// ConnectionHistory tracks connection history for an IP address
type ConnectionHistory struct {
	IP              string // IP address (e.g., "192.168.1.100")
	LastAddress     string // Last known address (IP:port)
	FirstSeen       time.Time
	LastSeen        time.Time
	ConnectionCount int // Number of connections
	DisconnectCount int // Number of disconnections
	LastDisconnect  time.Time
	TrustPenalty    float64 // Trust penalty for frequent disconnects
	mu              sync.RWMutex
}

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

	// Connection history tracking
	connectionHistory map[string]*ConnectionHistory // IP -> ConnectionHistory
	historyMu         sync.RWMutex

	// Configuration
	selectionInterval time.Duration
	pingInterval      time.Duration
	connectionTimeout time.Duration

	// Timeout and retry configuration
	connectionRetryTimeout time.Duration
	maxRetryAttempts       int
	trustPenaltyDecay      time.Duration
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
	ConnectionEventReconnected
)

// NewMeshManager creates a new mesh connection manager
func NewMeshManager(network *TrustNetwork) *MeshManager {
	return &MeshManager{
		network:           network,
		connections:       make(map[string]*MeshConnection),
		targetCount:       3, // Default: maintain 3 mesh connections
		connChan:          make(chan ConnectionEvent, 100),
		stopChan:          make(chan struct{}),
		connectionHistory: make(map[string]*ConnectionHistory),
		selectionInterval: 30 * time.Second, // Re-select peers every 30 seconds
		pingInterval:      10 * time.Second, // Ping peers every 10 seconds
		connectionTimeout: 5 * time.Second,  // Connection timeout

		// New timeout and retry configuration
		connectionRetryTimeout: 30 * time.Second, // Wait before retrying failed connections
		maxRetryAttempts:       3,                // Maximum retry attempts for failed connections
		trustPenaltyDecay:      24 * time.Hour,   // Trust penalty decays over 24 hours
	}
}

// extractIP extracts the IP address from a full address (IP:port)
func (mm *MeshManager) extractIP(address string) string {
	if strings.Contains(address, ":") {
		return strings.Split(address, ":")[0]
	}
	return address
}

// getConnectionHistory gets or creates connection history for an IP
func (mm *MeshManager) getConnectionHistory(ip string) *ConnectionHistory {
	mm.historyMu.Lock()
	defer mm.historyMu.Unlock()

	history, exists := mm.connectionHistory[ip]
	if !exists {
		history = &ConnectionHistory{
			IP:              ip,
			FirstSeen:       time.Now(),
			LastSeen:        time.Now(),
			ConnectionCount: 0,
			DisconnectCount: 0,
			TrustPenalty:    0.0,
		}
		mm.connectionHistory[ip] = history
	}
	return history
}

// updateConnectionHistory updates connection history when a peer connects
func (mm *MeshManager) updateConnectionHistory(address string) {
	ip := mm.extractIP(address)
	history := mm.getConnectionHistory(ip)

	history.mu.Lock()
	defer history.mu.Unlock()

	// Check if this is a reconnection (same IP, different port)
	if history.LastAddress != "" && history.LastAddress != address {
		log.Printf("Peer reconnected: %s -> %s (IP: %s)", history.LastAddress, address, ip)
		// Remove old connection if it exists
		mm.dropConnection(history.LastAddress)
	}

	history.LastAddress = address
	history.LastSeen = time.Now()
	history.ConnectionCount++
}

// updateDisconnectHistory updates connection history when a peer disconnects
func (mm *MeshManager) updateDisconnectHistory(address string) {
	ip := mm.extractIP(address)
	history := mm.getConnectionHistory(ip)

	history.mu.Lock()
	defer history.mu.Unlock()

	history.DisconnectCount++
	history.LastDisconnect = time.Now()

	// Calculate trust penalty based on disconnect frequency
	timeSinceFirst := time.Since(history.FirstSeen)
	disconnectRate := float64(history.DisconnectCount) / timeSinceFirst.Hours()

	// Apply trust penalty for high disconnect rates
	if disconnectRate > 1.0 { // More than 1 disconnect per hour
		history.TrustPenalty = 0.2 // 20% trust penalty
	} else if disconnectRate > 0.5 { // More than 1 disconnect per 2 hours
		history.TrustPenalty = 0.1 // 10% trust penalty
	}

	log.Printf("Peer disconnected: %s (IP: %s, disconnect rate: %.2f/hour, trust penalty: %.2f)",
		address, ip, disconnectRate, history.TrustPenalty)
}

// getTrustPenalty returns the current trust penalty for an IP
func (mm *MeshManager) getTrustPenalty(ip string) float64 {
	history := mm.getConnectionHistory(ip)

	history.mu.RLock()
	defer history.mu.RUnlock()

	// Decay trust penalty over time
	timeSinceDisconnect := time.Since(history.LastDisconnect)
	if timeSinceDisconnect > mm.trustPenaltyDecay {
		return 0.0 // Penalty has decayed
	}

	// Linear decay
	decayFactor := 1.0 - (timeSinceDisconnect.Hours() / mm.trustPenaltyDecay.Hours())
	if decayFactor < 0 {
		decayFactor = 0
	}

	return history.TrustPenalty * decayFactor
}

// isConnectionBlocked checks if a connection should be blocked due to recent failures
func (mm *MeshManager) isConnectionBlocked(address string) bool {
	ip := mm.extractIP(address)
	history := mm.getConnectionHistory(ip)

	history.mu.RLock()
	defer history.mu.RUnlock()

	// Block if too many recent disconnections
	timeSinceDisconnect := time.Since(history.LastDisconnect)
	if timeSinceDisconnect < mm.connectionRetryTimeout && history.DisconnectCount > mm.maxRetryAttempts {
		log.Printf("Connection blocked to %s (IP: %s): %d disconnections in last %v",
			address, ip, history.DisconnectCount, mm.connectionRetryTimeout)
		return true
	}

	return false
}

// Start begins the mesh connection management
func (mm *MeshManager) Start() error {
	log.Printf("Starting mesh manager with target %d connections", mm.targetCount)

	// Start background goroutines
	go mm.connectionSelector()
	go mm.connectionManager()
	go mm.pingManager()
	go mm.connectionHistoryCleaner()

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

// connectionHistoryCleaner periodically cleans up old connection history
func (mm *MeshManager) connectionHistoryCleaner() {
	ticker := time.NewTicker(1 * time.Hour) // Clean every hour
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mm.cleanupConnectionHistory()
		case <-mm.stopChan:
			return
		}
	}
}

// cleanupConnectionHistory removes old connection history entries
func (mm *MeshManager) cleanupConnectionHistory() {
	mm.historyMu.Lock()
	defer mm.historyMu.Unlock()

	cutoff := time.Now().Add(-7 * 24 * time.Hour) // Keep 7 days of history
	removed := 0

	for ip, history := range mm.connectionHistory {
		if history.LastSeen.Before(cutoff) {
			delete(mm.connectionHistory, ip)
			removed++
		}
	}

	if removed > 0 {
		log.Printf("Cleaned up %d old connection history entries", removed)
	}
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

		// If not currently connected and not blocked, establish connection
		if !currentConnections[peer.Address] && !mm.isConnectionBlocked(peer.Address) {
			log.Printf("Selected peer for connection: %s (trust: %.2f)", peer.Address, peer.TrustScore)
			go mm.establishConnection(peer.Address)
		} else if mm.isConnectionBlocked(peer.Address) {
			log.Printf("Peer blocked from connection: %s (recent failures)", peer.Address)
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

	// Check if connection is blocked
	if mm.isConnectionBlocked(address) {
		log.Printf("Connection blocked to %s due to recent failures", address)
		return
	}

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

	// --- Wallet handshake ---
	// Send our wallet address
	ourWallet := mm.network.Wallet.GetAddress()
	if err = writeToConn(conn, []byte(ourWallet+"\n")); err != nil {
		log.Printf("Failed to send handshake to %s: %v", address, err)
		conn.Close()
		return
	}
	// Read remote wallet address (bounded + deadlined against slowloris)
	remoteWallet, err := readHandshakeLine(conn)
	if err != nil {
		log.Printf("Failed to read handshake from %s: %v", address, err)
		conn.Close()
		return
	}
	if remoteWallet == ourWallet {
		// Self-connection, close quietly
		conn.Close()
		return
	}
	// --- End handshake ---

	// Update connection history
	mm.updateConnectionHistory(address)

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

	// Add to topology as a direct peer
	peer := &Peer{
		Address:     address,
		TrustScore:  0.5,
		UptimeScore: 0.5,
		AgeScore:    0.5,
		Latency:     0,
		HopDistance: 0,
		IsConnected: true,
		LastSeen:    time.Now().Unix(),
	}
	mm.network.Topology.AddPeer(peer)

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
		// Update connection history
		mm.updateDisconnectHistory(address)

		// Update peer table
		mm.network.PeerTable.MarkDisconnected(address)

		// Remove from topology
		mm.network.Topology.RemovePeer(address)

		// Send disconnection event
		mm.connChan <- ConnectionEvent{
			Type:    ConnectionEventDisconnected,
			Address: address,
			Conn:    conn,
		}
	}
}

// handleConnection handles an active connection. It reassembles the byte stream
// across reads (messages may span multiple TCP segments or exceed one read),
// bounds the per-connection buffer, and rate-limits the receive path.
func (mm *MeshManager) handleConnection(meshConn *MeshConnection) {
	defer func() {
		mm.dropConnection(meshConn.Address)
	}()

	readBuf := make([]byte, 32*1024)
	var acc []byte

	windowStart := time.Now()
	msgCount := 0

	for {
		_ = meshConn.Conn.SetReadDeadline(time.Now().Add(meshReadDeadline))

		n, err := meshConn.Conn.Read(readBuf)
		if err != nil {
			log.Printf("Connection read error from %s: %v", meshConn.Address, err)
			return
		}
		if n == 0 {
			continue
		}

		acc = append(acc, readBuf[:n]...)
		if len(acc) > maxMeshFrameBytes {
			log.Printf("Peer %s exceeded max buffered frame (%d bytes) - dropping", meshConn.Address, len(acc))
			return
		}

		consumed, processed := mm.processStream(meshConn.Address, acc)
		if consumed > 0 {
			// Retain any trailing incomplete message for the next read.
			acc = append(acc[:0], acc[consumed:]...)
		}

		// Receive-path rate limit: drop peers that flood us with messages.
		if time.Since(windowStart) >= meshRateWindow {
			windowStart = time.Now()
			msgCount = 0
		}
		msgCount += processed
		if msgCount > meshRateMaxMsgs {
			log.Printf("Peer %s exceeded receive rate limit (%d msgs / %s) - dropping", meshConn.Address, msgCount, meshRateWindow)
			return
		}

		// Update last activity time.
		mm.mu.Lock()
		if conn, exists := mm.connections[meshConn.Address]; exists {
			conn.LastPing = time.Now()
		}
		mm.mu.Unlock()
	}
}

// processStream extracts and processes every complete JSON message in data,
// treating non-JSON bytes (PING tokens, stray protocol text) as ignorable. It
// returns the number of bytes consumed (the caller retains the rest for the next
// read) and the number of messages processed (for rate limiting).
func (mm *MeshManager) processStream(address string, data []byte) (int, int) {
	consumed, processed := 0, 0

	for consumed < len(data) {
		// Find the next JSON object/array start; anything before it is non-JSON.
		jsonStart := -1
		for i := consumed; i < len(data); i++ {
			if data[i] == '{' || data[i] == '[' {
				jsonStart = i
				break
			}
		}
		if jsonStart == -1 {
			// No JSON ahead: the remainder is PING/junk, safe to discard.
			mm.logNonJSON(address, data[consumed:])
			return len(data), processed
		}
		if jsonStart > consumed {
			mm.logNonJSON(address, data[consumed:jsonStart])
		}

		end := jsonObjectEnd(data, jsonStart)
		if end == -1 {
			// Incomplete object: retain from its start and wait for more data.
			return jsonStart, processed
		}

		// Copy out the message so downstream handlers don't alias the buffer.
		msg := make([]byte, end-jsonStart)
		copy(msg, data[jsonStart:end])
		mm.processSingleJSONMessage(address, msg)
		processed++
		consumed = end
	}

	return consumed, processed
}

// logNonJSON logs unexpected non-JSON bytes on the mesh stream. PING tokens are
// expected and ignored silently.
func (mm *MeshManager) logNonJSON(address string, b []byte) {
	s := strings.TrimSpace(string(b))
	if s == "" || strings.HasPrefix(s, "PING:") {
		return
	}
	if len(s) > 40 {
		s = s[:40] + "..."
	}
	log.Printf("Ignoring non-JSON data from %s: %q", address, s)
}

// processSingleJSONMessage processes a single JSON message
func (mm *MeshManager) processSingleJSONMessage(address string, jsonData []byte) {
	// Try to decode as NetworkMessage first
	if err := mm.ReceiveNetworkMessage(jsonData); err != nil {
		// If it's not a NetworkMessage, it might be a consensus message
		// Check if it has a "type" field (consensus messages have this)
		if strings.Contains(string(jsonData), `"type"`) {
			// Route to consensus handler if available
			if mm.network.ConsensusMessageHandler != nil {
				if err := mm.network.ConsensusMessageHandler(jsonData, address); err != nil {
					log.Printf("Failed to handle consensus message from %s: %v", address, err)
					// Log the first 100 characters of the message for debugging
					if len(jsonData) > 100 {
						log.Printf("Message preview: %s...", string(jsonData[:100]))
					} else {
						log.Printf("Message content: %s", string(jsonData))
					}
				}
			} else {
				log.Printf("Received consensus message but no handler registered")
			}
		} else {
			log.Printf("Failed to decode JSON mesh message from %s: %v", address, err)
		}
	}
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
	case ConnectionEventReconnected:
		log.Printf("Mesh peer reconnected: %s", event.Address)
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
	if err := writeToConn(peer.Conn, []byte(pingMsg)); err != nil {
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

	// Update topology peer latency
	if topologyPeer, exists := mm.network.Topology.GetPeer(peer.Address); exists {
		topologyPeer.Latency = int(latency.Milliseconds())
		topologyPeer.LastSeen = time.Now().Unix()
	}

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
		if err := writeToConn(peer.Conn, message); err != nil {
			log.Printf("Failed to send to mesh peer %s: %v", peer.Address, err)
			lastError = err
		}
	}

	return lastError
}

// AcceptInboundConnection accepts an inbound connection and adds to mesh
func (mm *MeshManager) AcceptInboundConnection(conn net.Conn, remoteAddr string) {
	// Cap concurrent connections to avoid unbounded goroutine/memory growth
	// from a flood of inbound connections.
	mm.mu.RLock()
	nConns := len(mm.connections)
	mm.mu.RUnlock()
	if nConns >= maxMeshConnections {
		log.Printf("Rejecting inbound connection from %s: at max mesh connections (%d)", remoteAddr, maxMeshConnections)
		conn.Close()
		return
	}

	// --- Wallet handshake ---
	ourWallet := mm.network.Wallet.GetAddress()
	remoteWallet, err := readHandshakeLine(conn)
	if err != nil {
		conn.Close()
		return
	}
	// Send our wallet address in response
	if err = writeToConn(conn, []byte(ourWallet+"\n")); err != nil {
		conn.Close()
		return
	}
	if remoteWallet == ourWallet {
		// Self-connection, close quietly
		conn.Close()
		return
	}
	// --- End handshake ---

	// Update connection history (this will handle reconnections)
	mm.updateConnectionHistory(remoteAddr)

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

	// Add to topology as a direct peer
	peer := &Peer{
		Address:     remoteAddr,
		TrustScore:  0.5,
		UptimeScore: 0.5,
		AgeScore:    0.5,
		Latency:     0,
		HopDistance: 0,
		IsConnected: true,
		LastSeen:    time.Now().Unix(),
	}
	mm.network.Topology.AddPeer(peer)

	// Send connection event
	mm.connChan <- ConnectionEvent{
		Type:    ConnectionEventConnected,
		Address: remoteAddr,
		Conn:    meshConn,
	}

	// Start connection handler
	go mm.handleConnection(meshConn)
}

// SendNetworkMessage sends a NetworkMessage to all mesh peers
func (mm *MeshManager) SendNetworkMessage(msg *NetworkMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return mm.SendToMesh(data)
}

// ReceiveNetworkMessage decodes a NetworkMessage from bytes and forwards to MessageChan
func (mm *MeshManager) ReceiveNetworkMessage(data []byte) error {
	var msg NetworkMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	// Forward to network's message channel
	mm.network.MessageChan <- msg
	return nil
}

// BroadcastMessage sends a message to all mesh peers
func (mm *MeshManager) BroadcastMessage(message []byte) error {
	return mm.SendToMesh(message)
}

// SendMessageToPeer sends a message to a specific mesh peer
func (mm *MeshManager) SendMessageToPeer(peerID string, message []byte) error {
	mm.mu.RLock()
	conn, exists := mm.connections[peerID]
	mm.mu.RUnlock()
	if !exists || conn.Conn == nil {
		return fmt.Errorf("peer %s not connected", peerID)
	}
	return writeToConn(conn.Conn, message)
}

// GetStats returns mesh manager statistics
func (mm *MeshManager) GetStats() map[string]interface{} {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Get connection history stats
	mm.historyMu.RLock()
	historyStats := map[string]interface{}{
		"total_ips_tracked":    len(mm.connectionHistory),
		"connection_histories": make([]map[string]interface{}, 0),
	}

	for ip, history := range mm.connectionHistory {
		history.mu.RLock()
		historyStats["connection_histories"] = append(historyStats["connection_histories"].([]map[string]interface{}), map[string]interface{}{
			"ip":               ip,
			"last_address":     history.LastAddress,
			"connection_count": history.ConnectionCount,
			"disconnect_count": history.DisconnectCount,
			"trust_penalty":    history.TrustPenalty,
			"first_seen":       history.FirstSeen,
			"last_seen":        history.LastSeen,
			"last_disconnect":  history.LastDisconnect,
		})
		history.mu.RUnlock()
	}
	mm.historyMu.RUnlock()

	return map[string]interface{}{
		"connection_count":   len(mm.connections),
		"target_count":       mm.targetCount,
		"connection_history": historyStats,
	}
}

// GetActivePeers returns the list of connected peer addresses
func (mm *MeshManager) GetActivePeers() []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	peers := make([]string, 0, len(mm.connections))
	for addr, conn := range mm.connections {
		if conn.IsConnected {
			peers = append(peers, addr)
		}
	}
	return peers
}

// GetPeerCount returns the number of connected peers
func (mm *MeshManager) GetPeerCount() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	count := 0
	for _, conn := range mm.connections {
		if conn.IsConnected {
			count++
		}
	}
	return count
}

// IsPeerConnected checks if a peer is connected
func (mm *MeshManager) IsPeerConnected(peerID string) bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	conn, exists := mm.connections[peerID]
	return exists && conn.IsConnected
}

// GetConnectionHistory returns connection history for a specific IP
func (mm *MeshManager) GetConnectionHistory(ip string) *ConnectionHistory {
	mm.historyMu.RLock()
	defer mm.historyMu.RUnlock()
	return mm.connectionHistory[ip]
}

// GetConnectionHistoryStats returns summary statistics about connection history
func (mm *MeshManager) GetConnectionHistoryStats() map[string]interface{} {
	mm.historyMu.RLock()
	defer mm.historyMu.RUnlock()

	totalIPs := len(mm.connectionHistory)
	totalConnections := 0
	totalDisconnections := 0
	ipsWithPenalties := 0

	for _, history := range mm.connectionHistory {
		history.mu.RLock()
		totalConnections += history.ConnectionCount
		totalDisconnections += history.DisconnectCount
		if history.TrustPenalty > 0 {
			ipsWithPenalties++
		}
		history.mu.RUnlock()
	}

	return map[string]interface{}{
		"total_ips_tracked":      totalIPs,
		"total_connections":      totalConnections,
		"total_disconnections":   totalDisconnections,
		"ips_with_penalties":     ipsWithPenalties,
		"avg_connections_per_ip": float64(totalConnections) / float64(totalIPs),
	}
}
