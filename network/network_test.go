package network

import (
	"testing"
	"time"
)

func TestTrustEngine(t *testing.T) {
	engine := NewTrustEngine()

	// Test default weights
	if engine.UptimeWeight != 0.6 {
		t.Errorf("Expected uptime weight 0.6, got %f", engine.UptimeWeight)
	}

	if engine.AgeWeight != 0.4 {
		t.Errorf("Expected age weight 0.4, got %f", engine.AgeWeight)
	}

	// Test trust level classification
	if engine.GetTrustLevel(0.9) != "High" {
		t.Errorf("Expected High trust level for 0.9, got %s", engine.GetTrustLevel(0.9))
	}

	if engine.GetTrustLevel(0.7) != "Medium" {
		t.Errorf("Expected Medium trust level for 0.7, got %s", engine.GetTrustLevel(0.7))
	}

	if engine.GetTrustLevel(0.5) != "Low" {
		t.Errorf("Expected Low trust level for 0.5, got %s", engine.GetTrustLevel(0.5))
	}

	if engine.GetTrustLevel(0.2) != "Untrusted" {
		t.Errorf("Expected Untrusted level for 0.2, got %s", engine.GetTrustLevel(0.2))
	}
}

func TestPeerTrustCalculation(t *testing.T) {
	engine := NewTrustEngine()

	// Create a peer
	peer := &Peer{
		Address:   "test-peer-1",
		FirstSeen: time.Now().Unix() - 86400, // 1 day ago
		LastSeen:  time.Now().Unix(),
	}

	// Test uptime score update
	engine.UpdateUptimeScore(peer, 95.5) // 95.5% uptime
	if peer.UptimeScore != 0.955 {
		t.Errorf("Expected uptime score 0.955, got %f", peer.UptimeScore)
	}

	// Test trust score calculation
	trustScore := engine.CalculateTrustScore(peer)
	if trustScore <= 0 {
		t.Errorf("Expected positive trust score, got %f", trustScore)
	}

	// Test age calculation
	age := engine.GetPeerAge(peer)
	if age != 1 {
		t.Errorf("Expected peer age 1 day, got %d", age)
	}

	// Test trust validation
	if !engine.IsTrusted(peer, 0.1) {
		t.Errorf("Peer should be trusted with minimum score 0.1")
	}
}

func TestNetworkTopology(t *testing.T) {
	topology := NewNetworkTopology("test-node")

	// Test initial state
	if topology.NodeID != "test-node" {
		t.Errorf("Expected node ID 'test-node', got %s", topology.NodeID)
	}

	if len(topology.Peers) != 0 {
		t.Errorf("Expected empty peers map, got %d peers", len(topology.Peers))
	}

	// Test adding peers
	peer1 := &Peer{
		Address:    "peer-1",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.8,
		Latency:    50,
	}

	peer2 := &Peer{
		Address:    "peer-2",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.6,
		Latency:    100,
	}

	topology.AddPeer(peer1)
	topology.AddPeer(peer2)

	if len(topology.Peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(topology.Peers))
	}

	// Test route table
	if len(topology.RouteTable) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(topology.RouteTable))
	}

	// Test hop distance
	hopDistance := topology.GetHopDistance("peer-1")
	if hopDistance != 0 {
		t.Errorf("Expected hop distance 0 for direct peer, got %d", hopDistance)
	}

	// Test removing peer
	topology.RemovePeer("peer-1")
	if len(topology.Peers) != 1 {
		t.Errorf("Expected 1 peer after removal, got %d", len(topology.Peers))
	}
}

func TestGossipProtocol(t *testing.T) {
	topology := NewNetworkTopology("test-node")

	// Add some peers
	peer1 := &Peer{
		Address:    "peer-1",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.8,
		Latency:    50,
	}
	peer2 := &Peer{
		Address:    "peer-2",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.6,
		Latency:    100,
	}

	topology.AddPeer(peer1)
	topology.AddPeer(peer2)

	// Test gossip message creation
	gossipMsg := topology.CreateGossipMessage()

	if gossipMsg.SourceAddress != "test-node" {
		t.Errorf("Expected source address 'test-node', got %s", gossipMsg.SourceAddress)
	}

	if len(gossipMsg.Peers) != 2 {
		t.Errorf("Expected 2 peers in gossip message, got %d", len(gossipMsg.Peers))
	}

	// Test gossip message processing
	updatedRoutes := topology.ProcessGossipMessage(gossipMsg)
	if updatedRoutes != 0 {
		t.Errorf("Expected 0 updated routes for self-gossip, got %d", updatedRoutes)
	}
}

