package chain

import (
	"math"
	"testing"
	"time"
)

func TestConsensusSystem(t *testing.T) {
	// Create consensus configuration
	config := DefaultConsensusConfig()
	config.PostThreshold = 3 // Use 3 posts for testing
	config.ProposalTimeout = 1 * time.Minute
	config.MinTrustScore = 0.5

	// Create consensus engine
	nodeID := "test-node-1"
	engine := NewConsensusEngine(nodeID, config)

	// Start the engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start consensus engine: %v", err)
	}
	defer engine.Stop()

	// Create test posts
	posts := []Post{
		{
			Author:    "user1",
			Content:   "First test post",
			Timestamp: time.Now().Unix(),
			Signature: "sig1",
		},
		{
			Author:    "user2",
			Content:   "Second test post",
			Timestamp: time.Now().Unix(),
			Signature: "sig2",
		},
		{
			Author:    "user3",
			Content:   "Third test post",
			Timestamp: time.Now().Unix(),
			Signature: "sig3",
		},
	}

	// Set hashes for posts
	for i := range posts {
		posts[i].SetHash()
	}

	// Add posts to consensus engine
	for _, post := range posts {
		if err := engine.AddPost(post); err != nil {
			t.Fatalf("Failed to add post: %v", err)
		}
	}

	// Check that mempool has the posts
	if count := engine.mempool.GetPostCount(); count != 3 {
		t.Errorf("Expected 3 posts in mempool, got %d", count)
	}

	// Check that we're ready for block creation
	if !engine.mempool.IsReadyForBlock(config.PostThreshold) {
		t.Error("Expected to be ready for block creation")
	}

	// Check trust score
	trustScore := engine.trustManager.GetTrustScore(nodeID)
	if trustScore < config.MinTrustScore {
		t.Errorf("Expected trust score >= %f, got %f", config.MinTrustScore, trustScore)
	}

	// Check that we can propose
	if !engine.trustManager.CanPropose(nodeID) {
		t.Error("Expected to be able to propose blocks")
	}

	// Get stats
	stats := engine.GetStats()
	if stats["mempool_post_count"] != 3 {
		t.Errorf("Expected 3 posts in stats, got %v", stats["mempool_post_count"])
	}
}

func TestBlockProposalAndVoting(t *testing.T) {
	// Create consensus configuration
	config := DefaultConsensusConfig()
	config.PostThreshold = 2 // Use 2 posts for testing

	// Create consensus engine
	nodeID := "test-node-1"
	engine := NewConsensusEngine(nodeID, config)

	// Start the engine
	if err := engine.Start(); err != nil {
		t.Fatalf("Failed to start consensus engine: %v", err)
	}
	defer engine.Stop()

	// Create test posts
	posts := []Post{
		{
			Author:    "user1",
			Content:   "Test post 1",
			Timestamp: time.Now().Unix(),
			Signature: "sig1",
		},
		{
			Author:    "user2",
			Content:   "Test post 2",
			Timestamp: time.Now().Unix(),
			Signature: "sig2",
		},
	}

	// Set hashes for posts
	for i := range posts {
		posts[i].SetHash()
	}

	// Add posts to consensus engine
	for _, post := range posts {
		if err := engine.AddPost(post); err != nil {
			t.Fatalf("Failed to add post: %v", err)
		}
	}

	// Create a block proposal
	proposal := &BlockProposal{
		Index:      1,
		ProposerID: "test-node-2",
		PostHashes: []string{posts[0].Hash, posts[1].Hash},
		Timestamp:  time.Now().Unix(),
		TrustScore: 0.8,
		Signature:  "proposal_sig",
	}

	// Submit proposal
	if err := engine.proposalManager.SubmitProposal(proposal); err != nil {
		t.Fatalf("Failed to submit proposal: %v", err)
	}

	// Create a vote
	vote := &BlockVote{
		Index:      1,
		ProposerID: "test-node-2",
		VoterID:    nodeID,
		Timestamp:  time.Now().Unix(),
		Approved:   true,
		Signature:  "vote_sig",
	}

	// Submit vote
	if err := engine.proposalManager.SubmitVote(vote); err != nil {
		t.Fatalf("Failed to submit vote: %v", err)
	}

	// Check that we have active proposals
	proposals := engine.proposalManager.GetActiveProposals()
	if len(proposals) != 1 {
		t.Errorf("Expected 1 active proposal, got %d", len(proposals))
	}

	// Check that we have a reservation (since we voted)
	reservations := engine.proposalManager.GetActiveReservations()
	if len(reservations) != 1 {
		t.Errorf("Expected 1 active reservation, got %d", len(reservations))
	}

	// Check reservation details
	reservation := reservations[0]
	if reservation.Index != 1 {
		t.Errorf("Expected reservation for block 1, got %d", reservation.Index)
	}
	if reservation.Proposer != "test-node-2" {
		t.Errorf("Expected proposer test-node-2, got %s", reservation.Proposer)
	}
	if len(reservation.ApprovedBy) != 1 {
		t.Errorf("Expected 1 approval, got %d", len(reservation.ApprovedBy))
	}
}

