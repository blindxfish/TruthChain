package network

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// BootstrapNode represents a known mainnet node
type BootstrapNode struct {
	Address     string  `json:"address"`     // IP:port
	Description string  `json:"description"` // Human-readable description
	Region      string  `json:"region"`      // Geographic region
	IsBeacon    bool    `json:"is_beacon"`   // Whether this is a beacon node
	TrustScore  float64 `json:"trust_score"` // Initial trust score
	LastSeen    int64   `json:"last_seen"`   // Last successful connection
}

// BootstrapManager manages the list of known mainnet nodes
type BootstrapManager struct {
	Nodes      []*BootstrapNode `json:"nodes"`
	ConfigFile string           // Path to bootstrap config file
	mu         sync.RWMutex
}

// NewBootstrapManager creates a new bootstrap manager
func NewBootstrapManager(configFile string) *BootstrapManager {
	bm := &BootstrapManager{
		Nodes:      make([]*BootstrapNode, 0),
		ConfigFile: configFile,
	}

	// Load default bootstrap nodes if config doesn't exist or is empty
	if err := bm.LoadConfig(); err != nil {
		log.Printf("Failed to load bootstrap config, using defaults: %v", err)
		bm.loadDefaultNodes()
		bm.SaveConfig()
	} else if len(bm.Nodes) == 0 {
		log.Printf("Bootstrap config is empty, loading defaults")
		bm.loadDefaultNodes()
		bm.SaveConfig()
	}

	return bm
}

// loadDefaultNodes loads the default list of mainnet nodes
func (bm *BootstrapManager) loadDefaultNodes() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Default mainnet nodes (these would be real nodes in production)
	bm.Nodes = []*BootstrapNode{
		{
			Address:     "mainnet.truthchain.org:9876",
			Description: "Official TruthChain Mainnet Node",
			Region:      "Global",
			IsBeacon:    true,
			TrustScore:  0.9,
			LastSeen:    0,
		},
		{
			Address:     "beacon1.truthchain.org:9876",
			Description: "TruthChain Beacon Node 1",
			Region:      "North America",
			IsBeacon:    true,
			TrustScore:  0.8,
			LastSeen:    0,
		},
		{
			Address:     "beacon2.truthchain.org:9876",
			Description: "TruthChain Beacon Node 2",
			Region:      "Europe",
			IsBeacon:    true,
			TrustScore:  0.8,
			LastSeen:    0,
		},
		{
			Address:     "beacon3.truthchain.org:9876",
			Description: "TruthChain Beacon Node 3",
			Region:      "Asia",
			IsBeacon:    true,
			TrustScore:  0.8,
			LastSeen:    0,
		},
		{
			Address:     "node1.truthchain.org:9876",
			Description: "TruthChain Mesh Node 1",
			Region:      "North America",
			IsBeacon:    false,
			TrustScore:  0.7,
			LastSeen:    0,
		},
		{
			Address:     "node2.truthchain.org:9876",
			Description: "TruthChain Mesh Node 2",
			Region:      "Europe",
			IsBeacon:    false,
			TrustScore:  0.7,
			LastSeen:    0,
		},
		{
			Address:     "node3.truthchain.org:9876",
			Description: "TruthChain Mesh Node 3",
			Region:      "Asia",
			IsBeacon:    false,
			TrustScore:  0.7,
			LastSeen:    0,
		},
	}
}

// LoadConfig loads bootstrap nodes from config file
func (bm *BootstrapManager) LoadConfig() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	data, err := os.ReadFile(bm.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read bootstrap config: %w", err)
	}

	if err := json.Unmarshal(data, &bm.Nodes); err != nil {
		return fmt.Errorf("failed to parse bootstrap config: %w", err)
	}

	log.Printf("Loaded %d bootstrap nodes from %s", len(bm.Nodes), bm.ConfigFile)
	return nil
}

// SaveConfig saves bootstrap nodes to config file
func (bm *BootstrapManager) SaveConfig() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	data, err := json.MarshalIndent(bm.Nodes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal bootstrap config: %w", err)
	}

	if err := os.WriteFile(bm.ConfigFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write bootstrap config: %w", err)
	}

	log.Printf("Saved %d bootstrap nodes to %s", len(bm.Nodes), bm.ConfigFile)
	return nil
}

// AddNode adds a new bootstrap node
func (bm *BootstrapManager) AddNode(address, description, region string, isBeacon bool, trustScore float64) error {
	var updated bool
	bm.mu.Lock()
	// Check if node already exists
	for _, node := range bm.Nodes {
		if node.Address == address {
			// Update existing node
			node.Description = description
			node.Region = region
			node.IsBeacon = isBeacon
			node.TrustScore = trustScore
			node.LastSeen = time.Now().Unix()
			updated = true
			break
		}
	}
	if !updated {
		// Add new node
		newNode := &BootstrapNode{
			Address:     address,
			Description: description,
			Region:      region,
			IsBeacon:    isBeacon,
			TrustScore:  trustScore,
			LastSeen:    time.Now().Unix(),
		}
		bm.Nodes = append(bm.Nodes, newNode)
	}
	bm.mu.Unlock()
	return bm.SaveConfig()
}

