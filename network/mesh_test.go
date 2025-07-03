package network

import (
	"fmt"
	"testing"
	"time"
)

func TestNewPeerTable(t *testing.T) {
	pt := NewPeerTable(10)

	if pt.maxPeers != 10 {
		t.Errorf("Expected maxPeers to be 10, got %d", pt.maxPeers)
	}

	if len(pt.peers) != 0 {
		t.Errorf("Expected empty peer table, got %d peers", len(pt.peers))
	}

	if len(pt.connections) != 0 {
		t.Errorf("Expected empty connections, got %d connections", len(pt.connections))
	}
}

func TestAddPeer(t *testing.T) {
	pt := NewPeerTable(10)

	// Add a new peer
	pt.AddPeer("192.168.1.1:8080", 1, "", 0.8)

	peer, exists := pt.GetPeer("192.168.1.1:8080")
	if !exists {
		t.Fatal("Peer should exist after adding")
	}

	if peer.Address != "192.168.1.1:8080" {
		t.Errorf("Expected address 192.168.1.1:8080, got %s", peer.Address)
	}

	if peer.HopDistance != 1 {
		t.Errorf("Expected hop distance 1, got %d", peer.HopDistance)
	}

	if peer.TrustScore != 0.8 {
		t.Errorf("Expected trust score 0.8, got %f", peer.TrustScore)
	}

	if peer.IsConnected {
		t.Error("New peer should not be connected")
	}
}

func TestUpdatePeer(t *testing.T) {
	pt := NewPeerTable(10)

	// Add initial peer
	pt.AddPeer("192.168.1.1:8080", 2, "via-peer", 0.5)

	// Update with better path
	pt.AddPeer("192.168.1.1:8080", 1, "direct", 0.8)

	peer, exists := pt.GetPeer("192.168.1.1:8080")
	if !exists {
		t.Fatal("Peer should exist")
	}

	// Should use shorter path
	if peer.HopDistance != 1 {
		t.Errorf("Expected hop distance 1, got %d", peer.HopDistance)
	}

	if peer.Via != "direct" {
		t.Errorf("Expected via 'direct', got %s", peer.Via)
	}

	// Should use higher trust score
	if peer.TrustScore != 0.8 {
		t.Errorf("Expected trust score 0.8, got %f", peer.TrustScore)
	}
}

func TestUpdatePeerLatency(t *testing.T) {
	pt := NewPeerTable(10)
	pt.AddPeer("192.168.1.1:8080", 1, "", 0.5)

	pt.UpdatePeerLatency("192.168.1.1:8080", 150)

	peer, _ := pt.GetPeer("192.168.1.1:8080")
	if peer.Latency != 150 {
		t.Errorf("Expected latency 150, got %d", peer.Latency)
	}
}

func TestUpdatePeerTrust(t *testing.T) {
	pt := NewPeerTable(10)
	pt.AddPeer("192.168.1.1:8080", 1, "", 0.5)

	pt.UpdatePeerTrust("192.168.1.1:8080", 0.9)

	peer, _ := pt.GetPeer("192.168.1.1:8080")
	if peer.TrustScore != 0.9 {
		t.Errorf("Expected trust score 0.9, got %f", peer.TrustScore)
	}
}

func TestMarkConnected(t *testing.T) {
	pt := NewPeerTable(10)
	pt.AddPeer("192.168.1.1:8080", 1, "", 0.5)

	pt.MarkConnected("192.168.1.1:8080")

	peer, _ := pt.GetPeer("192.168.1.1:8080")
	if !peer.IsConnected {
		t.Error("Peer should be marked as connected")
	}

	if !pt.connections["192.168.1.1:8080"] {
		t.Error("Connection should be recorded")
	}
}

func TestMarkDisconnected(t *testing.T) {
	pt := NewPeerTable(10)
	pt.AddPeer("192.168.1.1:8080", 1, "", 0.5)
	pt.MarkConnected("192.168.1.1:8080")

	pt.MarkDisconnected("192.168.1.1:8080")

	peer, _ := pt.GetPeer("192.168.1.1:8080")
	if peer.IsConnected {
		t.Error("Peer should be marked as disconnected")
	}

	if pt.connections["192.168.1.1:8080"] {
		t.Error("Connection should be removed")
	}
}

