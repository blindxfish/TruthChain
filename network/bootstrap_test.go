package network

import (
	"os"
	"testing"
	"time"
)

func TestNewBootstrapManager(t *testing.T) {
	// Create temporary config file with some content
	tmpFile, err := os.CreateTemp("", "bootstrap_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write empty array to avoid JSON parsing error
	if _, err := tmpFile.WriteString("[]"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	bm := NewBootstrapManager(tmpFile.Name())

	if bm.ConfigFile != tmpFile.Name() {
		t.Errorf("Expected config file %s, got %s", tmpFile.Name(), bm.ConfigFile)
	}

	if len(bm.Nodes) == 0 {
		t.Error("Bootstrap manager should have default nodes")
	}
}

func TestLoadDefaultNodes(t *testing.T) {
	bm := &BootstrapManager{
		Nodes: make([]*BootstrapNode, 0),
	}

	bm.loadDefaultNodes()

	if len(bm.Nodes) == 0 {
		t.Error("Should load default nodes")
	}

	// Check for beacon nodes
	beaconCount := 0
	for _, node := range bm.Nodes {
		if node.IsBeacon {
			beaconCount++
		}
	}

	if beaconCount == 0 {
		t.Error("Should have beacon nodes in defaults")
	}

	// Check for mesh nodes
	meshCount := 0
	for _, node := range bm.Nodes {
		if !node.IsBeacon {
			meshCount++
		}
	}

	if meshCount == 0 {
		t.Error("Should have mesh nodes in defaults")
	}
}

func TestAddNode(t *testing.T) {
	// Create temporary config file with some content
	tmpFile, err := os.CreateTemp("", "bootstrap_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write empty array to avoid JSON parsing error
	if _, err := tmpFile.WriteString("[]"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	bm := NewBootstrapManager(tmpFile.Name())

	// Add a new node
	err = bm.AddNode("192.168.1.100:9876", "Test Node", "Test Region", true, 0.8)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	nodes := bm.GetNodes()
	if len(nodes) == 0 {
		t.Error("Should have nodes after adding")
	}

	// Check if the node was added
	found := false
	for _, node := range nodes {
		if node.Address == "192.168.1.100:9876" {
			found = true
			if node.Description != "Test Node" {
				t.Errorf("Expected description 'Test Node', got %s", node.Description)
			}
			if node.Region != "Test Region" {
				t.Errorf("Expected region 'Test Region', got %s", node.Region)
			}
			if !node.IsBeacon {
				t.Error("Node should be marked as beacon")
			}
			if node.TrustScore != 0.8 {
				t.Errorf("Expected trust score 0.8, got %f", node.TrustScore)
			}
			break
		}
	}

	if !found {
		t.Error("Added node not found in list")
	}
}

func TestRemoveNode(t *testing.T) {
	// Create temporary config file with some content
	tmpFile, err := os.CreateTemp("", "bootstrap_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write empty array to avoid JSON parsing error
	if _, err := tmpFile.WriteString("[]"); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	bm := NewBootstrapManager(tmpFile.Name())

	// Add a node
	bm.AddNode("192.168.1.100:9876", "Test Node", "Test Region", true, 0.8)

	// Remove the node
	err = bm.RemoveNode("192.168.1.100:9876")
	if err != nil {
		t.Fatalf("Failed to remove node: %v", err)
	}

	// Try to remove non-existent node
	err = bm.RemoveNode("192.168.1.999:9876")
	if err == nil {
		t.Error("Should return error for non-existent node")
	}
}

func TestGetBeaconNodes(t *testing.T) {
	bm := &BootstrapManager{
		Nodes: []*BootstrapNode{
			{
				Address:     "beacon1:9876",
				Description: "Beacon 1",
				IsBeacon:    true,
			},
			{
				Address:     "mesh1:9876",
				Description: "Mesh 1",
				IsBeacon:    false,
			},
			{
				Address:     "beacon2:9876",
				Description: "Beacon 2",
				IsBeacon:    true,
			},
		},
	}

	beacons := bm.GetBeaconNodes()
	if len(beacons) != 2 {
		t.Errorf("Expected 2 beacon nodes, got %d", len(beacons))
	}

	for _, beacon := range beacons {
		if !beacon.IsBeacon {
			t.Error("All returned nodes should be beacons")
		}
	}
}

func TestGetNodesByRegion(t *testing.T) {
	bm := &BootstrapManager{
		Nodes: []*BootstrapNode{
			{
				Address:     "node1:9876",
				Description: "Node 1",
				Region:      "North America",
			},
			{
				Address:     "node2:9876",
				Description: "Node 2",
				Region:      "Europe",
			},
			{
				Address:     "node3:9876",
				Description: "Node 3",
				Region:      "North America",
			},
		},
	}

	naNodes := bm.GetNodesByRegion("North America")
	if len(naNodes) != 2 {
		t.Errorf("Expected 2 North America nodes, got %d", len(naNodes))
	}

	euNodes := bm.GetNodesByRegion("Europe")
	if len(euNodes) != 1 {
		t.Errorf("Expected 1 Europe node, got %d", len(euNodes))
	}

	asiaNodes := bm.GetNodesByRegion("Asia")
	if len(asiaNodes) != 0 {
		t.Errorf("Expected 0 Asia nodes, got %d", len(asiaNodes))
	}
}

func TestUpdateLastSeen(t *testing.T) {
	bm := &BootstrapManager{
		Nodes: []*BootstrapNode{
			{
				Address:  "node1:9876",
				LastSeen: 0,
			},
		},
	}

	before := bm.Nodes[0].LastSeen
	time.Sleep(1 * time.Second) // Ensure time difference
	bm.UpdateLastSeen("node1:9876")
	after := bm.Nodes[0].LastSeen

	if after <= before {
		t.Error("LastSeen should be updated to current time")
	}
}

func TestValidateNode(t *testing.T) {
	bm := &BootstrapManager{}

	// Test valid node
	validNode := &BootstrapNode{
		Address:     "node1:9876",
		Description: "Valid Node",
		Region:      "Test Region",
		TrustScore:  0.8,
	}

	if err := bm.ValidateNode(validNode); err != nil {
		t.Errorf("Valid node should pass validation: %v", err)
	}

	// Test invalid nodes
	testCases := []struct {
		name string
		node *BootstrapNode
	}{
		{
			name: "empty address",
			node: &BootstrapNode{
				Address:     "",
				Description: "Test",
				Region:      "Test",
				TrustScore:  0.8,
			},
		},
		{
			name: "empty description",
			node: &BootstrapNode{
				Address:     "node1:9876",
				Description: "",
				Region:      "Test",
				TrustScore:  0.8,
			},
		},
		{
			name: "empty region",
			node: &BootstrapNode{
				Address:     "node1:9876",
				Description: "Test",
				Region:      "",
				TrustScore:  0.8,
			},
		},
		{
			name: "invalid trust score",
			node: &BootstrapNode{
				Address:     "node1:9876",
				Description: "Test",
				Region:      "Test",
				TrustScore:  1.5, // Invalid
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := bm.ValidateNode(tc.node); err == nil {
				t.Error("Invalid node should fail validation")
			}
		})
	}
}

func TestGetBootstrapStats(t *testing.T) {
	bm := &BootstrapManager{
		Nodes: []*BootstrapNode{
			{
				Address:  "beacon1:9876",
				IsBeacon: true,
				LastSeen: time.Now().Unix(),
			},
			{
				Address:  "beacon2:9876",
				IsBeacon: true,
				LastSeen: time.Now().Unix() - 7200, // 2 hours ago
			},
			{
				Address:  "mesh1:9876",
				IsBeacon: false,
				LastSeen: time.Now().Unix(),
			},
		},
	}

	stats := bm.GetBootstrapStats()

	if stats["total_nodes"] != 3 {
		t.Errorf("Expected 3 total nodes, got %v", stats["total_nodes"])
	}

	if stats["beacon_nodes"] != 2 {
		t.Errorf("Expected 2 beacon nodes, got %v", stats["beacon_nodes"])
	}

	if stats["mesh_nodes"] != 1 {
		t.Errorf("Expected 1 mesh node, got %v", stats["mesh_nodes"])
	}

	if stats["recent_nodes"] != 2 {
		t.Errorf("Expected 2 recent nodes, got %v", stats["recent_nodes"])
	}
}