func TestPostMempool(t *testing.T) {
	// Create mempool
	mempool := NewPostMempool()

	// Create test posts
	posts := []Post{
		{
			Author:    "user1",
			Content:   "Post 1",
			Timestamp: time.Now().Unix(),
			Signature: "sig1",
		},
		{
			Author:    "user2",
			Content:   "Post 2",
			Timestamp: time.Now().Unix() + 1,
			Signature: "sig2",
		},
		{
			Author:    "user3",
			Content:   "Post 3",
			Timestamp: time.Now().Unix() + 2,
			Signature: "sig3",
		},
	}

	// Set hashes
	for i := range posts {
		posts[i].SetHash()
	}

	// Add posts
	for _, post := range posts {
		if err := mempool.AddPost(post); err != nil {
			t.Fatalf("Failed to add post: %v", err)
		}
	}

	// Check post count
	if count := mempool.GetPostCount(); count != 3 {
		t.Errorf("Expected 3 posts, got %d", count)
	}

	// Check ready for block
	if !mempool.IsReadyForBlock(3) {
		t.Error("Expected to be ready for block with 3 posts")
	}

	if mempool.IsReadyForBlock(4) {
		t.Error("Expected not to be ready for block with 4 posts")
	}

	// Select posts for block
	selectedPosts := mempool.SelectPostsForBlock(2)
	if len(selectedPosts) != 2 {
		t.Errorf("Expected 2 selected posts, got %d", len(selectedPosts))
	}

	// Check ordering (should be by timestamp)
	if selectedPosts[0].Timestamp > selectedPosts[1].Timestamp {
		t.Error("Posts should be ordered by timestamp")
	}

	// Remove posts
	postHashes := []string{posts[0].Hash, posts[1].Hash}
	mempool.RemovePosts(postHashes)

	// Check remaining posts
	if count := mempool.GetPostCount(); count != 1 {
		t.Errorf("Expected 1 remaining post, got %d", count)
	}

	// Check that removed posts are gone
	if _, exists := mempool.GetPost(posts[0].Hash); exists {
		t.Error("Post 0 should be removed")
	}

	if _, exists := mempool.GetPost(posts[1].Hash); exists {
		t.Error("Post 1 should be removed")
	}

	if _, exists := mempool.GetPost(posts[2].Hash); !exists {
		t.Error("Post 2 should still exist")
	}
}

func TestTrustManager(t *testing.T) {
	// Create configuration
	config := DefaultConsensusConfig()

	// Create trust manager
	trustManager := NewTrustManager(config)

	// Test default trust score
	nodeID := "test-node"
	trustScore := trustManager.GetTrustScore(nodeID)
	if trustScore != config.MinTrustScore {
		t.Errorf("Expected default trust score %f, got %f", config.MinTrustScore, trustScore)
	}

	// Test trust score operations
	trustManager.SetTrustScore(nodeID, 0.8)
	if score := trustManager.GetTrustScore(nodeID); score != 0.8 {
		t.Errorf("Expected trust score 0.8, got %f", score)
	}

	// Test increase
	trustManager.IncreaseTrust(nodeID, 0.1)
	if score := trustManager.GetTrustScore(nodeID); score != 0.9 {
		t.Errorf("Expected trust score 0.9, got %f", score)
	}

	// Test decrease
	trustManager.DecreaseTrust(nodeID, 0.2)
	if score := trustManager.GetTrustScore(nodeID); score != 0.7 {
		t.Errorf("Expected trust score 0.7, got %f", score)
	}

	// Test clamping
	trustManager.SetTrustScore(nodeID, 1.5)
	if score := trustManager.GetTrustScore(nodeID); score != 1.0 {
		t.Errorf("Expected trust score 1.0, got %f", score)
	}

	trustManager.SetTrustScore(nodeID, -0.5)
	if score := trustManager.GetTrustScore(nodeID); score != 0.0 {
		t.Errorf("Expected trust score 0.0, got %f", score)
	}

	// Test proposal permissions
	trustManager.SetTrustScore(nodeID, 0.6)
	if !trustManager.CanPropose(nodeID) {
		t.Error("Node should be able to propose with trust score 0.6")
	}

	trustManager.SetTrustScore(nodeID, 0.4)
	if trustManager.CanPropose(nodeID) {
		t.Error("Node should not be able to propose with trust score 0.4")
	}

	// Test success/failure operations
	trustManager.SetTrustScore(nodeID, 0.5)
	trustManager.OnProposalSuccess(nodeID)
	if score := trustManager.GetTrustScore(nodeID); math.Abs(score-0.51) > 0.001 {
		t.Errorf("Expected trust score 0.51 after success, got %f", score)
	}

	trustManager.OnProposalFailure(nodeID)
	if score := trustManager.GetTrustScore(nodeID); math.Abs(score-0.41) > 0.001 {
		t.Errorf("Expected trust score 0.41 after failure, got %f", score)
	}
}