func TestGetAllPeers(t *testing.T) {
	pt := NewPeerTable(10)

	pt.AddPeer("192.168.1.1:8080", 1, "", 0.5)
	pt.AddPeer("192.168.1.2:8080", 2, "192.168.1.1:8080", 0.7)

	peers := pt.GetAllPeers()
	if len(peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(peers))
	}
}

func TestGetConnectedPeers(t *testing.T) {
	pt := NewPeerTable(10)

	pt.AddPeer("192.168.1.1:8080", 1, "", 0.5)
	pt.AddPeer("192.168.1.2:8080", 2, "192.168.1.1:8080", 0.7)

	pt.MarkConnected("192.168.1.1:8080")

	connected := pt.GetConnectedPeers()
	if len(connected) != 1 {
		t.Errorf("Expected 1 connected peer, got %d", len(connected))
	}

	if connected[0].Address != "192.168.1.1:8080" {
		t.Errorf("Expected connected peer 192.168.1.1:8080, got %s", connected[0].Address)
	}
}

func TestSelectPeers(t *testing.T) {
	pt := NewPeerTable(10)

	// Add peers with different characteristics
	pt.AddPeer("192.168.1.1:8080", 1, "", 0.9)                 // High trust, direct
	pt.AddPeer("192.168.1.2:8080", 3, "192.168.1.1:8080", 0.5) // Distant
	pt.AddPeer("192.168.1.3:8080", 2, "192.168.1.1:8080", 0.7) // Medium
	pt.AddPeer("192.168.1.4:8080", 1, "", 0.6)                 // Direct, lower trust

	// Set latencies
	pt.UpdatePeerLatency("192.168.1.1:8080", 50)  // Fastest
	pt.UpdatePeerLatency("192.168.1.2:8080", 200) // Slowest
	pt.UpdatePeerLatency("192.168.1.3:8080", 100) // Medium
	pt.UpdatePeerLatency("192.168.1.4:8080", 75)  // Fast

	selected := pt.SelectPeers(3)
	if len(selected) != 3 {
		t.Errorf("Expected 3 selected peers, got %d", len(selected))
	}

	// Should have diverse selection
	addresses := make(map[string]bool)
	for _, peer := range selected {
		addresses[peer.Address] = true
	}

	if len(addresses) != 3 {
		t.Error("Selected peers should be unique")
	}
}

func TestCleanupOldPeers(t *testing.T) {
	pt := NewPeerTable(10)

	pt.AddPeer("192.168.1.1:8080", 1, "", 0.5)
	pt.AddPeer("192.168.1.2:8080", 2, "192.168.1.1:8080", 0.7)

	// Simulate old peer by manually setting LastSeen
	pt.peers["192.168.1.1:8080"].LastSeen = time.Now().Add(-2 * time.Hour)

	removed := pt.CleanupOldPeers(1 * time.Hour)
	if removed != 1 {
		t.Errorf("Expected 1 peer removed, got %d", removed)
	}

	_, exists := pt.GetPeer("192.168.1.1:8080")
	if exists {
		t.Error("Old peer should be removed")
	}

	_, exists = pt.GetPeer("192.168.1.2:8080")
	if !exists {
		t.Error("Recent peer should not be removed")
	}
}

func TestGetMeshStats(t *testing.T) {
	pt := NewPeerTable(10)

	pt.AddPeer("192.168.1.1:8080", 1, "", 0.8)
	pt.AddPeer("192.168.1.2:8080", 2, "192.168.1.1:8080", 0.6)
	pt.AddPeer("192.168.1.3:8080", 1, "", 0.9)

	pt.UpdatePeerLatency("192.168.1.1:8080", 100)
	pt.UpdatePeerLatency("192.168.1.2:8080", 200)
	pt.UpdatePeerLatency("192.168.1.3:8080", 150)

	pt.MarkConnected("192.168.1.1:8080")
	pt.MarkConnected("192.168.1.3:8080")

	stats := pt.GetMeshStats()

	if stats["total_peers"] != 3 {
		t.Errorf("Expected 3 total peers, got %v", stats["total_peers"])
	}

	if stats["connected_peers"] != 2 {
		t.Errorf("Expected 2 connected peers, got %v", stats["connected_peers"])
	}

	avgTrust := stats["average_trust"].(float64)
	if avgTrust < 0.766 || avgTrust > 0.768 {
		t.Errorf("Expected average trust ~0.77, got %v", avgTrust)
	}

	if stats["average_latency"] != int64(150) {
		t.Errorf("Expected average latency 150, got %v", stats["average_latency"])
	}

	avgHop := stats["average_hop_distance"].(float64)
	if avgHop < 1.33 || avgHop > 1.34 {
		t.Errorf("Expected average hop distance ~1.33, got %v", avgHop)
	}
}

