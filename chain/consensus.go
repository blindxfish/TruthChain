package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// PostGossip represents a post being gossiped through the network
type PostGossip struct {
	Post      Post   `json:"post"`
	NodeID    string `json:"node_id"`   // Node that created the post
	Timestamp int64  `json:"timestamp"` // When the gossip was sent
	Signature string `json:"signature"` // Signature of the post by the creating node
}

// BlockProposal represents a proposal to create a block
type BlockProposal struct {
	Index      int      `json:"index"`       // Block index
	ProposerID string   `json:"proposer_id"` // Node proposing the block
	PostHashes []string `json:"post_hashes"` // Hashes of posts to include
	Timestamp  int64    `json:"timestamp"`   // When proposal was created
	Signature  string   `json:"signature"`   // Signature of proposer
	TrustScore float64  `json:"trust_score"` // Proposer's trust score
}

// BlockVote represents a vote on a block proposal
type BlockVote struct {
	Index      int    `json:"index"`       // Block index being voted on
	ProposerID string `json:"proposer_id"` // ID of the proposer
	VoterID    string `json:"voter_id"`    // ID of the voting node
	Timestamp  int64  `json:"timestamp"`   // When vote was cast
	Signature  string `json:"signature"`   // Signature of voter
	Approved   bool   `json:"approved"`    // Whether proposal is approved
}

// BlockReservation represents an approved block proposal
type BlockReservation struct {
	Index      int       `json:"index"`       // Block index
	Proposer   string    `json:"proposer"`    // Node that proposed
	ApprovedBy []string  `json:"approved_by"` // List of nodes that approved
	TimeoutAt  time.Time `json:"timeout_at"`  // When reservation expires
	PostHashes []string  `json:"post_hashes"` // Posts to be included
	TrustScore float64   `json:"trust_score"` // Proposer's trust score
	CreatedAt  time.Time `json:"created_at"`  // When reservation was created
}

// ProposalExpired represents an expired block proposal
type ProposalExpired struct {
	Index      int    `json:"index"`       // Block index
	ProposerID string `json:"proposer_id"` // ID of the expired proposer
	Timestamp  int64  `json:"timestamp"`   // When expiration was announced
	Signature  string `json:"signature"`   // Signature of announcing node
}

// ConsensusConfig holds consensus configuration
type ConsensusConfig struct {
	PostThreshold    int           `json:"post_threshold"`    // Posts needed for block (default: 5)
	ProposalTimeout  time.Duration `json:"proposal_timeout"`  // Time to create block after approval (default: 5 minutes)
	MinTrustScore    float64       `json:"min_trust_score"`   // Minimum trust to propose (default: 0.5)
	VoteQuorum       float64       `json:"vote_quorum"`       // Required vote percentage (default: 0.75)
	UptimeIncrement  float64       `json:"uptime_increment"`  // Trust increase per hour (default: 0.01)
	SuccessIncrement float64       `json:"success_increment"` // Trust increase on success (default: 0.01)
	FailurePenalty   float64       `json:"failure_penalty"`   // Trust decrease on failure (default: 0.1)
}

// DefaultConsensusConfig returns default consensus configuration
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		PostThreshold:    5,
		ProposalTimeout:  5 * time.Minute,
		MinTrustScore:    0.5,
		VoteQuorum:       0.75,
		UptimeIncrement:  0.01,
		SuccessIncrement: 0.01,
		FailurePenalty:   0.1,
	}
}

// PostMempool manages pending posts for consensus
type PostMempool struct {
	posts map[string]Post // Hash -> Post
	mu    sync.RWMutex
}

// NewPostMempool creates a new post mempool
func NewPostMempool() *PostMempool {
	return &PostMempool{
		posts: make(map[string]Post),
	}
}

// AddPost adds a post to the mempool
func (pm *PostMempool) AddPost(post Post) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if post.Hash == "" {
		post.SetHash()
	}

	// Check for duplicates
	if _, exists := pm.posts[post.Hash]; exists {
		return fmt.Errorf("duplicate post: %s", post.Hash)
	}

	pm.posts[post.Hash] = post
	return nil
}

// GetPost returns a post by hash
func (pm *PostMempool) GetPost(hash string) (Post, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	post, exists := pm.posts[hash]
	return post, exists
}

// GetPosts returns all posts in the mempool
func (pm *PostMempool) GetPosts() []Post {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	posts := make([]Post, 0, len(pm.posts))
	for _, post := range pm.posts {
		posts = append(posts, post)
	}

	// Sort by timestamp for deterministic ordering
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Timestamp < posts[j].Timestamp
	})

	return posts
}

// RemovePosts removes posts by their hashes
func (pm *PostMempool) RemovePosts(hashes []string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, hash := range hashes {
		delete(pm.posts, hash)
	}
}

// GetPostCount returns the number of posts in the mempool
func (pm *PostMempool) GetPostCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.posts)
}

