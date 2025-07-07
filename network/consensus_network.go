package network

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/blindxfish/truthchain/chain"
)

// ConsensusNetwork integrates consensus with the mesh network
type ConsensusNetwork struct {
	meshManager     *MeshManager
	consensusEngine *chain.ConsensusEngine
	syncManager     *chain.SyncManager
	nodeID          string

	// Message routing
	messageHandlers map[string]func([]byte, string) error

	// State
	isRunning bool
	mu        sync.RWMutex

	// Channels
	stopChan chan struct{}
}

// ConsensusMessage represents a consensus message sent over the network
type ConsensusMessage struct {
	Type      string          `json:"type"`      // Message type
	NodeID    string          `json:"node_id"`   // Sender node ID
	Timestamp int64           `json:"timestamp"` // Message timestamp
	Payload   json.RawMessage `json:"payload"`   // Message payload
	Signature string          `json:"signature"` // Message signature
}

// Message types
const (
	MessageTypePostGossip       = "post_gossip"
	MessageTypeBlockProposal    = "block_proposal"
	MessageTypeBlockVote        = "block_vote"
	MessageTypeBlockCreated     = "block_created"
	MessageTypeProposalExpired  = "proposal_expired"
	MessageTypeBlockRequest     = "block_request"
	MessageTypeBlockResponse    = "block_response"
	MessageTypeChainTipQuery    = "chain_tip_query"
	MessageTypeChainTipResponse = "chain_tip_response"
)

// NewConsensusNetwork creates a new consensus network integration
func NewConsensusNetwork(meshManager *MeshManager, consensusEngine *chain.ConsensusEngine, syncManager *chain.SyncManager, nodeID string) *ConsensusNetwork {
	cn := &ConsensusNetwork{
		meshManager:     meshManager,
		consensusEngine: consensusEngine,
		syncManager:     syncManager,
		nodeID:          nodeID,
		messageHandlers: make(map[string]func([]byte, string) error),
		stopChan:        make(chan struct{}),
	}

	// Register message handlers
	cn.registerMessageHandlers()

	// Set network callbacks for consensus engine
	consensusEngine.SetNetworkCallbacks(
		cn.handlePostGossipOutbound,
		cn.handleBlockProposalOutbound,
		cn.handleBlockVoteOutbound,
		cn.handleBlockCreatedOutbound,
		cn.handleProposalExpiredOutbound,
	)

	// Set network callbacks for sync manager
	if syncManager != nil {
		syncManager.SetNetworkCallbacks(
			cn.handleBlockRequestOutbound,
			cn.handleBlockResponseOutbound,
		)
	}

	return cn
}

// Start starts the consensus network
func (cn *ConsensusNetwork) Start() error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if cn.isRunning {
		return fmt.Errorf("consensus network already running")
	}

	cn.isRunning = true

	// Start message processing
	go cn.messageProcessor()

	log.Printf("[ConsensusNetwork] Started consensus network for node %s", cn.nodeID)
	return nil
}

// Stop stops the consensus network
func (cn *ConsensusNetwork) Stop() error {
	cn.mu.Lock()
	defer cn.mu.Unlock()

	if !cn.isRunning {
		return nil
	}

	cn.isRunning = false
	close(cn.stopChan)

	log.Printf("[ConsensusNetwork] Stopped consensus network for node %s", cn.nodeID)
	return nil
}

// registerMessageHandlers registers handlers for different message types
func (cn *ConsensusNetwork) registerMessageHandlers() {
	cn.messageHandlers[MessageTypePostGossip] = cn.handlePostGossipInbound
	cn.messageHandlers[MessageTypeBlockProposal] = cn.handleBlockProposalInbound
	cn.messageHandlers[MessageTypeBlockVote] = cn.handleBlockVoteInbound
	cn.messageHandlers[MessageTypeBlockCreated] = cn.handleBlockCreatedInbound
	cn.messageHandlers[MessageTypeProposalExpired] = cn.handleProposalExpiredInbound
	cn.messageHandlers[MessageTypeBlockRequest] = cn.handleBlockRequestInbound
	cn.messageHandlers[MessageTypeBlockResponse] = cn.handleBlockResponseInbound
	cn.messageHandlers[MessageTypeChainTipQuery] = cn.handleChainTipQueryInbound
	cn.messageHandlers[MessageTypeChainTipResponse] = cn.handleChainTipResponseInbound
}

