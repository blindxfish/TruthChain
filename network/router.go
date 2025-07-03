package network

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// MessageRouter handles message propagation and duplicate prevention
type MessageRouter struct {
	Network         *TrustNetwork
	DuplicateFilter *DuplicateFilter
	SpamProtection  *SpamProtection
	mu              sync.RWMutex
}

// DuplicateFilter prevents duplicate messages from being processed
type DuplicateFilter struct {
	RecentMessages map[string]time.Time
	TTL            time.Duration
	mu             sync.RWMutex
}

// SpamProtection prevents spam and rate limiting
type SpamProtection struct {
	MessageCounts map[string]int
	LastReset     time.Time
	MaxMessages   int
	WindowSize    time.Duration
	mu            sync.RWMutex
}

// NewMessageRouter creates a new message router
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		DuplicateFilter: &DuplicateFilter{
			RecentMessages: make(map[string]time.Time),
			TTL:            5 * time.Minute, // 5 minutes TTL
		},
		SpamProtection: &SpamProtection{
			MessageCounts: make(map[string]int),
			LastReset:     time.Now(),
			MaxMessages:   100,             // Max 100 messages per window
			WindowSize:    1 * time.Minute, // 1 minute window
		},
	}
}

// RouteMessage routes a message to appropriate peers
func (mr *MessageRouter) RouteMessage(msg NetworkMessage) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// Check for duplicates
	if mr.DuplicateFilter.IsDuplicate(msg) {
		return nil // Silently ignore duplicates
	}

	// Check spam protection
	if mr.SpamProtection.IsSpam(msg.Source) {
		return nil // Silently ignore spam
	}

	// Add to duplicate filter
	mr.DuplicateFilter.AddMessage(msg)

	// Add to spam protection
	mr.SpamProtection.AddMessage(msg.Source)

	// Route based on message type
	switch msg.Type {
	case MessageTypeGossip:
		return mr.routeGossipMessage(msg)
	case MessageTypePost:
		return mr.routePostMessage(msg)
	case MessageTypeTransfer:
		return mr.routeTransferMessage(msg)
	case MessageTypeBlock:
		return mr.routeBlockMessage(msg)
	case MessageTypePing:
		return mr.routePingMessage(msg)
	case MessageTypePong:
		return mr.routePongMessage(msg)
	default:
		return nil
	}
}

// routeGossipMessage routes gossip messages to all peers
func (mr *MessageRouter) routeGossipMessage(msg NetworkMessage) error {
	// Gossip messages go to all connected peers
	peers := mr.Network.Topology.SelectPeers(mr.Network.MaxPeers)

	for _, peer := range peers {
		// Skip the source peer
		if peer.Address == msg.Source {
			continue
		}

		// Send to peer (implementation will be added)
		mr.sendToPeer(peer, msg)
	}

	return nil
}

// routePostMessage routes post messages to trusted peers
func (mr *MessageRouter) routePostMessage(msg NetworkMessage) error {
	// Post messages go to trusted peers only
	peers := mr.Network.Topology.SelectPeers(mr.Network.MaxPeers)

	for _, peer := range peers {
		// Skip the source peer
		if peer.Address == msg.Source {
			continue
		}

		// Only send to trusted peers
		if mr.Network.TrustEngine.IsTrusted(peer, mr.Network.MinTrustScore) {
			mr.sendToPeer(peer, msg)
		}
	}

	return nil
}

// routeTransferMessage routes transfer messages to trusted peers
func (mr *MessageRouter) routeTransferMessage(msg NetworkMessage) error {
	// Transfer messages go to trusted peers only
	peers := mr.Network.Topology.SelectPeers(mr.Network.MaxPeers)

	for _, peer := range peers {
		// Skip the source peer
		if peer.Address == msg.Source {
			continue
		}

		// Only send to trusted peers
		if mr.Network.TrustEngine.IsTrusted(peer, mr.Network.MinTrustScore) {
			mr.sendToPeer(peer, msg)
		}
	}

	return nil
}

