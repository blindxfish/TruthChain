package main

import (
	"fmt"
	"time"

	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/wallet"
)

func main() {
	fmt.Println("=== TruthChain Chain Sync System Demo ===")
	fmt.Println()

	// Create wallets for different nodes
	node1Wallet, _ := wallet.NewWallet()
	node2Wallet, _ := wallet.NewWallet()
	beaconWallet, _ := wallet.NewWallet()

	fmt.Printf("Node 1 Address: %s\n", node1Wallet.GetAddress())
	fmt.Printf("Node 2 Address: %s\n", node2Wallet.GetAddress())
	fmt.Printf("Beacon Node Address: %s\n", beaconWallet.GetAddress())
	fmt.Println()

	// Create blockchain for Node 1
	blockchain1 := chain.NewBlockchain(3)
	_ = chain.NewChainSyncManager(blockchain1, node1Wallet.GetAddress())

	// Create some posts
	post1, _ := blockchain1.CreatePost("Hello from Node 1!", node1Wallet)
	post2, _ := blockchain1.CreatePost("This is a test post.", node1Wallet)
	post3, _ := blockchain1.CreatePost("Building the decentralized future.", node1Wallet)

	// Add posts to blockchain
	blockchain1.AddPost(*post1)
	blockchain1.AddPost(*post2)
	blockchain1.AddPost(*post3)

	// Create a beacon announcement (simplified for demo)
	beaconAnnounce := &chain.BeaconAnnounce{
		NodeID:    beaconWallet.GetAddress(),
		IP:        "beacon.truthchain.org",
		Port:      9876,
		Timestamp: time.Now().Unix(),
		Uptime:    99.5,
		Version:   "v1.0.0",
		Sig:       "test_signature_for_demo",
	}

	// Create a block with beacon announcement
	block := chain.CreateBlockWithBeacon(
		1,
		blockchain1.GetLatestBlock().Hash,
		blockchain1.PendingPosts,
		[]chain.Transfer{},
		nil,
		beaconAnnounce,
	)

	// Add block to blockchain
	blockchain1.Blocks = append(blockchain1.Blocks, block)
	blockchain1.PendingPosts = []chain.Post{}

	fmt.Println("=== Node 1 Blockchain ===")
	fmt.Printf("Chain length: %d\n", len(blockchain1.Blocks))
	fmt.Printf("Latest block hash: %s\n", blockchain1.GetLatestBlock().Hash)

	if blockchain1.GetLatestBlock().BeaconAnnounce != nil {
		beacon := blockchain1.GetLatestBlock().BeaconAnnounce
		fmt.Printf("Beacon announcement: %s:%d (uptime: %.1f%%)\n",
			beacon.IP, beacon.Port, beacon.Uptime)
	}
	fmt.Println()

	// Create blockchain for Node 2 (new node joining the network)
	blockchain2 := chain.NewBlockchain(3)
	syncManager2 := chain.NewChainSyncManager(blockchain2, node2Wallet.GetAddress())

	fmt.Println("=== Node 2 (New Node) ===")
	fmt.Printf("Initial chain length: %d\n", len(blockchain2.Blocks))
	fmt.Printf("Latest block hash: %s\n", blockchain2.GetLatestBlock().Hash)
	fmt.Println()

	// Node 2 discovers beacons from Node 1's blockchain
	fmt.Println("=== Beacon Discovery ===")
	// Create a temporary sync manager that uses Node 1's blockchain for discovery
	tempSyncManager := chain.NewChainSyncManager(blockchain1, node2Wallet.GetAddress())
	beacons, err := tempSyncManager.DiscoverBeaconsFromChain(100)
	if err != nil {
		fmt.Printf("Error discovering beacons: %v\n", err)
	} else {
		fmt.Printf("Discovered %d beacons\n", len(beacons))
		for i, beacon := range beacons {
			fmt.Printf("  Beacon %d: %s:%d (uptime: %.1f%%)\n",
				i+1, beacon.IP, beacon.Port, beacon.Uptime)
		}
	}
	fmt.Println()

	// Simulate Node 2 syncing from discovered beacons
	fmt.Println("=== Chain Synchronization ===")
	for _, beacon := range beacons {
		fmt.Printf("Attempting to sync from beacon %s:%d...\n", beacon.IP, beacon.Port)

		// Add beacon as peer
		syncManager2.AddPeer(beacon.NodeID, beacon.IP, beacon.Port)

		// Simulate successful connection
		syncManager2.UpdatePeerReachability(beacon.NodeID, true)

		// Simulate syncing blocks
		err := syncManager2.SyncFromPeer(beacon.IP, beacon.Port, 0)
		if err != nil {
			fmt.Printf("  Sync failed: %v\n", err)
		} else {
			fmt.Printf("  Sync successful from %s:%d\n", beacon.IP, beacon.Port)
		}
	}
	fmt.Println()

	// Show peer statistics
	fmt.Println("=== Peer Statistics ===")
	stats := syncManager2.GetSyncStats()
	fmt.Printf("Total peers: %d\n", stats["total_peers"])
	fmt.Printf("Reachable peers: %d\n", stats["reachable_peers"])
	fmt.Printf("Average trust score: %.2f\n", stats["average_trust_score"])
	fmt.Printf("Node ID: %s\n", stats["node_id"])
	fmt.Println()

	// Show reachable peers
	reachablePeers := syncManager2.GetReachablePeers()
	fmt.Printf("Reachable peers (%d):\n", len(reachablePeers))
	for _, peer := range reachablePeers {
		fmt.Printf("  %s (%s:%d) - Trust: %.2f\n",
			peer.NodeID, peer.IP, peer.Port, peer.TrustScore)
	}
	fmt.Println()

	// Test beacon announcement validation
	fmt.Println("=== Beacon Validation ===")
	for _, beacon := range beacons {
		if err := beacon.ValidateBeaconAnnounce(); err != nil {
			fmt.Printf("Invalid beacon %s: %v\n", beacon.NodeID, err)
		} else {
			fmt.Printf("Valid beacon: %s\n", beacon.NodeID)
		}
	}
	fmt.Println()

	// Test cleanup of old peers
	fmt.Println("=== Peer Cleanup ===")
	removed := syncManager2.CleanupOldPeers(24 * time.Hour)
	fmt.Printf("Removed %d old peers\n", removed)

	// Add a peer for cleanup test
	syncManager2.AddPeer("old_peer", "192.168.1.200", 8080)

	removed = syncManager2.CleanupOldPeers(24 * time.Hour)
	fmt.Printf("Removed %d old peers after cleanup\n", removed)
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
	fmt.Println("The chain sync system provides:")
	fmt.Println("1. Beacon discovery from blockchain")
	fmt.Println("2. Peer management with trust scoring")
	fmt.Println("3. Reachability tracking")
	fmt.Println("4. Automatic cleanup of stale peers")
	fmt.Println("5. Chain synchronization protocol")
}