// RemoveNode removes a bootstrap node
func (bm *BootstrapManager) RemoveNode(address string) error {
	bm.mu.Lock()
	found := false
	for i, node := range bm.Nodes {
		if node.Address == address {
			bm.Nodes = append(bm.Nodes[:i], bm.Nodes[i+1:]...)
			found = true
			break
		}
	}
	bm.mu.Unlock()
	if found {
		return bm.SaveConfig()
	}
	return fmt.Errorf("bootstrap node not found: %s", address)
}

// GetNodes returns all bootstrap nodes
func (bm *BootstrapManager) GetNodes() []*BootstrapNode {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	nodes := make([]*BootstrapNode, len(bm.Nodes))
	copy(nodes, bm.Nodes)
	return nodes
}

// GetBeaconNodes returns only beacon nodes
func (bm *BootstrapManager) GetBeaconNodes() []*BootstrapNode {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var beacons []*BootstrapNode
	for _, node := range bm.Nodes {
		if node.IsBeacon {
			beacons = append(beacons, node)
		}
	}
	return beacons
}

// GetNodesByRegion returns nodes from a specific region
func (bm *BootstrapManager) GetNodesByRegion(region string) []*BootstrapNode {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var regionalNodes []*BootstrapNode
	for _, node := range bm.Nodes {
		if node.Region == region {
			regionalNodes = append(regionalNodes, node)
		}
	}
	return regionalNodes
}

// UpdateLastSeen updates the last seen timestamp for a node
func (bm *BootstrapManager) UpdateLastSeen(address string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for _, node := range bm.Nodes {
		if node.Address == address {
			node.LastSeen = time.Now().Unix()
			break
		}
	}
}

// Bootstrap attempts to connect to bootstrap nodes
func (bm *BootstrapManager) Bootstrap(trustNetwork *TrustNetwork, maxAttempts int) error {
	log.Printf("Starting bootstrap process with %d nodes", len(bm.Nodes))

	bm.mu.RLock()
	nodes := make([]*BootstrapNode, len(bm.Nodes))
	copy(nodes, bm.Nodes)
	bm.mu.RUnlock()

	successCount := 0
	attemptCount := 0

	// Try beacon nodes first
	for _, node := range nodes {
		if node.IsBeacon && attemptCount < maxAttempts {
			attemptCount++
			log.Printf("Attempting to connect to beacon node: %s (%s)", node.Address, node.Description)

			if err := bm.attemptConnection(node, trustNetwork); err != nil {
				log.Printf("Failed to connect to beacon node %s: %v", node.Address, err)
			} else {
				successCount++
				bm.UpdateLastSeen(node.Address)
				log.Printf("Successfully connected to beacon node: %s", node.Address)
			}
		}
	}

	// Then try regular nodes
	for _, node := range nodes {
		if !node.IsBeacon && attemptCount < maxAttempts {
			attemptCount++
			log.Printf("Attempting to connect to mesh node: %s (%s)", node.Address, node.Description)

			if err := bm.attemptConnection(node, trustNetwork); err != nil {
				log.Printf("Failed to connect to mesh node %s: %v", node.Address, err)
			} else {
				successCount++
				bm.UpdateLastSeen(node.Address)
				log.Printf("Successfully connected to mesh node: %s", node.Address)
			}
		}
	}

	log.Printf("Bootstrap completed: %d/%d successful connections", successCount, attemptCount)
	return nil
}

// attemptConnection attempts to connect to a single bootstrap node
func (bm *BootstrapManager) attemptConnection(node *BootstrapNode, trustNetwork *TrustNetwork) error {
	// Add to peer table
	trustNetwork.PeerTable.AddPeer(node.Address, 1, "", node.TrustScore)

	// Mark as beacon if applicable
	if peer, exists := trustNetwork.PeerTable.GetPeer(node.Address); exists {
		peer.IsBeacon = node.IsBeacon
	}

	// Try to establish mesh connection
	if trustNetwork.MeshManager != nil {
		// This will attempt to establish a TCP connection
		go trustNetwork.MeshManager.establishConnection(node.Address)
	}

	return nil
}

// GetBootstrapStats returns statistics about bootstrap nodes
func (bm *BootstrapManager) GetBootstrapStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	totalNodes := len(bm.Nodes)
	beaconNodes := 0
	recentNodes := 0
	now := time.Now().Unix()

	for _, node := range bm.Nodes {
		if node.IsBeacon {
			beaconNodes++
		}
		if now-node.LastSeen < 3600 { // Seen in last hour
			recentNodes++
		}
	}

	return map[string]interface{}{
		"total_nodes":  totalNodes,
		"beacon_nodes": beaconNodes,
		"mesh_nodes":   totalNodes - beaconNodes,
		"recent_nodes": recentNodes,
		"config_file":  bm.ConfigFile,
	}
}

// ValidateNode validates a bootstrap node configuration
func (bm *BootstrapManager) ValidateNode(node *BootstrapNode) error {
	if node.Address == "" {
		return fmt.Errorf("node address cannot be empty")
	}

	if node.Description == "" {
		return fmt.Errorf("node description cannot be empty")
	}

	if node.Region == "" {
		return fmt.Errorf("node region cannot be empty")
	}

	if node.TrustScore < 0 || node.TrustScore > 1 {
		return fmt.Errorf("trust score must be between 0 and 1")
	}

	return nil
}
