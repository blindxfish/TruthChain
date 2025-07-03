package chain

import (
	"testing"
	"time"
)

func TestChainSyncManager(t *testing.T) {
	// Create a test blockchain
	blockchain := NewBlockchain(5)

	// Create a test node ID
	nodeID := "test_node_123"

	// Create sync manager
	syncManager := NewChainSyncManager(blockchain, nodeID)

	// Test initial state
	if syncManager.nodeID != nodeID {
		t.Errorf("Expected node ID %s, got %s", nodeID, syncManager.nodeID)
	}

	if len(syncManager.peers) != 0 {
		t.Error("Should start with no peers")
	}
}

func TestDiscoverBeaconsFromChain(t *testing.T) {
	// Create a test blockchain
	blockchain := NewBlockchain(5)

	// Create sync manager
	syncManager := NewChainSyncManager(blockchain, "test_node")

	// Test with empty blockchain
	beacons, err := syncManager.DiscoverBeaconsFromChain(100)
	if err != nil {
		t.Fatalf("Failed to discover beacons: %v", err)
	}

	if len(beacons) != 0 {
		t.Error("Should find no beacons in empty blockchain")
	}

	// Add some blocks with beacon announcements
	block1 := CreateBlock(1, blockchain.GetLatestBlock().Hash, []Post{}, []Transfer{}, nil)
	block1.BeaconAnnounce = &BeaconAnnounce{
		NodeID:    "beacon1",
		IP:        "192.168.1.100",
		Port:      8080,
		Timestamp: time.Now().Unix(),
		Uptime:    95.5,
		Version:   "v1.0.0",
		Sig:       "test_signature_1",
	}
	blockchain.Blocks = append(blockchain.Blocks, block1)

	block2 := CreateBlock(2, block1.Hash, []Post{}, []Transfer{}, nil)
	block2.BeaconAnnounce = &BeaconAnnounce{
		NodeID:    "beacon2",
		IP:        "192.168.1.101",
		Port:      8081,
		Timestamp: time.Now().Unix(),
		Uptime:    98.0,
		Version:   "v1.0.0",
		Sig:       "test_signature_2",
	}
	blockchain.Blocks = append(blockchain.Blocks, block2)

	// Test beacon discovery
	beacons, err = syncManager.DiscoverBeaconsFromChain(10)
	if err != nil {
		t.Fatalf("Failed to discover beacons: %v", err)
	}

	if len(beacons) != 2 {
		t.Errorf("Expected 2 beacons, got %d", len(beacons))
	}

	// Check first beacon
	if beacons[0].NodeID != "beacon1" {
		t.Errorf("Expected beacon1, got %s", beacons[0].NodeID)
	}

	if beacons[0].IP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", beacons[0].IP)
	}

	// Check second beacon
	if beacons[1].NodeID != "beacon2" {
		t.Errorf("Expected beacon2, got %s", beacons[1].NodeID)
	}

	if beacons[1].IP != "192.168.1.101" {
		t.Errorf("Expected IP 192.168.1.101, got %s", beacons[1].IP)
	}
}

func TestPeerManagement(t *testing.T) {
	// Create a test blockchain
	blockchain := NewBlockchain(5)

	// Create sync manager
	syncManager := NewChainSyncManager(blockchain, "test_node")

	// Add peers
	syncManager.AddPeer("peer1", "192.168.1.100", 8080)
	syncManager.AddPeer("peer2", "192.168.1.101", 8081)

	// Test peer count
	peers := syncManager.GetPeers()
	if len(peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(peers))
	}

	// Test peer details
	peer1 := peers[0]
	if peer1.NodeID != "peer1" && peer1.NodeID != "peer2" {
		t.Errorf("Unexpected peer ID: %s", peer1.NodeID)
	}

	if peer1.TrustScore != 0.5 {
		t.Errorf("Expected initial trust score 0.5, got %f", peer1.TrustScore)
	}

	if peer1.IsReachable {
		t.Error("New peer should not be reachable initially")
	}

	// Test reachability update
	syncManager.UpdatePeerReachability("peer1", true)

	reachablePeers := syncManager.GetReachablePeers()
	if len(reachablePeers) != 1 {
		t.Errorf("Expected 1 reachable peer, got %d", len(reachablePeers))
	}

	if reachablePeers[0].NodeID != "peer1" {
		t.Errorf("Expected peer1 to be reachable, got %s", reachablePeers[0].NodeID)
	}

	// Test trust score boost
	if reachablePeers[0].TrustScore <= 0.5 {
		t.Error("Trust score should be boosted for reachable peer")
	}
}