// routeBlockMessage routes block messages to all peers
func (mr *MessageRouter) routeBlockMessage(msg NetworkMessage) error {
	// Block messages go to all connected peers
	peers := mr.Network.Topology.SelectPeers(mr.Network.MaxPeers)

	for _, peer := range peers {
		// Skip the source peer
		if peer.Address == msg.Source {
			continue
		}

		mr.sendToPeer(peer, msg)
	}

	return nil
}

// routePingMessage routes ping messages to specific peer
func (mr *MessageRouter) routePingMessage(msg NetworkMessage) error {
	// Ping messages go to specific peer (implementation will be added)
	return nil
}

// routePongMessage routes pong messages to specific peer
func (mr *MessageRouter) routePongMessage(msg NetworkMessage) error {
	// Pong messages go to specific peer (implementation will be added)
	return nil
}

// sendToPeer sends a message to a specific peer
func (mr *MessageRouter) sendToPeer(peer *Peer, msg NetworkMessage) {
	// This will be implemented when we add the actual network transport layer
	// For now, just log the message
	// log.Printf("Would send %s message to peer %s", msg.Type, peer.Address)
}

// DuplicateFilter methods

// IsDuplicate checks if a message is a duplicate
func (df *DuplicateFilter) IsDuplicate(msg NetworkMessage) bool {
	df.mu.RLock()
	defer df.mu.RUnlock()

	msgHash := df.getMessageHash(msg)
	if lastSeen, exists := df.RecentMessages[msgHash]; exists {
		return time.Since(lastSeen) < df.TTL
	}

	return false
}

// AddMessage adds a message to the duplicate filter
func (df *DuplicateFilter) AddMessage(msg NetworkMessage) {
	df.mu.Lock()
	defer df.mu.Unlock()

	msgHash := df.getMessageHash(msg)
	df.RecentMessages[msgHash] = time.Now()

	// Clean up old entries
	df.cleanup()
}

// cleanup removes old entries from the duplicate filter
func (df *DuplicateFilter) cleanup() {
	cutoff := time.Now().Add(-df.TTL)

	for hash, timestamp := range df.RecentMessages {
		if timestamp.Before(cutoff) {
			delete(df.RecentMessages, hash)
		}
	}
}

// getMessageHash creates a hash for a message
func (df *DuplicateFilter) getMessageHash(msg NetworkMessage) string {
	// Create a unique hash based on message content
	content := fmt.Sprintf("%d-%s-%d", msg.Type, msg.Source, msg.Timestamp)

	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// SpamProtection methods

// IsSpam checks if a source is sending too many messages
func (sp *SpamProtection) IsSpam(source string) bool {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Reset counters if window has passed
	if time.Since(sp.LastReset) > sp.WindowSize {
		sp.MessageCounts = make(map[string]int)
		sp.LastReset = time.Now()
	}

	count := sp.MessageCounts[source]
	return count > sp.MaxMessages
}

// AddMessage adds a message from a source
func (sp *SpamProtection) AddMessage(source string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Reset counters if window has passed
	if time.Since(sp.LastReset) > sp.WindowSize {
		sp.MessageCounts = make(map[string]int)
		sp.LastReset = time.Now()
	}

	sp.MessageCounts[source]++
}

// GetStats returns statistics about the message router
func (mr *MessageRouter) GetStats() map[string]interface{} {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	df := mr.DuplicateFilter
	sp := mr.SpamProtection

	df.mu.RLock()
	duplicateCount := len(df.RecentMessages)
	df.mu.RUnlock()

	sp.mu.RLock()
	spamCount := len(sp.MessageCounts)
	sp.mu.RUnlock()

	return map[string]interface{}{
		"duplicate_filter_size":        duplicateCount,
		"duplicate_filter_ttl":         df.TTL.Seconds(),
		"spam_protection_sources":      spamCount,
		"spam_protection_max_messages": sp.MaxMessages,
		"spam_protection_window":       sp.WindowSize.Seconds(),
	}
}