func TestPeerSelection(t *testing.T) {
	topology := NewNetworkTopology("test-node")

	// Add peers with different characteristics
	peer1 := &Peer{
		Address:    "fast-peer",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.5,
		Latency:    10, // Fastest
	}
	peer2 := &Peer{
		Address:    "trusted-peer",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.9, // Most trusted
		Latency:    100,
	}
	peer3 := &Peer{
		Address:    "distant-peer",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.3,
		Latency:    200,
	}

	topology.AddPeer(peer1)
	topology.AddPeer(peer2)
	topology.AddPeer(peer3)

	// Test peer selection
	selectedPeers := topology.SelectPeers(3)

	if len(selectedPeers) != 3 {
		t.Errorf("Expected 3 selected peers, got %d", len(selectedPeers))
	}

	// Verify we have the expected peers
	addresses := make(map[string]bool)
	for _, peer := range selectedPeers {
		addresses[peer.Address] = true
	}

	expectedAddresses := []string{"fast-peer", "trusted-peer", "distant-peer"}
	for _, expected := range expectedAddresses {
		if !addresses[expected] {
			t.Errorf("Expected peer %s to be selected", expected)
		}
	}
}

func TestMessageRouter(t *testing.T) {
	router := NewMessageRouter()

	// Test duplicate filter
	msg1 := NetworkMessage{
		Type:      MessageTypePost,
		Source:    "test-source",
		Timestamp: time.Now().Unix(),
	}

	msg2 := NetworkMessage{
		Type:      MessageTypePost,
		Source:    "test-source",
		Timestamp: msg1.Timestamp, // Same timestamp = duplicate
	}

	// First message should not be duplicate
	if router.DuplicateFilter.IsDuplicate(msg1) {
		t.Error("First message should not be duplicate")
	}

	// Add first message
	router.DuplicateFilter.AddMessage(msg1)

	// Second message should be duplicate
	if !router.DuplicateFilter.IsDuplicate(msg2) {
		t.Error("Second message should be duplicate")
	}

	// Test spam protection
	if router.SpamProtection.IsSpam("test-source") {
		t.Error("New source should not be considered spam")
	}

	// Add many messages from same source
	for i := 0; i < 101; i++ {
		router.SpamProtection.AddMessage("test-source")
	}

	// Now it should be considered spam
	if !router.SpamProtection.IsSpam("test-source") {
		t.Error("Source should be considered spam after too many messages")
	}
}

func TestNetworkStats(t *testing.T) {
	topology := NewNetworkTopology("test-node")

	// Add some peers
	peer1 := &Peer{
		Address:    "peer-1",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.8,
		Latency:    50,
	}
	peer2 := &Peer{
		Address:    "peer-2",
		FirstSeen:  time.Now().Unix(),
		TrustScore: 0.6,
		Latency:    100,
	}

	topology.AddPeer(peer1)
	topology.AddPeer(peer2)

	// Test network stats
	stats := topology.GetNetworkStats()

	if stats["total_peers"].(int) != 2 {
		t.Errorf("Expected 2 total peers, got %d", stats["total_peers"])
	}

	if stats["total_routes"].(int) != 2 {
		t.Errorf("Expected 2 total routes, got %d", stats["total_routes"])
	}

	avgTrust := stats["average_trust"].(float64)
	if avgTrust != 0.7 {
		t.Errorf("Expected average trust 0.7, got %f", avgTrust)
	}

	hopDistribution := stats["hop_distribution"].(map[int]int)
	if hopDistribution[0] != 2 {
		t.Errorf("Expected 2 peers at hop distance 0, got %d", hopDistribution[0])
	}
}

func TestTrustNetworkIntegration(t *testing.T) {
	// This test would require mocking the blockchain components
	// For now, we'll test the basic structure

	// Create a mock trust network (without blockchain dependencies)
	network := &TrustNetwork{
		NodeID:        "test-node",
		TrustEngine:   NewTrustEngine(),
		Topology:      NewNetworkTopology("test-node"),
		MessageRouter: NewMessageRouter(),
		ListenPort:    8080,
		MaxPeers:      10,
		MinTrustScore: 0.3,
		MessageChan:   make(chan NetworkMessage, 100),
		PeerChan:      make(chan PeerEvent, 50),
		StopChan:      make(chan struct{}),
	}

	// Set up message router
	network.MessageRouter.Network = network

	// Test network stats
	stats := network.GetNetworkStats()

	if stats["node_id"].(string) != "test-node" {
		t.Errorf("Expected node ID 'test-node', got %s", stats["node_id"])
	}

	if stats["is_running"].(bool) != false {
		t.Errorf("Expected network to not be running initially")
	}

	if stats["listen_port"].(int) != 8080 {
		t.Errorf("Expected listen port 8080, got %d", stats["listen_port"])
	}

	if stats["max_peers"].(int) != 10 {
		t.Errorf("Expected max peers 10, got %d", stats["max_peers"])
	}

	if stats["min_trust_score"].(float64) != 0.3 {
		t.Errorf("Expected min trust score 0.3, got %f", stats["min_trust_score"])
	}
}
