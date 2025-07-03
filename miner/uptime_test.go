package miner

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

func TestNewUptimeTracker(t *testing.T) {
	// Create temporary database file
	dbPath := "test_uptime.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create uptime tracker
	ut := NewUptimeTracker(wallet, storage, nil)

	// Test initial state
	if ut.wallet == nil {
		t.Error("Wallet is nil")
	}
	if ut.storage == nil {
		t.Error("Storage is nil")
	}
	if ut.startTime.IsZero() {
		t.Error("Start time is zero")
	}
	if len(ut.heartbeats) != 0 {
		t.Error("Initial heartbeats should be empty")
	}
}

func TestDefaultUptimeConfig(t *testing.T) {
	config := DefaultUptimeConfig()

	// Test configuration values
	if config.HeartbeatInterval != 1*time.Hour {
		t.Errorf("Expected heartbeat interval 1h, got %v", config.HeartbeatInterval)
	}
	if config.RewardInterval != 10*time.Minute {
		t.Errorf("Expected reward interval 10m, got %v", config.RewardInterval)
	}
	if config.DailyCap != 280000 {
		t.Errorf("Expected daily cap 280000, got %d", config.DailyCap)
	}
	if config.MinUptimePercent != 80.0 {
		t.Errorf("Expected min uptime 80%%, got %.2f", config.MinUptimePercent)
	}
}

func TestCalculateReward(t *testing.T) {
	// Create temporary database file
	dbPath := "test_reward.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create uptime tracker
	ut := NewUptimeTracker(wallet, storage, nil)

	// Test reward calculation for different node counts
	testCases := []struct {
		nodeCount int
		expected  int
	}{
		{1, 1120},
		{10, 1037},
		{100, 800},
		{500, 451},
		{1000, 280},
		{10000, 28}, // Should be around 28 (280000/10000)
	}

	for _, tc := range testCases {
		reward := ut.calculateReward(tc.nodeCount)
		if reward != tc.expected {
			t.Errorf("For %d nodes, expected reward %d, got %d", tc.nodeCount, tc.expected, reward)
		}
	}

	// Test edge cases
	if ut.calculateReward(0) != 0 {
		t.Error("Reward for 0 nodes should be 0")
	}
	if ut.calculateReward(-1) != 0 {
		t.Error("Reward for negative nodes should be 0")
	}
}

func TestCalculateUptimePercent(t *testing.T) {
	// Create temporary database file
	dbPath := "test_uptime_percent.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create uptime tracker
	ut := NewUptimeTracker(wallet, storage, nil)

	// Test with no heartbeats
	uptime := ut.calculateUptimePercent()
	if uptime != 0.0 {
		t.Errorf("Expected 0%% uptime with no heartbeats, got %.2f%%", uptime)
	}

	// Add some heartbeats for the last 24 hours
	now := time.Now()
	for i := 0; i < 24; i++ {
		// Add heartbeats for the last 24 hours (one per hour)
		heartbeatTime := now.Add(-time.Duration(i) * time.Hour)
		heartbeat := Heartbeat{
			Timestamp: heartbeatTime.Unix(),
			Signature: "test_signature",
			Hash:      "test_hash",
		}
		ut.heartbeats = append(ut.heartbeats, heartbeat)
	}

	// Test with 24 heartbeats (100% uptime)
	uptime = ut.calculateUptimePercent()
	if uptime < 95.0 || uptime > 105.0 { // Allow some tolerance
		t.Errorf("Expected ~100%% uptime with 24 heartbeats, got %.2f%%", uptime)
	}

	// Test with 12 heartbeats (~50% uptime)
	ut.heartbeats = ut.heartbeats[:12]
	uptime = ut.calculateUptimePercent()
	if uptime < 45.0 || uptime > 55.0 { // Allow some tolerance
		t.Errorf("Expected ~50%% uptime with 12 heartbeats, got %.2f%%", uptime)
	}
}

func TestMintCharacters(t *testing.T) {
	// Create temporary database file
	dbPath := "test_mint.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create uptime tracker
	ut := NewUptimeTracker(wallet, storage, nil)

	// Test initial balance
	balance, err := storage.GetCharacterBalance(wallet.GetAddress())
	if err != nil {
		t.Fatalf("Failed to get initial balance: %v", err)
	}
	if balance != 0 {
		t.Errorf("Expected initial balance 0, got %d", balance)
	}

	// Mint some characters
	amount := 1000
	if err := ut.mintCharacters(amount); err != nil {
		t.Fatalf("Failed to mint characters: %v", err)
	}

	// Check balance
	balance, err = storage.GetCharacterBalance(wallet.GetAddress())
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}
	if balance != amount {
		t.Errorf("Expected balance %d, got %d", amount, balance)
	}

	// Test invalid amount
	if err := ut.mintCharacters(0); err == nil {
		t.Error("Expected error for zero amount")
	}
	if err := ut.mintCharacters(-100); err == nil {
		t.Error("Expected error for negative amount")
	}
}

