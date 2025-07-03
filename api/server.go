package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/miner"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

// Server represents the TruthChain HTTP API server
type Server struct {
	blockchain    *blockchain.Blockchain
	uptimeTracker *miner.UptimeTracker
	wallet        *wallet.Wallet
	storage       store.Storage
	port          int
}

// NewServer creates a new HTTP API server
func NewServer(bc *blockchain.Blockchain, ut *miner.UptimeTracker, w *wallet.Wallet, s store.Storage, port int) *Server {
	return &Server{
		blockchain:    bc,
		uptimeTracker: ut,
		wallet:        w,
		storage:       s,
		port:          port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Set up routes
	http.HandleFunc("/status", s.handleStatus)
	http.HandleFunc("/wallet", s.handleWallet)
	http.HandleFunc("/post", s.handlePost)
	http.HandleFunc("/posts/latest", s.handleLatestPosts)
	http.HandleFunc("/characters/send", s.handleSendCharacters)
	http.HandleFunc("/uptime", s.handleUptime)
	http.HandleFunc("/balance", s.handleBalance)

	// Start server
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	log.Printf("Starting TruthChain API server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleStatus handles GET /status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get blockchain info
	chainInfo, err := s.blockchain.GetBlockchainInfo()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get blockchain info: %v", err), http.StatusInternalServerError)
		return
	}

	// Get uptime info
	uptimeInfo := s.uptimeTracker.GetUptimeInfo()

	// Combine info
	status := map[string]interface{}{
		"node_info": map[string]interface{}{
			"wallet_address":    s.wallet.GetAddress(),
			"network":           s.wallet.GetNetwork(),
			"uptime_24h":        uptimeInfo["uptime_24h_percent"],
			"uptime_total":      uptimeInfo["uptime_total_percent"],
			"character_balance": uptimeInfo["character_balance"],
		},
		"blockchain_info": chainInfo,
		"timestamp":       time.Now().Unix(),
	}

	s.writeJSON(w, status)
}

// handleWallet handles GET /wallet
func (s *Server) handleWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get character balance
	balance, err := s.storage.GetCharacterBalance(s.wallet.GetAddress())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get balance: %v", err), http.StatusInternalServerError)
		return
	}

	walletInfo := map[string]interface{}{
		"address":           s.wallet.GetAddress(),
		"network":           s.wallet.GetNetwork(),
		"character_balance": balance,
		"public_key":        s.wallet.ExportPublicKeyHex(),
	}

	if s.wallet.Metadata != nil {
		walletInfo["name"] = s.wallet.Metadata.Name
		walletInfo["created"] = s.wallet.Metadata.Created.Format(time.RFC3339)
		walletInfo["last_used"] = s.wallet.Metadata.LastUsed.Format(time.RFC3339)
	}

	s.writeJSON(w, walletInfo)
}

// handlePost handles POST /post
func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "Content cannot be empty", http.StatusBadRequest)
		return
	}

	// Check character balance
	balance, err := s.storage.GetCharacterBalance(s.wallet.GetAddress())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get balance: %v", err), http.StatusInternalServerError)
		return
	}

	if balance < len(req.Content) {
		http.Error(w, fmt.Sprintf("Insufficient character balance: %d, need %d", balance, len(req.Content)), http.StatusBadRequest)
		return
	}

	// Create and add post
	post, err := s.blockchain.CreatePost(req.Content, s.wallet)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create post: %v", err), http.StatusInternalServerError)
		return
	}

	if err := s.blockchain.AddPost(*post); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add post: %v", err), http.StatusInternalServerError)
		return
	}

	// Deduct characters from balance
	if err := s.storage.UpdateCharacterBalance(s.wallet.GetAddress(), -len(req.Content)); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update balance: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"post": map[string]interface{}{
			"hash":       post.Hash,
			"author":     post.Author,
			"content":    post.Content,
			"timestamp":  post.Timestamp,
			"characters": post.GetCharacterCount(),
		},
		"new_balance": balance - len(req.Content),
	}

	s.writeJSON(w, response)
}

// handleLatestPosts handles GET /posts/latest
func (s *Server) handleLatestPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get latest block
	latestBlock, err := s.blockchain.GetLatestBlock()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get latest block: %v", err), http.StatusInternalServerError)
		return
	}

	// Get recent posts from mempool
	mempoolInfo := s.blockchain.GetMempoolInfo()
	pendingPosts := mempoolInfo["posts"].([]map[string]interface{})

	response := map[string]interface{}{
		"latest_block": map[string]interface{}{
			"index":     latestBlock.Index,
			"hash":      latestBlock.Hash,
			"timestamp": latestBlock.Timestamp,
			"posts":     latestBlock.Posts,
		},
		"pending_posts": pendingPosts,
	}

	s.writeJSON(w, response)
}

// handleSendCharacters handles POST /characters/send
func (s *Server) handleSendCharacters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		To     string `json:"to"`
		Amount int    `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.To == "" {
		http.Error(w, "Recipient address cannot be empty", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	// Validate recipient address
	if !wallet.ValidateAddress(req.To) {
		http.Error(w, "Invalid recipient address", http.StatusBadRequest)
		return
	}

	// Check sender balance (including gas fee)
	totalCost := req.Amount + 1 // 1 character gas fee
	balance, err := s.storage.GetCharacterBalance(s.wallet.GetAddress())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get balance: %v", err), http.StatusInternalServerError)
		return
	}

	if balance < totalCost {
		http.Error(w, fmt.Sprintf("Insufficient balance: %d, need %d (including 1 char gas fee)", balance, totalCost), http.StatusBadRequest)
		return
	}

	// Transfer characters
	if err := s.storage.UpdateCharacterBalance(s.wallet.GetAddress(), -totalCost); err != nil {
		http.Error(w, fmt.Sprintf("Failed to deduct from sender: %v", err), http.StatusInternalServerError)
		return
	}

	if err := s.storage.UpdateCharacterBalance(req.To, req.Amount); err != nil {
		// Rollback sender deduction
		s.storage.UpdateCharacterBalance(s.wallet.GetAddress(), totalCost)
		http.Error(w, fmt.Sprintf("Failed to add to recipient: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"transfer": map[string]interface{}{
			"from":       s.wallet.GetAddress(),
			"to":         req.To,
			"amount":     req.Amount,
			"gas_fee":    1,
			"total_cost": totalCost,
		},
		"new_balance": balance - totalCost,
	}

	s.writeJSON(w, response)
}

// handleUptime handles GET /uptime
func (s *Server) handleUptime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptimeInfo := s.uptimeTracker.GetUptimeInfo()
	s.writeJSON(w, uptimeInfo)
}

// handleBalance handles GET /balance
func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	balance, err := s.storage.GetCharacterBalance(s.wallet.GetAddress())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get balance: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"address": s.wallet.GetAddress(),
		"balance": balance,
	}

	s.writeJSON(w, response)
}

// writeJSON writes a JSON response
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
	}
}
