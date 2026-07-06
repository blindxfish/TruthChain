package miner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

// BeaconChecker interface for checking beacon status
type BeaconChecker interface {
	IsBeaconMode() bool
	GetBeaconUptime() float64
}

// UptimeTracker manages node uptime tracking and character rewards.
//
// NOTE: reward minting here is a LOCAL balance credit, not yet validated by
// network consensus — a node operator can always mint to themselves at the
// capped rate using their own key. The hardening below prevents forged
// heartbeats (from DB tampering without the key), double/rapid claims, and
// always-max-reward, but consensus-validated issuance remains future work.
type UptimeTracker struct {
	wallet            *wallet.Wallet
	storage           store.Storage
	beaconChecker     BeaconChecker    // Beacon checker for incentive calculation
	nodeCountProvider func() int       // Returns the observed network node count
	mu                sync.RWMutex
	startTime         time.Time
	lastReward        time.Time
	heartbeats        []Heartbeat
	config            UptimeConfig
	stopChan          chan struct{}
	stopOnce          sync.Once
}

// UptimeConfig contains configuration for the uptime tracker
type UptimeConfig struct {
	HeartbeatInterval time.Duration // How often to log heartbeats
	RewardInterval    time.Duration // How often to distribute rewards (24h)
	DailyCap          int           // Total characters per day (280,000)
	MinUptimePercent  float64       // Minimum uptime required for rewards
}

// Heartbeat represents a single uptime heartbeat
type Heartbeat struct {
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
	Hash      string `json:"hash"`
}

// DefaultUptimeConfig returns the default configuration for mainnet
func DefaultUptimeConfig() UptimeConfig {
	return UptimeConfig{
		HeartbeatInterval: 1 * time.Hour,    // Log heartbeat every hour
		RewardInterval:    10 * time.Minute, // Distribute rewards every 10 minutes (like Bitcoin)
		DailyCap:          280000,           // 280,000 characters per day
		MinUptimePercent:  80.0,             // 80% uptime required
	}
}

// NewUptimeTracker creates a new uptime tracker
func NewUptimeTracker(w *wallet.Wallet, s store.Storage, beaconChecker BeaconChecker) *UptimeTracker {
	return &UptimeTracker{
		wallet:        w,
		storage:       s,
		beaconChecker: beaconChecker,
		startTime:     time.Now(),
		lastReward:    time.Now(),
		heartbeats:    []Heartbeat{},
		config:        DefaultUptimeConfig(),
		stopChan:      make(chan struct{}),
	}
}

// SetNodeCountProvider wires a function that reports the current observed
// network size (e.g. connected peers + self). The reward-per-node decays with
// this count; without it the node conservatively assumes it is alone.
func (ut *UptimeTracker) SetNodeCountProvider(provider func() int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.nodeCountProvider = provider
}

// Start begins the uptime tracking process
func (ut *UptimeTracker) Start() error {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Load existing heartbeats from storage
	if err := ut.LoadHeartbeats(); err != nil {
		return fmt.Errorf("failed to load heartbeats: %w", err)
	}

	// Start heartbeat logging
	go ut.heartbeatLoop()

	// Start reward distribution
	go ut.rewardLoop()

	return nil
}

// heartbeatLoop continuously logs heartbeats until stopped.
func (ut *UptimeTracker) heartbeatLoop() {
	ticker := time.NewTicker(ut.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ut.stopChan:
			return
		case <-ticker.C:
			if err := ut.logHeartbeat(); err != nil {
				fmt.Printf("Failed to log heartbeat: %v\n", err)
			}
		}
	}
}

// rewardLoop handles reward distribution until stopped.
func (ut *UptimeTracker) rewardLoop() {
	ticker := time.NewTicker(ut.config.RewardInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ut.stopChan:
			return
		case <-ticker.C:
			if err := ut.distributeRewards(); err != nil {
				fmt.Printf("Failed to distribute rewards: %v\n", err)
			}
		}
	}
}

