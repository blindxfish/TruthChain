package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/wallet"
)

// ---- config defaults -------------------------------------------------------

func (n *TruthChainNode) apiRateLimit() int {
	if n.config != nil && n.config.APIRateLimit > 0 {
		return n.config.APIRateLimit
	}
	return 120 // requests/min per IP
}

func (n *TruthChainNode) faucetAmount() int {
	if n.config != nil && n.config.FaucetAmount > 0 {
		return n.config.FaucetAmount
	}
	return 100
}

func (n *TruthChainNode) faucetCooldown() time.Duration {
	if n.config != nil && n.config.FaucetCooldownSec > 0 {
		return time.Duration(n.config.FaucetCooldownSec) * time.Second
	}
	return time.Hour
}

func (n *TruthChainNode) faucetDailyCap() int {
	if n.config != nil && n.config.FaucetDailyCap > 0 {
		return n.config.FaucetDailyCap
	}
	return 10000
}

// clientIP extracts the caller IP for per-IP limiting.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---- per-IP rate limiter (fixed 1-minute window) ---------------------------

type ipRateLimiter struct {
	mu       sync.Mutex
	counts   map[string]int
	window   time.Time
	limit    int
	duration time.Duration
}

func newIPRateLimiter(limit int) *ipRateLimiter {
	return &ipRateLimiter{
		counts:   make(map[string]int),
		window:   time.Now(),
		limit:    limit,
		duration: time.Minute,
	}
}

// allow reports whether ip may make another request in the current window.
func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if time.Since(l.window) >= l.duration {
		l.counts = make(map[string]int)
		l.window = time.Now()
	}
	l.counts[ip]++
	return l.counts[ip] <= l.limit
}

// rateLimitMiddleware caps requests per IP. It is only meaningful for a public
// node; on a loopback-only node every caller is 127.0.0.1 but the limit is high
// enough not to interfere with local use.
func (n *TruthChainNode) rateLimitMiddleware(next http.Handler) http.Handler {
	limiter := newIPRateLimiter(n.apiRateLimit())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- faucet ----------------------------------------------------------------

// faucetState tracks anti-abuse counters for character distribution.
type faucetState struct {
	mu          sync.Mutex
	lastClaim   map[string]time.Time // address -> last claim
	dayStart    time.Time
	dispensedTo int // characters dispensed in the current day
}

func newFaucetState() *faucetState {
	return &faucetState{lastClaim: make(map[string]time.Time), dayStart: time.Now()}
}

// canDispense reports whether amount may be sent to address now, updating
// counters if so. Returns a reason when denied.
func (f *faucetState) canDispense(address string, amount, dailyCap int, cooldown time.Duration) (bool, string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Reset the daily counter on a new day.
	if time.Since(f.dayStart) >= 24*time.Hour {
		f.dayStart = time.Now()
		f.dispensedTo = 0
	}

	if last, ok := f.lastClaim[address]; ok && time.Since(last) < cooldown {
		return false, fmt.Sprintf("address on cooldown, try again in %s", (cooldown - time.Since(last)).Round(time.Second))
	}
	if f.dispensedTo+amount > dailyCap {
		return false, "faucet daily cap reached, try again later"
	}

	f.lastClaim[address] = time.Now()
	f.dispensedTo += amount
	return true, ""
}

// handleFaucet dispenses characters from the node operator's wallet to a
// caller-supplied address. This is how a website funds its users so they can
// post. It is node-signed (spends the operator's balance), disabled by default,
// and rate-limited per address + per day.
func (n *TruthChainNode) handleFaucet(w http.ResponseWriter, r *http.Request) {
	if n.config == nil || !n.config.FaucetEnabled {
		http.Error(w, "faucet is disabled on this node", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	var req struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: expected {\"address\":\"...\"}", http.StatusBadRequest)
		return
	}
	if !wallet.ValidateAddress(req.Address) {
		http.Error(w, "invalid recipient address", http.StatusBadRequest)
		return
	}

	amount := n.faucetAmount()
	if ok, reason := n.faucet.canDispense(req.Address, amount, n.faucetDailyCap(), n.faucetCooldown()); !ok {
		http.Error(w, reason, http.StatusTooManyRequests)
		return
	}

	// Node-signed transfer from the operator's wallet to the user.
	transfer, err := n.blockchain.CreateTransfer(req.Address, amount, n.wallet)
	if err != nil {
		http.Error(w, fmt.Sprintf("faucet transfer failed: %v", err), http.StatusInternalServerError)
		return
	}
	if err := n.blockchain.AddTransfer(*transfer); err != nil {
		http.Error(w, fmt.Sprintf("faucet transfer rejected: %v", err), http.StatusBadRequest)
		return
	}
	if n.trustNetwork != nil {
		_ = n.trustNetwork.BroadcastTransfer(transfer)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "sent",
		"to":      req.Address,
		"amount":  amount,
		"tx_hash": transfer.Hash,
	})
}