// IsReadyForBlock checks if there are enough posts to create a block
func (pm *PostMempool) IsReadyForBlock(threshold int) bool {
	return pm.GetPostCount() >= threshold
}

// SelectPostsForBlock selects posts for block creation (up to threshold)
func (pm *PostMempool) SelectPostsForBlock(threshold int) []Post {
	posts := pm.GetPosts()
	if len(posts) <= threshold {
		return posts
	}
	return posts[:threshold]
}

// TrustManager manages node trust scores
type TrustManager struct {
	scores map[string]float64 // NodeID -> TrustScore
	mu     sync.RWMutex
	config *ConsensusConfig
}

// NewTrustManager creates a new trust manager
func NewTrustManager(config *ConsensusConfig) *TrustManager {
	return &TrustManager{
		scores: make(map[string]float64),
		config: config,
	}
}

// GetTrustScore returns a node's trust score
func (tm *TrustManager) GetTrustScore(nodeID string) float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	score, exists := tm.scores[nodeID]
	if !exists {
		// New nodes start with minimum trust score
		return tm.config.MinTrustScore
	}
	return score
}

// SetTrustScore sets a node's trust score
func (tm *TrustManager) SetTrustScore(nodeID string, score float64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Clamp score between 0.0 and 1.0
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	tm.scores[nodeID] = score
}

// IncreaseTrust increases a node's trust score
func (tm *TrustManager) IncreaseTrust(nodeID string, amount float64) {
	current := tm.GetTrustScore(nodeID)
	tm.SetTrustScore(nodeID, current+amount)
}

// DecreaseTrust decreases a node's trust score
func (tm *TrustManager) DecreaseTrust(nodeID string, amount float64) {
	current := tm.GetTrustScore(nodeID)
	tm.SetTrustScore(nodeID, current-amount)
}

// CanPropose checks if a node can propose blocks
func (tm *TrustManager) CanPropose(nodeID string) bool {
	return tm.GetTrustScore(nodeID) >= tm.config.MinTrustScore
}

// UpdateUptime updates trust score based on uptime
func (tm *TrustManager) UpdateUptime(nodeID string, uptimeHours float64) {
	increment := uptimeHours * tm.config.UptimeIncrement
	tm.IncreaseTrust(nodeID, increment)
}

// OnProposalSuccess increases trust when proposal succeeds
func (tm *TrustManager) OnProposalSuccess(nodeID string) {
	tm.IncreaseTrust(nodeID, tm.config.SuccessIncrement)
}

// OnProposalFailure decreases trust when proposal fails
func (tm *TrustManager) OnProposalFailure(nodeID string) {
	tm.DecreaseTrust(nodeID, tm.config.FailurePenalty)
}

// GetAllScores returns all trust scores
func (tm *TrustManager) GetAllScores() map[string]float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	scores := make(map[string]float64)
	for nodeID, score := range tm.scores {
		scores[nodeID] = score
	}
	return scores
}

// BlockProposalManager manages block proposals and voting
type BlockProposalManager struct {
	reservations map[int]*BlockReservation // BlockIndex -> Reservation
	proposals    map[int]*BlockProposal    // BlockIndex -> Proposal
	votes        map[int][]*BlockVote      // BlockIndex -> Votes
	mu           sync.RWMutex
	config       *ConsensusConfig
}

// NewBlockProposalManager creates a new proposal manager
func NewBlockProposalManager(config *ConsensusConfig) *BlockProposalManager {
	return &BlockProposalManager{
		reservations: make(map[int]*BlockReservation),
		proposals:    make(map[int]*BlockProposal),
		votes:        make(map[int][]*BlockVote),
		config:       config,
	}
}

// SubmitProposal submits a new block proposal
func (bpm *BlockProposalManager) SubmitProposal(proposal *BlockProposal) error {
	bpm.mu.Lock()
	defer bpm.mu.Unlock()

	// Check if there's already a proposal for this block
	if _, exists := bpm.proposals[proposal.Index]; exists {
		return fmt.Errorf("proposal already exists for block %d", proposal.Index)
	}

	// Check if there's already a reservation for this block
	if _, exists := bpm.reservations[proposal.Index]; exists {
		return fmt.Errorf("reservation already exists for block %d", proposal.Index)
	}

	bpm.proposals[proposal.Index] = proposal
	bpm.votes[proposal.Index] = []*BlockVote{}

	return nil
}

// SubmitVote submits a vote on a proposal
func (bpm *BlockProposalManager) SubmitVote(vote *BlockVote) error {
	bpm.mu.Lock()
	defer bpm.mu.Unlock()

	// Check if proposal exists
	_, exists := bpm.proposals[vote.Index]
	if !exists {
		return fmt.Errorf("no proposal found for block %d", vote.Index)
	}

	// Check if already voted
	for _, existingVote := range bpm.votes[vote.Index] {
		if existingVote.VoterID == vote.VoterID {
			return fmt.Errorf("node %s already voted on block %d", vote.VoterID, vote.Index)
		}
	}

	// Add vote
	bpm.votes[vote.Index] = append(bpm.votes[vote.Index], vote)

	// Check if we have enough votes for approval
	if bpm.checkApproval(vote.Index) {
		bpm.createReservation(vote.Index)
	}

	return nil
}

