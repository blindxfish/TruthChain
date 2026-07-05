package chain

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/blindxfish/truthchain/wallet"
)

// signedPost builds a Post correctly signed by w, matching the production
// signing format (sha256 of Author+Content+Timestamp, compact secp256k1 sig).
func signedPost(t *testing.T, w *wallet.Wallet, content string, ts int64) Post {
	t.Helper()
	data := fmt.Sprintf("%s%s%d", w.GetAddress(), content, ts)
	sig, err := w.Sign([]byte(data))
	if err != nil {
		t.Fatalf("sign post: %v", err)
	}
	p := Post{
		Author:    w.GetAddress(),
		Signature: hex.EncodeToString(sig),
		Content:   content,
		Timestamp: ts,
	}
	p.SetHash()
	return p
}

func signedTransfer(t *testing.T, w *wallet.Wallet, to string, amount int, nonce int64) Transfer {
	t.Helper()
	tr := Transfer{
		From:      w.GetAddress(),
		To:        to,
		Amount:    amount,
		GasFee:    1,
		Timestamp: time.Now().Unix(),
		Nonce:     nonce,
	}
	h, err := tr.CalculateHash()
	if err != nil {
		t.Fatalf("hash transfer: %v", err)
	}
	tr.Hash = h
	data := fmt.Sprintf("%s:%s:%d:%d:%d:%d", tr.From, tr.To, tr.Amount, tr.GasFee, tr.Timestamp, tr.Nonce)
	sig, err := w.Sign([]byte(data))
	if err != nil {
		t.Fatalf("sign transfer: %v", err)
	}
	tr.Signature = hex.EncodeToString(sig)
	return tr
}

// Regression for the integer-overflow minting bug: an enormous Amount must be
// rejected by Validate before it can overflow GetTotalCost and invert balance
// checks.
func TestTransferAmountBoundsRejectOverflow(t *testing.T) {
	sender, _ := wallet.NewWallet()
	recip, _ := wallet.NewWallet()

	// MaxInt64 amount: previously passed Validate and overflowed GetTotalCost.
	huge := signedTransfer(t, sender, recip.GetAddress(), int(^uint(0)>>1), 0)
	if err := huge.Validate(); err == nil {
		t.Fatal("expected huge transfer amount to be rejected, but Validate passed")
	}

	// Just over the documented maximum must also be rejected.
	over := signedTransfer(t, sender, recip.GetAddress(), MaxTransferAmount+1, 0)
	if err := over.Validate(); err == nil {
		t.Fatal("expected amount above MaxTransferAmount to be rejected")
	}

	// A normal in-bounds transfer must still validate.
	ok := signedTransfer(t, sender, recip.GetAddress(), 100, 0)
	if err := ok.Validate(); err != nil {
		t.Fatalf("expected valid transfer to pass, got: %v", err)
	}
}

func TestPostVerifySignature(t *testing.T) {
	author, _ := wallet.NewWallet()
	ts := time.Now().Unix()

	// Correctly signed post verifies.
	p := signedPost(t, author, "hello truthchain", ts)
	if err := p.VerifySignature(); err != nil {
		t.Fatalf("valid post should verify: %v", err)
	}

	// Tampering the content after signing must be detected (hash no longer
	// matches, and the signature no longer recovers the author).
	tampered := p
	tampered.Content = "hello evil"
	if err := tampered.VerifySignature(); err == nil {
		t.Fatal("tampered post content should fail verification")
	}

	// A forged post claiming someone else's address with a junk signature must
	// be rejected — this is the authorship-forgery guard.
	victim, _ := wallet.NewWallet()
	forged := Post{
		Author:    victim.GetAddress(),
		Content:   "I never said this",
		Timestamp: ts,
		Signature: "00",
	}
	forged.SetHash()
	if err := forged.VerifySignature(); err == nil {
		t.Fatal("forged post should fail verification")
	}
}

func TestValidateBlockDetectsHashTampering(t *testing.T) {
	author, _ := wallet.NewWallet()
	post := signedPost(t, author, "on-chain forever", time.Now().Unix())

	block := &Block{
		Index:     1,
		Timestamp: time.Now().Unix(),
		PrevHash:  MainnetGenesisHash,
		Posts:     []Post{post},
		CharCount: post.GetCharacterCount(),
	}
	block.SetHash()

	if err := block.ValidateBlock(); err != nil {
		t.Fatalf("well-formed block should validate: %v", err)
	}
	if err := block.VerifySignatures(); err != nil {
		t.Fatalf("block with a validly signed post should pass VerifySignatures: %v", err)
	}

	// Tamper the stored hash: ValidateBlock must reject it.
	bad := *block
	bad.Hash = "deadbeef"
	if err := bad.ValidateBlock(); err == nil {
		t.Fatal("block with mismatched hash should be rejected")
	}

	// Tamper a post's content without re-signing: VerifySignatures must reject.
	forgedBlock := &Block{
		Index:     1,
		Timestamp: block.Timestamp,
		PrevHash:  MainnetGenesisHash,
		Posts:     []Post{{Author: author.GetAddress(), Content: "forged", Timestamp: post.Timestamp, Signature: "00"}},
	}
	forgedBlock.Posts[0].SetHash()
	forgedBlock.CharCount = forgedBlock.Posts[0].GetCharacterCount()
	forgedBlock.SetHash()
	if err := forgedBlock.VerifySignatures(); err == nil {
		t.Fatal("block containing a forged post should fail VerifySignatures")
	}
}
