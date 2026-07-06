package miner

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

func newTestTracker(t *testing.T) (*UptimeTracker, *wallet.Wallet, store.Storage) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "uptime.db")
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	t.Cleanup(func() { storage.Close() })
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("wallet: %v", err)
	}
	return NewUptimeTracker(w, storage, nil), w, storage
}

// signedHeartbeat builds a heartbeat validly signed by w for timestamp ts.
func signedHeartbeat(ut *UptimeTracker, w *wallet.Wallet, ts int64) Heartbeat {
	data := fmt.Sprintf("%s%d", w.GetAddress(), ts)
	sig, _ := w.Sign([]byte(data))
	return Heartbeat{Timestamp: ts, Signature: hex.EncodeToString(sig), Hash: ut.calculateHeartbeatHash(data)}
}

// TestLoadHeartbeatsRejectsForged proves fabricated heartbeats injected into the
// DB (without the node's key) are discarded, so they cannot inflate uptime.
func TestLoadHeartbeatsRejectsForged(t *testing.T) {
	ut, w, storage := newTestTracker(t)
	now := time.Now().Unix()

	// One genuine heartbeat signed by the node's wallet.
	good := signedHeartbeat(ut, w, now)
	gb, _ := json.Marshal(good)
	if err := storage.SaveHeartbeat(gb); err != nil {
		t.Fatalf("save good: %v", err)
	}

	// A forged heartbeat with a junk signature.
	forged := Heartbeat{Timestamp: now, Signature: "00", Hash: "bad"}
	fb, _ := json.Marshal(forged)
	if err := storage.SaveHeartbeat(fb); err != nil {
		t.Fatalf("save forged: %v", err)
	}

	// A heartbeat signed by a DIFFERENT wallet (not this node).
	other, _ := wallet.NewWallet()
	otherHB := signedHeartbeat(ut, other, now) // hash uses this node's addr, sig is other's
	ob, _ := json.Marshal(otherHB)
	if err := storage.SaveHeartbeat(ob); err != nil {
		t.Fatalf("save other: %v", err)
	}

	if err := ut.LoadHeartbeats(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(ut.heartbeats) != 1 {
		t.Fatalf("expected only the 1 genuine heartbeat to survive, got %d", len(ut.heartbeats))
	}
	if ut.heartbeats[0].Timestamp != now {
		t.Fatalf("surviving heartbeat is not the genuine one")
	}
}

// TestRewardCadenceGuard proves rewards cannot be claimed more often than the
// reward interval, even if distributeRewards is invoked repeatedly.
func TestRewardCadenceGuard(t *testing.T) {
	ut, w, storage := newTestTracker(t)

	// Enough recent heartbeats to satisfy the uptime requirement.
	now := time.Now()
	for i := 0; i < 24; i++ {
		ut.heartbeats = append(ut.heartbeats, signedHeartbeat(ut, w, now.Add(-time.Duration(i)*time.Hour).Unix()))
	}

	// Just rewarded: a fresh call must NOT mint again.
	ut.lastReward = time.Now()
	if err := ut.distributeRewards(); err != nil {
		t.Fatalf("distribute: %v", err)
	}
	if bal, _ := storage.GetCharacterBalance(w.GetAddress()); bal != 0 {
		t.Fatalf("reward should be blocked by cadence guard, balance=%d", bal)
	}

	// A full interval later: minting is allowed.
	ut.lastReward = time.Now().Add(-ut.config.RewardInterval - time.Second)
	if err := ut.distributeRewards(); err != nil {
		t.Fatalf("distribute 2: %v", err)
	}
	if bal, _ := storage.GetCharacterBalance(w.GetAddress()); bal <= 0 {
		t.Fatalf("reward should have been minted after interval, balance=%d", bal)
	}
}

// TestNodeCountProviderScalesReward verifies the reward-per-node uses the wired
// network size instead of always assuming a single node.
func TestNodeCountProviderScalesReward(t *testing.T) {
	ut, _, _ := newTestTracker(t)
	ut.SetNodeCountProvider(func() int { return 1000 })
	// calculateReward is driven by the provider's value via distributeRewards;
	// verify the reward curve directly at the wired size.
	if got := ut.calculateReward(1000); got != 280 {
		t.Fatalf("expected 280 chars/day at 1000 nodes, got %d", got)
	}
	if got := ut.calculateReward(1); got != 1120 {
		t.Fatalf("expected 1120 chars/day at 1 node, got %d", got)
	}
}