func TestGetUptimeInfo(t *testing.T) {
	// Create temporary database file
	dbPath := "test_info.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create uptime tracker
	ut := NewUptimeTracker(wallet, storage, nil)

	// Get uptime info
	info := ut.GetUptimeInfo()

	// Test required fields
	requiredFields := []string{
		"start_time",
		"last_reward",
		"heartbeat_count",
		"uptime_24h_percent",
		"uptime_total_percent",
		"character_balance",
		"heartbeat_interval",
		"reward_interval",
		"daily_cap",
		"min_uptime_percent",
	}

	for _, field := range requiredFields {
		if _, exists := info[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Test initial values
	if info["heartbeat_count"] != 0 {
		t.Errorf("Expected heartbeat_count 0, got %v", info["heartbeat_count"])
	}
	if info["character_balance"] != 0 {
		t.Errorf("Expected character_balance 0, got %v", info["character_balance"])
	}
	if info["daily_cap"] != 280000 {
		t.Errorf("Expected daily_cap 280000, got %v", info["daily_cap"])
	}
}

func TestHeartbeatLogging(t *testing.T) {
	// Create temporary database file
	dbPath := "test_heartbeat.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create uptime tracker
	ut := NewUptimeTracker(wallet, storage, nil)

	// Test heartbeat logging
	if err := ut.logHeartbeat(); err != nil {
		t.Fatalf("Failed to log heartbeat: %v", err)
	}

	// Check that heartbeat was added
	if len(ut.heartbeats) != 1 {
		t.Errorf("Expected 1 heartbeat, got %d", len(ut.heartbeats))
	}

	// Check heartbeat fields
	heartbeat := ut.heartbeats[0]
	if heartbeat.Timestamp <= 0 {
		t.Error("Heartbeat timestamp should be positive")
	}
	if heartbeat.Signature == "" {
		t.Error("Heartbeat signature should not be empty")
	}
	if heartbeat.Hash == "" {
		t.Error("Heartbeat hash should not be empty")
	}

	// Test heartbeat hash calculation
	expectedData := fmt.Sprintf("%s%d", wallet.GetAddress(), heartbeat.Timestamp)
	expectedHash := ut.calculateHeartbeatHash(expectedData)
	if heartbeat.Hash != expectedHash {
		t.Errorf("Heartbeat hash mismatch: expected %s, got %s", expectedHash, heartbeat.Hash)
	}
}

func TestDistributeRewards(t *testing.T) {
	// Create temporary database file
	dbPath := "test_distribute.db"
	defer os.Remove(dbPath)

	// Create storage
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create wallet
	wallet, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Create uptime tracker with shorter intervals for testing
	ut := NewUptimeTracker(wallet, storage, nil)
	ut.config.HeartbeatInterval = 1 * time.Hour // Keep default 1 hour interval
	ut.config.RewardInterval = 1 * time.Minute  // 1 minute for testing
	ut.config.MinUptimePercent = 50.0           // Lower threshold for testing

	// Add enough heartbeats to ensure uptime requirement is met
	// We need at least 50% of expected heartbeats in the last 24 hours
	// Expected: 24 heartbeats (one per hour for 24 hours)
	// We'll add 20 heartbeats (83.33% uptime, above 50% threshold)
	now := time.Now()
	for i := 0; i < 20; i++ { // Add 20 heartbeats (one per hour for 20 hours)
		heartbeatTime := now.Add(-time.Duration(i) * time.Hour)
		heartbeat := Heartbeat{
			Timestamp: heartbeatTime.Unix(),
			Signature: "test_signature",
			Hash:      "test_hash",
		}
		ut.heartbeats = append(ut.heartbeats, heartbeat)
	}

	// Get initial balance
	initialBalance, err := storage.GetCharacterBalance(wallet.GetAddress())
	if err != nil {
		t.Fatalf("Failed to get initial balance: %v", err)
	}

	// Distribute rewards
	if err := ut.distributeRewards(); err != nil {
		t.Fatalf("Failed to distribute rewards: %v", err)
	}

	// Check that balance increased
	finalBalance, err := storage.GetCharacterBalance(wallet.GetAddress())
	if err != nil {
		t.Fatalf("Failed to get final balance: %v", err)
	}

	if finalBalance <= initialBalance {
		t.Errorf("Balance should have increased: initial %d, final %d", initialBalance, finalBalance)
	}

	// Verify reward amount (should be daily reward / 144 for single node)
	// Daily reward for 1 node: 1120 characters
	// Batch reward: 1120 / 144 ≈ 7.78, but minimum is 1
	expectedDailyReward := 1120
	expectedBatchReward := expectedDailyReward / 144
	if expectedBatchReward < 1 {
		expectedBatchReward = 1
	}

	actualReward := finalBalance - initialBalance
	if actualReward != expectedBatchReward {
		t.Errorf("Expected batch reward %d, got %d (daily rate: %d chars/day)",
			expectedBatchReward, actualReward, expectedDailyReward)
	}
}