// checkApproval checks if a proposal has enough votes to be approved
func (bpm *BlockProposalManager) checkApproval(blockIndex int) bool {
	// proposal := bpm.proposals[blockIndex]
	votes := bpm.votes[blockIndex]

	// Count approved votes
	approvedCount := 0
	for _, vote := range votes {
		if vote.Approved {
			approvedCount++
		}
	}

	// Calculate approval percentage (simplified - in real implementation,
	// you'd need to know total online peers)
	totalVotes := len(votes)
	if totalVotes == 0 {
		return false
	}

	approvalPercentage := float64(approvedCount) / float64(totalVotes)
	return approvalPercentage >= bpm.config.VoteQuorum
}

// createReservation creates a reservation for an approved proposal
func (bpm *BlockProposalManager) createReservation(blockIndex int) {
	proposal := bpm.proposals[blockIndex]
	votes := bpm.votes[blockIndex]

	// Collect voter IDs
	voterIDs := make([]string, 0)
	for _, vote := range votes {
		if vote.Approved {
			voterIDs = append(voterIDs, vote.VoterID)
		}
	}

	reservation := &BlockReservation{
		Index:      blockIndex,
		Proposer:   proposal.ProposerID,
		ApprovedBy: voterIDs,
		TimeoutAt:  time.Now().Add(bpm.config.ProposalTimeout),
		PostHashes: proposal.PostHashes,
		TrustScore: proposal.TrustScore,
		CreatedAt:  time.Now(),
	}

	bpm.reservations[blockIndex] = reservation

	// Remove the proposal since it's now reserved
	delete(bpm.proposals, blockIndex)
}

// GetReservation returns a reservation for a block
func (bpm *BlockProposalManager) GetReservation(blockIndex int) (*BlockReservation, bool) {
	bpm.mu.RLock()
	defer bpm.mu.RUnlock()

	reservation, exists := bpm.reservations[blockIndex]
	return reservation, exists
}

// RemoveReservation removes a reservation
func (bpm *BlockProposalManager) RemoveReservation(blockIndex int) {
	bpm.mu.Lock()
	defer bpm.mu.Unlock()

	delete(bpm.reservations, blockIndex)
}

// CleanupExpiredReservations removes expired reservations
func (bpm *BlockProposalManager) CleanupExpiredReservations() []int {
	bpm.mu.Lock()
	defer bpm.mu.Unlock()

	expired := []int{}
	now := time.Now()

	for blockIndex, reservation := range bpm.reservations {
		if now.After(reservation.TimeoutAt) {
			expired = append(expired, blockIndex)
			delete(bpm.reservations, blockIndex)
		}
	}

	return expired
}

// GetActiveProposals returns all active proposals
func (bpm *BlockProposalManager) GetActiveProposals() []*BlockProposal {
	bpm.mu.RLock()
	defer bpm.mu.RUnlock()

	proposals := make([]*BlockProposal, 0, len(bpm.proposals))
	for _, proposal := range bpm.proposals {
		proposals = append(proposals, proposal)
	}
	return proposals
}

// GetActiveReservations returns all active reservations
func (bpm *BlockProposalManager) GetActiveReservations() []*BlockReservation {
	bpm.mu.RLock()
	defer bpm.mu.RUnlock()

	reservations := make([]*BlockReservation, 0, len(bpm.reservations))
	for _, reservation := range bpm.reservations {
		reservations = append(reservations, reservation)
	}
	return reservations
}

// CalculateHash calculates hash for consensus messages
func (pg *PostGossip) CalculateHash() string {
	data := fmt.Sprintf("%s%s%d%s", pg.Post.Hash, pg.NodeID, pg.Timestamp, pg.Post.Signature)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CalculateHash calculates hash for block proposal
func (bp *BlockProposal) CalculateHash() string {
	// Sort post hashes for deterministic hashing
	sortedHashes := make([]string, len(bp.PostHashes))
	copy(sortedHashes, bp.PostHashes)
	sort.Strings(sortedHashes)

	data := fmt.Sprintf("%d%s%v%d%f", bp.Index, bp.ProposerID, sortedHashes, bp.Timestamp, bp.TrustScore)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CalculateHash calculates hash for block vote
func (bv *BlockVote) CalculateHash() string {
	data := fmt.Sprintf("%d%s%s%d%t", bv.Index, bv.ProposerID, bv.VoterID, bv.Timestamp, bv.Approved)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CalculateHash calculates hash for proposal expired message
func (pe *ProposalExpired) CalculateHash() string {
	data := fmt.Sprintf("%d%s%d", pe.Index, pe.ProposerID, pe.Timestamp)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
