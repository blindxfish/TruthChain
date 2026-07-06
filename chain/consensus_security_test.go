package chain

import (
	"testing"
	"time"

	"github.com/blindxfish/truthchain/wallet"
)

// TestConsensusMessageSigning verifies that proposals and votes signed by a
// wallet verify against that wallet's address, and that tampering or a wrong
// signer is rejected.
func TestConsensusMessageSigning(t *testing.T) {
	proposer, _ := wallet.NewWallet()
	voter, _ := wallet.NewWallet()

	proposal := &BlockProposal{
		Index:      1,
		ProposerID: proposer.GetAddress(),
		PostHashes: []string{"a", "b"},
		Timestamp:  time.Now().Unix(),
	}
	if err := proposal.Sign(proposer); err != nil {
		t.Fatalf("sign proposal: %v", err)
	}
	if err := proposal.VerifySignature(); err != nil {
		t.Fatalf("valid proposal should verify: %v", err)
	}

	// Tampering the proposal (adding a post) must invalidate the signature.
	tampered := *proposal
	tampered.PostHashes = []string{"a", "b", "c"}
	if err := tampered.VerifySignature(); err == nil {
		t.Fatal("tampered proposal should fail verification")
	}

	// A proposal claiming the proposer's ID but signed by someone else fails.
	forged := &BlockProposal{
		Index:      1,
		ProposerID: proposer.GetAddress(),
		PostHashes: []string{"a", "b"},
		Timestamp:  proposal.Timestamp,
	}
	if err := forged.Sign(voter); err != nil { // signed by the wrong key
		t.Fatalf("sign forged: %v", err)
	}
	if err := forged.VerifySignature(); err == nil {
		t.Fatal("proposal signed by a non-proposer key should fail verification")
	}

	vote := &BlockVote{
		Index:      1,
		ProposerID: proposer.GetAddress(),
		VoterID:    voter.GetAddress(),
		Timestamp:  time.Now().Unix(),
		Approved:   true,
	}
	if err := vote.Sign(voter); err != nil {
		t.Fatalf("sign vote: %v", err)
	}
	if err := vote.VerifySignature(); err != nil {
		t.Fatalf("valid vote should verify: %v", err)
	}

	// A vote forged in another node's name (junk signature) must be rejected.
	forgedVote := &BlockVote{Index: 1, ProposerID: proposer.GetAddress(), VoterID: voter.GetAddress(), Timestamp: vote.Timestamp, Approved: true, Signature: "00"}
	if err := forgedVote.VerifySignature(); err == nil {
		t.Fatal("forged vote should fail verification")
	}
}

// TestQuorumRequiresValidatorMajority proves the core fix: a single approving
// vote no longer finalizes a block when the validator set is larger than the
// proposer + that one voter.
func TestQuorumRequiresValidatorMajority(t *testing.T) {
	config := DefaultConsensusConfig()
	config.PostThreshold = 2
	config.VoteQuorum = 0.75

	bpm := NewBlockProposalManager(config)

	// Simulate a 10-node validator set.
	bpm.SetNetworkSizeProvider(func() int { return 10 })

	proposal := &BlockProposal{
		Index:      1,
		ProposerID: "proposer",
		PostHashes: []string{"a", "b"},
		Timestamp:  time.Now().Unix(),
	}
	if err := bpm.SubmitProposal(proposal); err != nil {
		t.Fatalf("submit proposal: %v", err)
	}

	// One approving vote: proposer(1) + 1 = 2 approvals, but quorum needs
	// ceil(0.75 * 10) = 8. No reservation must be created.
	if err := bpm.SubmitVote(&BlockVote{Index: 1, ProposerID: "proposer", VoterID: "voter-1", Approved: true}); err != nil {
		t.Fatalf("submit vote: %v", err)
	}
	if _, ok := bpm.GetReservation(1); ok {
		t.Fatal("a single vote must NOT reach quorum in a 10-node network")
	}

	// Add votes up to 7 total approving voters: proposer(1) + 7 = 8 >= 8 -> quorum.
	for i := 2; i <= 7; i++ {
		voterID := "voter-" + string(rune('0'+i))
		if err := bpm.SubmitVote(&BlockVote{Index: 1, ProposerID: "proposer", VoterID: voterID, Approved: true}); err != nil {
			t.Fatalf("submit vote %d: %v", i, err)
		}
	}
	if _, ok := bpm.GetReservation(1); !ok {
		t.Fatal("proposer + 7 approving voters (8/10) should reach the 0.75 quorum")
	}
}

// TestProposerCannotPadOwnQuorum verifies a proposer cannot submit a vote on its
// own proposal to inflate the count.
func TestProposerCannotPadOwnQuorum(t *testing.T) {
	config := DefaultConsensusConfig()
	config.PostThreshold = 2
	bpm := NewBlockProposalManager(config)
	bpm.SetNetworkSizeProvider(func() int { return 4 })

	proposal := &BlockProposal{Index: 1, ProposerID: "proposer", PostHashes: []string{"a", "b"}, Timestamp: time.Now().Unix()}
	if err := bpm.SubmitProposal(proposal); err != nil {
		t.Fatalf("submit proposal: %v", err)
	}
	if err := bpm.SubmitVote(&BlockVote{Index: 1, ProposerID: "proposer", VoterID: "proposer", Approved: true}); err == nil {
		t.Fatal("proposer voting on its own proposal must be rejected")
	}
}
