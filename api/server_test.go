package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/miner"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	// Create test wallet
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("Failed to create test wallet: %v", err)
	}

	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test_truthchain_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	// Create test storage
	storage, err := store.NewBoltDBStorage(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create test storage: %v", err)
	}

	// Create test blockchain
	bc, err := blockchain.NewBlockchain(storage, chain.MainnetMinPosts)
	if err != nil {
		storage.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create test blockchain: %v", err)
	}

	// Create test uptime tracker
	uptimeTracker := miner.NewUptimeTracker(w, storage, nil)

	// Create test server
	server := NewServer(bc, uptimeTracker, w, storage, 8080)

	// Cleanup function
	cleanup := func() {
		storage.Close()
		os.Remove(tmpFile.Name())
	}

	return server, cleanup
}

func TestHandleStatus(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleStatus(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"node_info", "blockchain_info", "timestamp"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Response missing required field: %s", field)
		}
	}

	// Check node info
	nodeInfo, ok := response["node_info"].(map[string]interface{})
	if !ok {
		t.Fatal("node_info is not a map")
	}

	requiredNodeFields := []string{"wallet_address", "network", "uptime_24h", "uptime_total", "character_balance"}
	for _, field := range requiredNodeFields {
		if _, exists := nodeInfo[field]; !exists {
			t.Errorf("node_info missing required field: %s", field)
		}
	}
}

func TestHandleWallet(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/wallet", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleWallet(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"address", "network", "character_balance", "public_key"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Response missing required field: %s", field)
		}
	}
}

func TestHandlePost(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Give wallet some characters
	server.storage.UpdateCharacterBalance(server.wallet.GetAddress(), 100)

	// Create test post request
	postData := map[string]string{
		"content": "Hello, TruthChain!",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/post", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call handler
	server.handlePost(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check success
	if success, ok := response["success"].(bool); !ok || !success {
		t.Error("Expected success: true")
	}

	// Check post data
	post, ok := response["post"].(map[string]interface{})
	if !ok {
		t.Fatal("post is not a map")
	}

	requiredPostFields := []string{"hash", "author", "content", "timestamp", "characters"}
	for _, field := range requiredPostFields {
		if _, exists := post[field]; !exists {
			t.Errorf("post missing required field: %s", field)
		}
	}

	// Check new balance
	if newBalance, ok := response["new_balance"].(float64); !ok || newBalance != 82 { // 100 - 17 - 1 gas fee
		t.Errorf("Expected new_balance: 82, got %v", newBalance)
	}
}

func TestHandlePostInsufficientBalance(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test post request with insufficient balance
	postData := map[string]string{
		"content": "This is a very long post that requires more characters than the wallet has",
	}
	jsonData, _ := json.Marshal(postData)

	req := httptest.NewRequest(http.MethodPost, "/post", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call handler
	server.handlePost(w, req)

	// Check response
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleLatestPosts(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/posts/latest", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleLatestPosts(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"latest_block", "pending_posts"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Response missing required field: %s", field)
		}
	}
}

func TestHandleSendCharacters(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Give sender some characters
	server.storage.UpdateCharacterBalance(server.wallet.GetAddress(), 50)

	// Create test transfer request
	transferData := map[string]interface{}{
		"to":     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // Valid test address
		"amount": 10,
	}
	jsonData, _ := json.Marshal(transferData)

	req := httptest.NewRequest(http.MethodPost, "/characters/send", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call handler
	server.handleSendCharacters(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check success
	if success, ok := response["success"].(bool); !ok || !success {
		t.Error("Expected success: true")
	}

	// Check transfer data
	transfer, ok := response["transfer"].(map[string]interface{})
	if !ok {
		t.Fatal("transfer is not a map")
	}

	requiredTransferFields := []string{"from", "to", "amount", "gas_fee", "total_cost"}
	for _, field := range requiredTransferFields {
		if _, exists := transfer[field]; !exists {
			t.Errorf("transfer missing required field: %s", field)
		}
	}

	// Check new balance (50 - 10 - 1 gas fee = 39)
	if newBalance, ok := response["new_balance"].(float64); !ok || newBalance != 39 {
		t.Errorf("Expected new_balance: 39, got %v", newBalance)
	}
}

func TestHandleSendCharactersInsufficientBalance(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test transfer request with insufficient balance
	transferData := map[string]interface{}{
		"to":     "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		"amount": 100,
	}
	jsonData, _ := json.Marshal(transferData)

	req := httptest.NewRequest(http.MethodPost, "/characters/send", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call handler
	server.handleSendCharacters(w, req)

	// Check response
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleUptime(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/uptime", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleUptime(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"uptime_24h_percent", "uptime_total_percent", "character_balance", "heartbeat_count", "last_reward"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Response missing required field: %s", field)
		}
	}
}

func TestHandleBalance(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/balance", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.handleBalance(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"address", "balance"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Response missing required field: %s", field)
		}
	}
}
