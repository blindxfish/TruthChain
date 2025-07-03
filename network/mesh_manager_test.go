package network

import (
	"fmt"
	"testing"
	"time"
)

func TestNewMeshManager(t *testing.T) {
	// Create a mock network
	network := &TrustNetwork{
		NodeID:    "test-node",
		PeerTable: NewPeerTable(10),
	}

	mm := NewMeshManager(network)

	if mm.network != network {
		t.Error("Mesh manager should reference the network")
	}

	if mm.targetCount != 3 {
		t.Errorf("Expected target count 3, got %d", mm.targetCount)
	}

	if len(mm.connections) != 0 {
		t.Error("New mesh manager should have no connections")
	}
}

func TestMeshManagerStartStop(t *testing.T) {
	network := &TrustNetwork{
		NodeID:    "test-node",
		PeerTable: NewPeerTable(10),
	}

	mm := NewMeshManager(network)

	// Test start
	err := mm.Start()
	if err != nil {
		t.Errorf("Failed to start mesh manager: %v", err)
	}

	// Test stop
	err = mm.Stop()
	if err != nil {
		t.Errorf("Failed to stop mesh manager: %v", err)
	}
}

func TestMeshManagerConnectionSelection(t *testing.T) {
	network := &TrustNetwork{
		NodeID:    "test-node",
		PeerTable: NewPeerTable(10),
	}

	mm := NewMeshManager(network)

	// Add some peers to the peer table
	network.PeerTable.AddPeer("192.168.1.1:8080", 1, "", 0.8)
	network.PeerTable.AddPeer("192.168.1.2:8080", 2, "192.168.1.1:8080", 0.6)
	network.PeerTable.AddPeer("192.168.1.3:8080", 3, "192.168.1.2:8080", 0.7)
	network.PeerTable.AddPeer("192.168.1.4:8080", 1, "", 0.9)

	// Set latencies
	network.PeerTable.UpdatePeerLatency("192.168.1.1:8080", 50)
	network.PeerTable.UpdatePeerLatency("192.168.1.2:8080", 150)
	network.PeerTable.UpdatePeerLatency("192.168.1.3:8080", 200)
	network.PeerTable.UpdatePeerLatency("192.168.1.4:8080", 75)

	// Test connection selection
	mm.selectAndMaintainConnections()

	// Should have attempted to connect to selected peers
	// (Note: actual connections will fail in test environment)
	stats := mm.GetMeshStats()

	if stats["target_connections"] != 3 {
		t.Errorf("Expected target connections 3, got %v", stats["target_connections"])
	}
}

func TestMeshManagerStats(t *testing.T) {
	network := &TrustNetwork{
		NodeID:    "test-node",
		PeerTable: NewPeerTable(10),
	}

	mm := NewMeshManager(network)

	stats := mm.GetMeshStats()

	expectedFields := []string{
		"target_connections",
		"connected_peers",
		"average_latency_ms",
		"selection_interval_s",
		"ping_interval_s",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Stats missing field: %s", field)
		}
	}

	if stats["target_connections"] != 3 {
		t.Errorf("Expected target connections 3, got %v", stats["target_connections"])
	}

	if stats["connected_peers"] != 0 {
		t.Errorf("Expected 0 connected peers initially, got %v", stats["connected_peers"])
	}
}

func TestMeshConnection(t *testing.T) {
	conn := &MeshConnection{
		Address:     "192.168.1.1:8080",
		IsConnected: true,
		LastPing:    time.Now(),
		Latency:     100 * time.Millisecond,
		TrustScore:  0.8,
		HopDistance: 1,
	}

	if conn.Address != "192.168.1.1:8080" {
		t.Errorf("Expected address 192.168.1.1:8080, got %s", conn.Address)
	}

	if !conn.IsConnected {
		t.Error("Connection should be marked as connected")
	}

	if conn.Latency != 100*time.Millisecond {
		t.Errorf("Expected latency 100ms, got %v", conn.Latency)
	}
}

func TestConnectionEvent(t *testing.T) {
	event := ConnectionEvent{
		Type:    ConnectionEventConnected,
		Address: "192.168.1.1:8080",
		Latency: 100 * time.Millisecond,
	}

	if event.Type != ConnectionEventConnected {
		t.Error("Event type should be ConnectionEventConnected")
	}

	if event.Address != "192.168.1.1:8080" {
		t.Errorf("Expected address 192.168.1.1:8080, got %s", event.Address)
	}

	if event.Latency != 100*time.Millisecond {
		t.Errorf("Expected latency 100ms, got %v", event.Latency)
	}
}

func TestMeshManagerDropConnection(t *testing.T) {
	network := &TrustNetwork{
		NodeID:    "test-node",
		PeerTable: NewPeerTable(10),
	}

	mm := NewMeshManager(network)

	// Add a peer to the peer table
	network.PeerTable.AddPeer("192.168.1.1:8080", 1, "", 0.8)

	// Simulate dropping a connection
	mm.dropConnection("192.168.1.1:8080")

	// Check that peer is marked as disconnected
	peer, exists := network.PeerTable.GetPeer("192.168.1.1:8080")
	if !exists {
		t.Fatal("Peer should still exist in peer table")
	}

	if peer.IsConnected {
		t.Error("Peer should be marked as disconnected")
	}
}

func TestMeshManagerConcurrency(t *testing.T) {
	network := &TrustNetwork{
		NodeID:    "test-node",
		PeerTable: NewPeerTable(10),
	}

	mm := NewMeshManager(network)

	// Test concurrent access to mesh manager
	done := make(chan bool, 5)

	for i := 0; i < 5; i++ {
		go func(id int) {
			address := fmt.Sprintf("192.168.1.%d:8080", id)
			network.PeerTable.AddPeer(address, 1, "", 0.5)
			mm.GetMeshStats()
			mm.dropConnection(address)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}

	// Should not have crashed
	stats := mm.GetMeshStats()
	if stats == nil {
		t.Error("Stats should not be nil after concurrent access")
	}
}
