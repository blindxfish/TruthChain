package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/blindxfish/truthchain/wallet"
)

// Signer is the minimal signing capability the consensus layer needs from a
// node's wallet. *wallet.Wallet satisfies it. Consensus messages (block
// proposals and votes) are authenticated with the node's key so that a
// ProposerID / VoterID cannot be forged and votes cannot be attributed to
// nodes that did not cast them.
type Signer interface {
	Sign(data []byte) ([]byte, error)
	GetAddress() string
}

// signCanonical signs a canonical-hash string with the signer and returns a
// hex-encoded compact signature. wallet.Sign hashes the input internally, so
// verification must recover over sha256(canonicalHash).
func signCanonical(signer Signer, canonicalHash string) (string, error) {
	if signer == nil {
		return "", fmt.Errorf("no signer configured")
	}
	sig, err := signer.Sign([]byte(canonicalHash))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sig), nil
}

// verifyCanonical recovers the signing address from a hex signature over the
// canonical-hash string and verifies it equals expectedAddress.
func verifyCanonical(canonicalHash, signatureHex, expectedAddress string) error {
	if signatureHex == "" {
		return fmt.Errorf("missing signature")
	}
	if expectedAddress == "" {
		return fmt.Errorf("missing signer address")
	}
	h := sha256.Sum256([]byte(canonicalHash))
	pub, err := wallet.RecoverPublicKeyFromSignature(hex.EncodeToString(h[:]), signatureHex)
	if err != nil {
		return fmt.Errorf("signature recovery failed: %w", err)
	}
	if got := wallet.DeriveAddress(pub); got != expectedAddress {
		return fmt.Errorf("signature address mismatch: signed by %s, expected %s", got, expectedAddress)
	}
	return nil
}

// Sign signs the proposal with the proposer's key.
func (bp *BlockProposal) Sign(signer Signer) error {
	sig, err := signCanonical(signer, bp.CalculateHash())
	if err != nil {
		return err
	}
	bp.Signature = sig
	return nil
}

// VerifySignature verifies the proposal was signed by its ProposerID.
func (bp *BlockProposal) VerifySignature() error {
	return verifyCanonical(bp.CalculateHash(), bp.Signature, bp.ProposerID)
}

// Sign signs the vote with the voter's key.
func (bv *BlockVote) Sign(signer Signer) error {
	sig, err := signCanonical(signer, bv.CalculateHash())
	if err != nil {
		return err
	}
	bv.Signature = sig
	return nil
}

// VerifySignature verifies the vote was signed by its VoterID.
func (bv *BlockVote) VerifySignature() error {
	return verifyCanonical(bv.CalculateHash(), bv.Signature, bv.VoterID)
}

// CalculateHash returns the canonical hash of a time-based block request.
func (r *TimeBasedBlockRequest) CalculateHash() string {
	data := fmt.Sprintf("%s%d%d", r.ProposerID, r.BlockIndex, r.Timestamp)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// Sign signs the time-based block request with the proposer's key.
func (r *TimeBasedBlockRequest) Sign(signer Signer) error {
	sig, err := signCanonical(signer, r.CalculateHash())
	if err != nil {
		return err
	}
	r.Signature = sig
	return nil
}

// VerifySignature verifies the request was signed by its ProposerID.
func (r *TimeBasedBlockRequest) VerifySignature() error {
	return verifyCanonical(r.CalculateHash(), r.Signature, r.ProposerID)
}

// CalculateHash returns the canonical hash of a time-based block vote.
func (v *TimeBasedBlockVote) CalculateHash() string {
	data := fmt.Sprintf("%s%s%d%t%d", v.ProposerID, v.VoterID, v.BlockIndex, v.Approved, v.Timestamp)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// Sign signs the time-based block vote with the voter's key.
func (v *TimeBasedBlockVote) Sign(signer Signer) error {
	sig, err := signCanonical(signer, v.CalculateHash())
	if err != nil {
		return err
	}
	v.Signature = sig
	return nil
}

// VerifySignature verifies the vote was signed by its VoterID.
func (v *TimeBasedBlockVote) VerifySignature() error {
	return verifyCanonical(v.CalculateHash(), v.Signature, v.VoterID)
}