// logHeartbeat creates and stores a new heartbeat
func (ut *UptimeTracker) logHeartbeat() error {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Create heartbeat data
	timestamp := time.Now().Unix()
	heartbeatData := fmt.Sprintf("%s%d", ut.wallet.GetAddress(), timestamp)

	// Sign the heartbeat
	signature, err := ut.wallet.Sign([]byte(heartbeatData))
	if err != nil {
		return fmt.Errorf("failed to sign heartbeat: %w", err)
	}

	// Create heartbeat
	heartbeat := Heartbeat{
		Timestamp: timestamp,
		Signature: hex.EncodeToString(signature),
		Hash:      ut.calculateHeartbeatHash(heartbeatData),
	}

	// Add to heartbeats and bound in-memory growth
	ut.heartbeats = append(ut.heartbeats, heartbeat)
	ut.pruneHeartbeats()

	// Save to storage
	if err := ut.saveHeartbeat(heartbeat); err != nil {
		return fmt.Errorf("failed to save heartbeat: %w", err)
	}

	fmt.Printf("Heartbeat logged: %s\n", time.Unix(timestamp, 0).Format("2006-01-02 15:04:05"))
	return nil
}

// calculateHeartbeatHash calculates the hash of a heartbeat
func (ut *UptimeTracker) calculateHeartbeatHash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// saveHeartbeat saves a heartbeat to storage
func (ut *UptimeTracker) saveHeartbeat(heartbeat Heartbeat) error {
	// Convert to JSON
	data, err := json.Marshal(heartbeat)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	// Save to storage
	if err := ut.storage.SaveHeartbeat(data); err != nil {
		return fmt.Errorf("failed to save heartbeat to storage: %w", err)
	}

	return nil
}

// LoadHeartbeats loads heartbeats from storage, discarding any whose signature
// does not verify against this node's own wallet key. This prevents fabricated
// heartbeats injected into the database (without the key) from inflating the
// node's measured uptime and thus its reward.
func (ut *UptimeTracker) LoadHeartbeats() error {
	// Load from storage
	heartbeatData, err := ut.storage.GetHeartbeats()
	if err != nil {
		return fmt.Errorf("failed to load heartbeats from storage: %w", err)
	}

	// Parse and authenticate heartbeats
	ut.heartbeats = []Heartbeat{}
	skipped := 0
	for _, data := range heartbeatData {
		var heartbeat Heartbeat
		if err := json.Unmarshal(data, &heartbeat); err != nil {
			skipped++
			continue
		}
		if err := ut.verifyHeartbeat(heartbeat); err != nil {
			skipped++
			continue
		}
		ut.heartbeats = append(ut.heartbeats, heartbeat)
	}
	if skipped > 0 {
		fmt.Printf("Discarded %d invalid/unauthenticated heartbeats on load\n", skipped)
	}

	ut.pruneHeartbeats()
	return nil
}

