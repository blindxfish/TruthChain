package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/store"
)

// TestSubmitBrowserSignedPostE2E proves the full path: a post signed by the
// browser wallet crypto (the committed interop vector) is accepted by the real
// node HTTP handler. This ties together the JS SDK crypto and the node's
// non-custodial submit endpoint.
func TestSubmitBrowserSignedPostE2E(t *testing.T) {
	// The browser-signed vectors live with the chain package's tests.
	data, err := os.ReadFile(filepath.Join("..", "chain", "testdata", "interop_vectors.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var v struct {
		Account struct {
			Address string `json:"address"`
		} `json:"account"`
		Post json.RawMessage `json:"post"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal vectors: %v", err)
	}

	// Build a node and fund the post author so the balance check passes.
	dbPath := filepath.Join(t.TempDir(), "e2e.db")
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	t.Cleanup(func() { storage.Close() })
	bc, err := blockchain.NewBlockchain(storage, 1000, "truthchain-local")
	if err != nil {
		t.Fatalf("blockchain: %v", err)
	}
	bc.UpdateWalletState(v.Account.Address, 1000, 0)
	node := &TruthChainNode{blockchain: bc, storage: storage}

	// POST the browser-signed post to the real handler.
	rr := httptest.NewRecorder()
	node.handleSubmitPost(rr, httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(v.Post)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("browser-signed post should be accepted (201), got %d: %s", rr.Code, rr.Body.String())
	}
	if len(bc.GetPendingPosts()) != 1 {
		t.Fatalf("browser-signed post was not enqueued")
	}
}
