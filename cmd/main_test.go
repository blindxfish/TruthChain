package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blindxfish/truthchain/blockchain"
	"github.com/blindxfish/truthchain/chain"
	"github.com/blindxfish/truthchain/store"
	"github.com/blindxfish/truthchain/wallet"
)

// newTestNode builds a TruthChainNode backed by a temporary local blockchain
// with no mesh network (trustNetwork nil, so broadcast is skipped).
func newTestNode(t *testing.T) *TruthChainNode {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	storage, err := store.NewBoltDBStorage(dbPath)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { storage.Close(); os.Remove(dbPath) })

	// High post threshold so a single submitted post does not trigger block
	// creation during the test.
	bc, err := blockchain.NewBlockchain(storage, 1000, "truthchain-local")
	if err != nil {
		t.Fatalf("new blockchain: %v", err)
	}
	return &TruthChainNode{blockchain: bc, storage: storage}
}

// signPost builds a post signed exactly like a client wallet would.
func signPost(t *testing.T, w *wallet.Wallet, content string, ts int64) chain.Post {
	t.Helper()
	data := fmt.Sprintf("%s%s%d", w.GetAddress(), content, ts)
	sig, err := w.Sign([]byte(data))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	p := chain.Post{Author: w.GetAddress(), Content: content, Timestamp: ts, Signature: hex.EncodeToString(sig)}
	p.SetHash()
	return p
}

func TestSubmitPostAcceptsValidClientSignedPost(t *testing.T) {
	node := newTestNode(t)
	w, _ := wallet.NewWallet()
	node.blockchain.UpdateWalletState(w.GetAddress(), 1000, 0) // fund the author

	post := signPost(t, w, "hello, permanent world", time.Now().Unix())
	body, _ := json.Marshal(post)

	rr := httptest.NewRecorder()
	node.handleSubmitPost(rr, httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	// The post must have actually been enqueued (fixes the earlier no-persist bug).
	if got := node.blockchain.GetPendingPosts(); len(got) != 1 || got[0].Hash != post.Hash {
		t.Fatalf("post was not enqueued into pending posts: %+v", got)
	}
}

func TestSubmitPostRejectsForgedSignature(t *testing.T) {
	node := newTestNode(t)
	victim, _ := wallet.NewWallet()
	node.blockchain.UpdateWalletState(victim.GetAddress(), 1000, 0)

	// A post claiming the victim's address but with a junk signature.
	forged := chain.Post{Author: victim.GetAddress(), Content: "I never said this", Timestamp: time.Now().Unix(), Signature: "00"}
	forged.SetHash()
	body, _ := json.Marshal(forged)

	rr := httptest.NewRecorder()
	node.handleSubmitPost(rr, httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(body)))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for forged post, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(node.blockchain.GetPendingPosts()) != 0 {
		t.Fatal("forged post must not be enqueued")
	}
}

func TestSubmitPostRejectsTamperedContent(t *testing.T) {
	node := newTestNode(t)
	w, _ := wallet.NewWallet()
	node.blockchain.UpdateWalletState(w.GetAddress(), 1000, 0)

	post := signPost(t, w, "original", time.Now().Unix())
	post.Content = "tampered" // change content after signing
	body, _ := json.Marshal(post)

	rr := httptest.NewRecorder()
	node.handleSubmitPost(rr, httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(body)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for tampered post, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSubmitPostRejectsMalformedBody(t *testing.T) {
	node := newTestNode(t)
	rr := httptest.NewRecorder()
	node.handleSubmitPost(rr, httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader([]byte("not json"))))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d", rr.Code)
	}
}

func TestLocalCreatePostRejectsNonLoopback(t *testing.T) {
	node := newTestNode(t)
	req := httptest.NewRequest(http.MethodPost, "/local/posts", bytes.NewReader([]byte(`{"content":"hi"}`)))
	req.RemoteAddr = "203.0.113.7:44444" // public, non-loopback
	rr := httptest.NewRecorder()
	node.handleLocalCreatePost(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-loopback local signing, got %d", rr.Code)
	}
}