// verifyHeartbeat checks that a heartbeat's hash and signature are consistent
// with this node's wallet. Heartbeats are the node's own uptime proof, so a
// valid one must be signed by this node's key.
func (ut *UptimeTracker) verifyHeartbeat(hb Heartbeat) error {
	if hb.Timestamp <= 0 {
		return fmt.Errorf("invalid heartbeat timestamp")
	}
	data := fmt.Sprintf("%s%d", ut.wallet.GetAddress(), hb.Timestamp)

	if hb.Hash != ut.calculateHeartbeatHash(data) {
		return fmt.Errorf("heartbeat hash mismatch")
	}

	sigBytes, err := hex.DecodeString(hb.Signature)
	if err != nil {
		return fmt.Errorf("invalid heartbeat signature encoding: %w", err)
	}
	ok, err := ut.wallet.Verify([]byte(data), sigBytes)
	if err != nil {
		return fmt.Errorf("heartbeat signature verification failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("heartbeat signature does not match node wallet")
	}
	return nil
}

// pruneHeartbeats drops in-memory heartbeats older than the reward window (24h
// plus a small margin) so the slice cannot grow without bound over long uptime.
// Caller must hold ut.mu (or be in a single-threaded load path).
func (ut *UptimeTracker) pruneHeartbeats() {
	cutoff := time.Now().Add(-25 * time.Hour).Unix()
	kept := ut.heartbeats[:0]
	for _, hb := range ut.heartbeats {
		if hb.Timestamp >= cutoff {
			kept = append(kept, hb)
		}
	}
	ut.heartbeats = kept
}

// distributeRewards calculates and distributes character rewards every 10 minutes
func (ut *UptimeTracker) distributeRewards() error {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Enforce the reward cadence by wall-clock, not just ticker frequency: this
	// prevents rapid/duplicate claims if distributeRewards is invoked more often
	// than RewardInterval (e.g. restart loops or extra calls). A small slack
	// absorbs ticker jitter.
	if elapsed := time.Since(ut.lastReward); elapsed < ut.config.RewardInterval-time.Second {
		return nil
	}

	// Calculate uptime percentage for the last 24 hours
	uptimePercent := ut.calculateUptimePercent()
	if uptimePercent < ut.config.MinUptimePercent {
		fmt.Printf("Uptime too low for rewards: %.2f%% (minimum: %.2f%%)\n",
			uptimePercent, ut.config.MinUptimePercent)
		return nil
	}

	// Reward per node decays with the observed network size. Fall back to a
	// single-node assumption only when no provider is wired.
	nodeCount := 1
	if ut.nodeCountProvider != nil {
		if n := ut.nodeCountProvider(); n > 0 {
			nodeCount = n
		}
	}
	dailyReward := ut.calculateReward(nodeCount)

	// Distribute daily reward across 144 batches (every 10 minutes)
	// 24 hours × 6 batches per hour = 144 batches per day
	batchReward := dailyReward / 144

	// Ensure minimum reward of 1 character per batch
	if batchReward < 1 {
		batchReward = 1
	}

	// Apply beacon bonus if this node is a beacon
	beaconBonus := 0
	isBeacon := false
	if ut.beaconChecker != nil {
		isBeacon = ut.beaconChecker.IsBeaconMode()
		if isBeacon {
			// +50% bonus for beacon nodes as specified in NetworkDesign.txt
			beaconBonus = batchReward / 2 // 50% of base reward
			batchReward += beaconBonus
			fmt.Printf("Beacon bonus applied: +%d characters (50%% bonus)\n", beaconBonus)
		}
	}

	// Mint characters to the wallet
	if err := ut.mintCharacters(batchReward); err != nil {
		return fmt.Errorf("failed to mint characters: %w", err)
	}

	bonusInfo := ""
	if isBeacon {
		bonusInfo = fmt.Sprintf(" (includes +%d beacon bonus)", beaconBonus)
	}

	fmt.Printf("Reward distributed: %d characters%s (uptime: %.2f%%, daily rate: %d chars/day)\n",
		batchReward, bonusInfo, uptimePercent, dailyReward)
	ut.lastReward = time.Now()

	return nil
}

// calculateUptimePercent calculates the uptime percentage for the last 24 hours
func (ut *UptimeTracker) calculateUptimePercent() float64 {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	// Count heartbeats in the last 24 hours
	heartbeatCount := 0
	for _, hb := range ut.heartbeats {
		hbTime := time.Unix(hb.Timestamp, 0)
		if hbTime.After(dayAgo) && hbTime.Before(now) {
			heartbeatCount++
		}
	}

	// Expected heartbeats: 24 hours / heartbeat interval
	expectedHeartbeats := 24.0 / ut.config.HeartbeatInterval.Hours()

	if expectedHeartbeats == 0 {
		return 0.0
	}

	uptimePercent := (float64(heartbeatCount) / expectedHeartbeats) * 100.0
	if uptimePercent > 100.0 {
		uptimePercent = 100.0
	}

	return uptimePercent
}

// calculateReward calculates the daily reward based on node count
func (ut *UptimeTracker) calculateReward(nodeCount int) int {
	if nodeCount <= 0 {
		return 0
	}

	// Logarithmic decay formula from whitepaper
	// For nodeCount = 1: reward = 1120
	// For nodeCount = 1000: reward = 280
	// For nodeCount > 1000: reward decreases further

	if nodeCount == 1 {
		return 1120
	} else if nodeCount <= 1000 {
		// Linear interpolation between known points
		if nodeCount <= 10 {
			// Between 1 and 10 nodes
			ratio := float64(nodeCount-1) / 9.0
			return int(1120 - ratio*(1120-1037))
		} else if nodeCount <= 100 {
			// Between 10 and 100 nodes
			ratio := float64(nodeCount-10) / 90.0
			return int(1037 - ratio*(1037-800))
		} else if nodeCount <= 500 {
			// Between 100 and 500 nodes
			ratio := float64(nodeCount-100) / 400.0
			return int(800 - ratio*(800-451))
		} else {
			// Between 500 and 1000 nodes
			ratio := float64(nodeCount-500) / 500.0
			return int(451 - ratio*(451-280))
		}
	} else {
		// For more than 1000 nodes, use logarithmic decay
		// This ensures we never exceed the daily cap
		reward := float64(ut.config.DailyCap) / float64(nodeCount)
		if reward < 1 {
			reward = 1
		}
		return int(reward)
	}
}

// mintCharacters adds characters to the wallet balance
func (ut *UptimeTracker) mintCharacters(amount int) error {
	if amount <= 0 {
		return fmt.Errorf("invalid character amount: %d", amount)
	}

	// Update balance in storage
	if err := ut.storage.UpdateCharacterBalance(ut.wallet.GetAddress(), amount); err != nil {
		return fmt.Errorf("failed to update character balance: %w", err)
	}

	return nil
}

// GetUptimeInfo returns information about the node's uptime
func (ut *UptimeTracker) GetUptimeInfo() map[string]interface{} {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	uptimePercent := ut.calculateUptimePercent()
	heartbeatCount := len(ut.heartbeats)

	// Calculate total uptime since start
	totalUptime := time.Since(ut.startTime)
	expectedHeartbeats := totalUptime.Hours() / ut.config.HeartbeatInterval.Hours()
	totalUptimePercent := 0.0
	if expectedHeartbeats > 0 {
		totalUptimePercent = (float64(heartbeatCount) / expectedHeartbeats) * 100.0
		if totalUptimePercent > 100.0 {
			totalUptimePercent = 100.0
		}
	}

	// Get current balance
	balance, err := ut.storage.GetCharacterBalance(ut.wallet.GetAddress())
	if err != nil {
		balance = 0
	}

	// Check beacon status
	isBeacon := false
	beaconUptime := 0.0
	if ut.beaconChecker != nil {
		isBeacon = ut.beaconChecker.IsBeaconMode()
		beaconUptime = ut.beaconChecker.GetBeaconUptime()
	}

	// Calculate beacon bonus rate
	beaconBonusRate := 0.0
	if isBeacon {
		beaconBonusRate = 50.0 // +50% bonus as specified in NetworkDesign.txt
	}

	return map[string]interface{}{
		"start_time":           ut.startTime.Format("2006-01-02 15:04:05"),
		"last_reward":          ut.lastReward.Format("2006-01-02 15:04:05"),
		"heartbeat_count":      heartbeatCount,
		"uptime_24h_percent":   uptimePercent,
		"uptime_total_percent": totalUptimePercent,
		"character_balance":    balance,
		"heartbeat_interval":   ut.config.HeartbeatInterval.String(),
		"reward_interval":      ut.config.RewardInterval.String(),
		"daily_cap":            ut.config.DailyCap,
		"min_uptime_percent":   ut.config.MinUptimePercent,
		"is_beacon":            isBeacon,
		"beacon_uptime":        beaconUptime,
		"beacon_bonus_rate":    beaconBonusRate,
	}
}

// Stop stops the uptime tracker's background goroutines. Safe to call more than
// once.
func (ut *UptimeTracker) Stop() {
	ut.stopOnce.Do(func() {
		close(ut.stopChan)
	})
	fmt.Println("Uptime tracker stopped")
}