// messageProcessor processes incoming consensus messages
func (cn *ConsensusNetwork) messageProcessor() {
	for {
		select {
		case <-cn.stopChan:
			return
		default:
			// Process messages from mesh manager
			// This would integrate with the existing mesh message system
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// BroadcastMessage broadcasts a consensus message to all peers
func (cn *ConsensusNetwork) BroadcastMessage(messageType string, payload interface{}) error {
	message := &ConsensusMessage{
		Type:      messageType,
		NodeID:    cn.nodeID,
		Timestamp: time.Now().Unix(),
		Signature: "", // TODO: Sign the message
	}

	// Serialize payload
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	message.Payload = payloadData

	// Serialize full message
	messageData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Broadcast via mesh manager
	if err := cn.meshManager.BroadcastMessage(messageData); err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	log.Printf("[ConsensusNetwork] Broadcasted %s message", messageType)
	return nil
}

// SendMessageToPeer sends a consensus message to a specific peer
func (cn *ConsensusNetwork) SendMessageToPeer(peerID string, messageType string, payload interface{}) error {
	message := &ConsensusMessage{
		Type:      messageType,
		NodeID:    cn.nodeID,
		Timestamp: time.Now().Unix(),
		Signature: "", // TODO: Sign the message
	}

	// Serialize payload
	payloadData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	message.Payload = payloadData

	// Serialize full message
	messageData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send via mesh manager
	if err := cn.meshManager.SendMessageToPeer(peerID, messageData); err != nil {
		return fmt.Errorf("failed to send message to peer %s: %w", peerID, err)
	}

	log.Printf("[ConsensusNetwork] Sent %s message to peer %s", messageType, peerID)
	return nil
}

// HandleIncomingMessage handles an incoming consensus message
func (cn *ConsensusNetwork) HandleIncomingMessage(messageData []byte, sourcePeer string) error {
	var message ConsensusMessage
	if err := json.Unmarshal(messageData, &message); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Validate message
	if err := cn.validateMessage(&message); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Route to appropriate handler
	handler, exists := cn.messageHandlers[message.Type]
	if !exists {
		return fmt.Errorf("unknown message type: %s", message.Type)
	}

	return handler(message.Payload, sourcePeer)
}

// validateMessage validates an incoming consensus message
func (cn *ConsensusNetwork) validateMessage(message *ConsensusMessage) error {
	if message.Type == "" {
		return fmt.Errorf("empty message type")
	}

	if message.NodeID == "" {
		return fmt.Errorf("empty node ID")
	}

	if message.NodeID == cn.nodeID {
		return fmt.Errorf("message from self")
	}

	if message.Timestamp <= 0 {
		return fmt.Errorf("invalid timestamp")
	}

	// Check if message is too old (e.g., more than 1 hour)
	if time.Now().Unix()-message.Timestamp > 3600 {
		return fmt.Errorf("message too old")
	}

	return nil
}

// Outbound message handlers (called by consensus engine)

func (cn *ConsensusNetwork) handlePostGossipOutbound(gossip *chain.PostGossip) error {
	return cn.BroadcastMessage(MessageTypePostGossip, gossip)
}

func (cn *ConsensusNetwork) handleBlockProposalOutbound(proposal *chain.BlockProposal) error {
	return cn.BroadcastMessage(MessageTypeBlockProposal, proposal)
}

func (cn *ConsensusNetwork) handleBlockVoteOutbound(vote *chain.BlockVote) error {
	return cn.BroadcastMessage(MessageTypeBlockVote, vote)
}

func (cn *ConsensusNetwork) handleBlockCreatedOutbound(block *chain.Block) error {
	return cn.BroadcastMessage(MessageTypeBlockCreated, block)
}

func (cn *ConsensusNetwork) handleProposalExpiredOutbound(expired *chain.ProposalExpired) error {
	return cn.BroadcastMessage(MessageTypeProposalExpired, expired)
}

// Inbound message handlers (called by message processor)

func (cn *ConsensusNetwork) handlePostGossipInbound(payload []byte, sourcePeer string) error {
	var gossip chain.PostGossip
	if err := json.Unmarshal(payload, &gossip); err != nil {
		return fmt.Errorf("failed to unmarshal post gossip: %w", err)
	}

	return cn.consensusEngine.HandlePostGossip(&gossip)
}

func (cn *ConsensusNetwork) handleBlockProposalInbound(payload []byte, sourcePeer string) error {
	var proposal chain.BlockProposal
	if err := json.Unmarshal(payload, &proposal); err != nil {
		return fmt.Errorf("failed to unmarshal block proposal: %w", err)
	}

	return cn.consensusEngine.HandleBlockProposal(&proposal)
}

func (cn *ConsensusNetwork) handleBlockVoteInbound(payload []byte, sourcePeer string) error {
	var vote chain.BlockVote
	if err := json.Unmarshal(payload, &vote); err != nil {
		return fmt.Errorf("failed to unmarshal block vote: %w", err)
	}

	return cn.consensusEngine.HandleBlockVote(&vote)
}

func (cn *ConsensusNetwork) handleBlockCreatedInbound(payload []byte, sourcePeer string) error {
	var block chain.Block
	if err := json.Unmarshal(payload, &block); err != nil {
		return fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return cn.consensusEngine.HandleBlockCreated(&block)
}

func (cn *ConsensusNetwork) handleProposalExpiredInbound(payload []byte, sourcePeer string) error {
	var expired chain.ProposalExpired
	if err := json.Unmarshal(payload, &expired); err != nil {
		return fmt.Errorf("failed to unmarshal proposal expired: %w", err)
	}

	// Handle proposal expiration (e.g., remove from local proposal manager)
	log.Printf("[ConsensusNetwork] Received proposal expired for block %d from %s", expired.Index, expired.ProposerID)
	return nil
}

// Sync-related outbound handlers

func (cn *ConsensusNetwork) handleBlockRequestOutbound(request *chain.BlockRequest) error {
	return cn.BroadcastMessage(MessageTypeBlockRequest, request)
}

func (cn *ConsensusNetwork) handleBlockResponseOutbound(response *chain.BlockResponse) error {
	return cn.BroadcastMessage(MessageTypeBlockResponse, response)
}

// Sync-related inbound handlers

func (cn *ConsensusNetwork) handleBlockRequestInbound(payload []byte, sourcePeer string) error {
	var request chain.BlockRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return fmt.Errorf("failed to unmarshal block request: %w", err)
	}

	if cn.syncManager != nil {
		return cn.syncManager.HandleBlockRequest(&request)
	}
	return nil
}

func (cn *ConsensusNetwork) handleBlockResponseInbound(payload []byte, sourcePeer string) error {
	var response chain.BlockResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return fmt.Errorf("failed to unmarshal block response: %w", err)
	}

	if cn.syncManager != nil {
		return cn.syncManager.HandleBlockResponse(&response)
	}
	return nil
}

func (cn *ConsensusNetwork) handleChainTipQueryInbound(payload []byte, sourcePeer string) error {
	// TODO: Implement chain tip query handling
	log.Printf("[ConsensusNetwork] Received chain tip query from %s", sourcePeer)
	return nil
}

func (cn *ConsensusNetwork) handleChainTipResponseInbound(payload []byte, sourcePeer string) error {
	// TODO: Implement chain tip response handling
	log.Printf("[ConsensusNetwork] Received chain tip response from %s", sourcePeer)
	return nil
}

// GetStats returns consensus network statistics
func (cn *ConsensusNetwork) GetStats() map[string]interface{} {
	cn.mu.RLock()
	defer cn.mu.RUnlock()

	consensusStats := cn.consensusEngine.GetStats()
	meshStats := cn.meshManager.GetStats()

	return map[string]interface{}{
		"node_id":          cn.nodeID,
		"is_running":       cn.isRunning,
		"consensus":        consensusStats,
		"mesh":             meshStats,
		"message_handlers": len(cn.messageHandlers),
	}
}

// GetActivePeers returns the list of active peers
func (cn *ConsensusNetwork) GetActivePeers() []string {
	return cn.meshManager.GetActivePeers()
}

// GetPeerCount returns the number of connected peers
func (cn *ConsensusNetwork) GetPeerCount() int {
	return cn.meshManager.GetPeerCount()
}

// IsPeerConnected checks if a peer is connected
func (cn *ConsensusNetwork) IsPeerConnected(peerID string) bool {
	return cn.meshManager.IsPeerConnected(peerID)
}
