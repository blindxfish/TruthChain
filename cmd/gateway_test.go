package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

func newGatewayNode(t *testing.T, faucetEnabled bool) *TruthChainNode {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gw.db")
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	t.Cleanup(func() { storage.Close() })
	bc, err := blockchain.NewBlockchain(storage, 1000, "truthchain-local")
	if err != nil {
		t.Fatalf("blockchain: %v", err)
	}
	w, _ := wallet.NewWallet()
	bc.UpdateWalletState(w.GetAddress(), 200000, 0) // fund the operator wallet generously
	cfg := &NodeConfig{FaucetEnabled: faucetEnabled, FaucetAmount: 100, FaucetCooldownSec: 3600, FaucetDailyCap: 10000}
	return &TruthChainNode{blockchain: bc, storage: storage, wallet: w, config: cfg, faucet: newFaucetState()}
}

func TestFaucetDisabledReturns403(t *testing.T) {
	node := newGatewayNode(t, false)
	recip, _ := wallet.NewWallet()
	rr := httptest.NewRecorder()
	node.handleFaucet(rr, httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(`{"address":"`+recip.GetAddress()+`"}`)))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when faucet disabled, got %d", rr.Code)
	}
}

func TestFaucetDispensesThenCooldown(t *testing.T) {
	node := newGatewayNode(t, true)
	recip, _ := wallet.NewWallet()
	body := `{"address":"` + recip.GetAddress() + `"}`

	// First claim succeeds.
	rr := httptest.NewRecorder()
	node.handleFaucet(rr, httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 on first claim, got %d: %s", rr.Code, rr.Body.String())
	}

	// Second claim for the same address is blocked by the cooldown.
	rr2 := httptest.NewRecorder()
	node.handleFaucet(rr2, httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(body)))
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on cooldown, got %d", rr2.Code)
	}
}

func TestFaucetRejectsInvalidAddress(t *testing.T) {
	node := newGatewayNode(t, true)
	rr := httptest.NewRecorder()
	node.handleFaucet(rr, httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(`{"address":"not-a-real-address"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid address, got %d", rr.Code)
	}
}

func TestFaucetDailyCap(t *testing.T) {
	node := newGatewayNode(t, true)
	// Cap is 10000, amount 100 => 100 claims allowed; the 101st (distinct
	// address, so no cooldown) must be capped.
	for i := 0; i < 100; i++ {
		w, _ := wallet.NewWallet()
		rr := httptest.NewRecorder()
		node.handleFaucet(rr, httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(`{"address":"`+w.GetAddress()+`"}`)))
		if rr.Code != http.StatusCreated {
			t.Fatalf("claim %d expected 201, got %d: %s", i, rr.Code, rr.Body.String())
		}
	}
	w, _ := wallet.NewWallet()
	rr := httptest.NewRecorder()
	node.handleFaucet(rr, httptest.NewRequest(http.MethodPost, "/faucet", strings.NewReader(`{"address":"`+w.GetAddress()+`"}`)))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once daily cap reached, got %d", rr.Code)
	}
}

func TestIPRateLimiter(t *testing.T) {
	l := newIPRateLimiter(3)
	ip := "203.0.113.9"
	for i := 0; i < 3; i++ {
		if !l.allow(ip) {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	if l.allow(ip) {
		t.Fatal("4th request should be rate limited")
	}
	// A different IP is unaffected.
	if !l.allow("198.51.100.1") {
		t.Fatal("different IP should be allowed")
	}
}