func TestBeaconAnnounceValidation(t *testing.T) {
	// Test valid beacon announcement
	validBeacon := &BeaconAnnounce{
		NodeID:    "beacon1",
		IP:        "192.168.1.100",
		Port:      8080,
		Timestamp: time.Now().Unix(),
		Uptime:    95.5,
		Version:   "v1.0.0",
		Sig:       "test_signature",
	}

	if err := validBeacon.ValidateBeaconAnnounce(); err != nil {
		t.Errorf("Valid beacon should pass validation: %v", err)
	}

	// Test invalid beacon announcements
	testCases := []struct {
		name        string
		beacon      *BeaconAnnounce
		expectError bool
	}{
		{
			name: "empty node ID",
			beacon: &BeaconAnnounce{
				NodeID:    "",
				IP:        "192.168.1.100",
				Port:      8080,
				Timestamp: time.Now().Unix(),
				Uptime:    95.5,
				Version:   "v1.0.0",
				Sig:       "test_signature",
			},
			expectError: true,
		},
		{
			name: "empty IP",
			beacon: &BeaconAnnounce{
				NodeID:    "beacon1",
				IP:        "",
				Port:      8080,
				Timestamp: time.Now().Unix(),
				Uptime:    95.5,
				Version:   "v1.0.0",
				Sig:       "test_signature",
			},
			expectError: true,
		},
		{
			name: "invalid port",
			beacon: &BeaconAnnounce{
				NodeID:    "beacon1",
				IP:        "192.168.1.100",
				Port:      70000, // > 65535
				Timestamp: time.Now().Unix(),
				Uptime:    95.5,
				Version:   "v1.0.0",
				Sig:       "test_signature",
			},
			expectError: true,
		},
		{
			name: "invalid uptime",
			beacon: &BeaconAnnounce{
				NodeID:    "beacon1",
				IP:        "192.168.1.100",
				Port:      8080,
				Timestamp: time.Now().Unix(),
				Uptime:    150.0, // > 100
				Version:   "v1.0.0",
				Sig:       "test_signature",
			},
			expectError: true,
		},
		{
			name: "empty signature",
			beacon: &BeaconAnnounce{
				NodeID:    "beacon1",
				IP:        "192.168.1.100",
				Port:      8080,
				Timestamp: time.Now().Unix(),
				Uptime:    95.5,
				Version:   "v1.0.0",
				Sig:       "",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.beacon.ValidateBeaconAnnounce()
			if tc.expectError && err == nil {
				t.Error("Expected validation error, got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestSyncStats(t *testing.T) {
	// Create a test blockchain
	blockchain := NewBlockchain(5)

	// Create sync manager
	syncManager := NewChainSyncManager(blockchain, "test_node")

	// Add some peers
	syncManager.AddPeer("peer1", "192.168.1.100", 8080)
	syncManager.AddPeer("peer2", "192.168.1.101", 8081)
	syncManager.UpdatePeerReachability("peer1", true)

	// Get stats
	stats := syncManager.GetSyncStats()

	if stats["total_peers"].(int) != 2 {
		t.Errorf("Expected 2 total peers, got %d", stats["total_peers"])
	}

	if stats["reachable_peers"].(int) != 1 {
		t.Errorf("Expected 1 reachable peer, got %d", stats["reachable_peers"])
	}

	if stats["node_id"].(string) != "test_node" {
		t.Errorf("Expected node_id 'test_node', got %s", stats["node_id"])
	}

	avgTrustScore := stats["average_trust_score"].(float64)
	if avgTrustScore <= 0.5 {
		t.Errorf("Expected average trust score > 0.5, got %f", avgTrustScore)
	}
}

func TestCleanupOldPeers(t *testing.T) {
	// Create a test blockchain
	blockchain := NewBlockchain(5)

	// Create sync manager
	syncManager := NewChainSyncManager(blockchain, "test_node")

	// Add a peer
	syncManager.AddPeer("peer1", "192.168.1.100", 8080)

	// Manually set last seen to old time
	if peer, exists := syncManager.peers["peer1"]; exists {
		peer.LastSeen = time.Now().Add(-25 * time.Hour) // 25 hours ago
	}

	// Cleanup old peers (older than 24 hours)
	removed := syncManager.CleanupOldPeers(24 * time.Hour)
	if removed != 1 {
		t.Errorf("Expected 1 peer removed, got %d", removed)
	}

	if len(syncManager.peers) != 0 {
		t.Error("All old peers should be removed")
	}
}