func TestProcessGossipMessage(t *testing.T) {
	pt := NewPeerTable(10)

	// Add local peer
	pt.AddPeer("192.168.1.1:8080", 1, "", 0.8)

	// Create gossip message with new peers
	gossipPeers := []*MeshPeer{
		{
			Address:     "192.168.1.2:8080",
			HopDistance: 1,
			Via:         "",
			TrustScore:  0.7,
			Latency:     100,
			LastSeen:    time.Now(),
			IsConnected: false,
			IsBeacon:    false,
			Version:     "1.0.0",
			Uptime:      95.5,
		},
		{
			Address:     "192.168.1.3:8080",
			HopDistance: 2,
			Via:         "192.168.1.2:8080",
			TrustScore:  0.6,
			Latency:     150,
			LastSeen:    time.Now(),
			IsConnected: false,
			IsBeacon:    true,
			Version:     "1.0.0",
			Uptime:      98.2,
		},
	}

	pt.ProcessGossipMessage("192.168.1.1:8080", gossipPeers)

	// Check that new peers were added with incremented hop distance
	peer2, exists := pt.GetPeer("192.168.1.2:8080")
	if !exists {
		t.Fatal("Peer 2 should be added")
	}

	if peer2.HopDistance != 2 { // 1 + 1 (via gossip)
		t.Errorf("Expected hop distance 2, got %d", peer2.HopDistance)
	}

	if peer2.Via != "192.168.1.1:8080" {
		t.Errorf("Expected via '192.168.1.1:8080', got %s", peer2.Via)
	}

	peer3, exists := pt.GetPeer("192.168.1.3:8080")
	if !exists {
		t.Fatal("Peer 3 should be added")
	}

	if peer3.HopDistance != 3 { // 2 + 1 (via gossip)
		t.Errorf("Expected hop distance 3, got %d", peer3.HopDistance)
	}

	if !peer3.IsBeacon {
		t.Error("Peer 3 should be marked as beacon")
	}
}

func TestCreateGossipMessage(t *testing.T) {
	pt := NewPeerTable(10)

	pt.AddPeer("192.168.1.1:8080", 1, "", 0.8)
	pt.AddPeer("192.168.1.2:8080", 2, "192.168.1.1:8080", 0.7)

	gossip := pt.CreateGossipMessage()

	if len(gossip) != 2 {
		t.Errorf("Expected 2 peers in gossip message, got %d", len(gossip))
	}

	// Check that all peers are included
	addresses := make(map[string]bool)
	for _, peer := range gossip {
		addresses[peer.Address] = true
	}

	if !addresses["192.168.1.1:8080"] {
		t.Error("Peer 1 should be in gossip message")
	}

	if !addresses["192.168.1.2:8080"] {
		t.Error("Peer 2 should be in gossip message")
	}
}

func TestPeerTableConcurrency(t *testing.T) {
	pt := NewPeerTable(100)

	// Test concurrent access
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			address := fmt.Sprintf("192.168.1.%d:8080", id)
			pt.AddPeer(address, 1, "", 0.5)
			pt.UpdatePeerLatency(address, int64(100+id))
			pt.MarkConnected(address)
			pt.GetAllPeers()
			pt.GetMeshStats()
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have all peers
	peers := pt.GetAllPeers()
	if len(peers) != 10 {
		t.Errorf("Expected 10 peers after concurrent operations, got %d", len(peers))
	}
}
