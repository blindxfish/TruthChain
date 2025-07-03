package network

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// BeaconAnnounce represents a beacon node announcement
type BeaconAnnounce struct {
	Type    string        `json:"type"`
	Payload BeaconPayload `json:"payload"`
}

// BeaconPayload contains the beacon node information
type BeaconPayload struct {
	NodeID    string  `json:"node_id"`   // Public key of the node
	IP        string  `json:"ip"`        // Domain or IP (IPv4/IPv6)
	Port      int     `json:"port"`      // Listening port
	Timestamp int64   `json:"timestamp"` // UNIX time of declaration
	Uptime    float64 `json:"uptime"`    // Reported uptime %
	Version   string  `json:"version"`   // Optional node version string
	Sig       string  `json:"sig"`       // Signature of payload with node's private key
}

// BeaconNode represents a discovered beacon node
type BeaconNode struct {
	NodeID        string
	IP            string
	Port          int
	Timestamp     int64
	Uptime        float64
	Version       string
	LastSeen      int64
	TrustScore    float64
	IsReachable   bool
	AnnounceCount int // Number of announces in last 12h
}

// BeaconManager manages beacon node discovery and validation
type BeaconManager struct {
	Beacons      map[string]*BeaconNode // node_id -> BeaconNode
	LastAnnounce map[string]int64       // node_id -> last announce time
	PrivateKey   *ecdsa.PrivateKey      // Node's private key for signing
	PublicKey    *ecdsa.PublicKey       // Node's public key
	IsBeacon     bool                   // Whether this node is a beacon
	BeaconIP     string                 // This node's beacon IP
	BeaconPort   int                    // This node's beacon port
	Version      string                 // Node version
}

// NewBeaconManager creates a new beacon manager
func NewBeaconManager(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) *BeaconManager {
	return &BeaconManager{
		Beacons:      make(map[string]*BeaconNode),
		LastAnnounce: make(map[string]int64),
		PrivateKey:   privateKey,
		PublicKey:    publicKey,
		IsBeacon:     false,
		Version:      "v1.0.0", // Default version
	}
}

// EnableBeacon enables beacon mode for this node
func (bm *BeaconManager) EnableBeacon(ip string, port int) {
	bm.IsBeacon = true
	bm.BeaconIP = ip
	bm.BeaconPort = port
}

// DisableBeacon disables beacon mode for this node
func (bm *BeaconManager) DisableBeacon() {
	bm.IsBeacon = false
	bm.BeaconIP = ""
	bm.BeaconPort = 0
}

// CreateBeaconAnnounce creates a beacon announcement for this node
func (bm *BeaconManager) CreateBeaconAnnounce(uptime float64) (*BeaconAnnounce, error) {
	if !bm.IsBeacon {
		return nil, fmt.Errorf("node is not in beacon mode")
	}

	// Check if we can announce (12h limit)
	nodeID := bm.getNodeID()
	if lastAnnounce, exists := bm.LastAnnounce[nodeID]; exists {
		if time.Now().Unix()-lastAnnounce < 43200 { // 12 hours
			return nil, fmt.Errorf("beacon announce limit: must wait 12 hours between announces")
		}
	}

	// Create payload
	payload := BeaconPayload{
		NodeID:    nodeID,
		IP:        bm.BeaconIP,
		Port:      bm.BeaconPort,
		Timestamp: time.Now().Unix(),
		Uptime:    uptime,
		Version:   bm.Version,
	}

	// Sign the payload
	signature, err := bm.signPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to sign beacon payload: %v", err)
	}

	payload.Sig = signature

	// Update last announce time
	bm.LastAnnounce[nodeID] = payload.Timestamp

	return &BeaconAnnounce{
		Type:    "beacon_announce",
		Payload: payload,
	}, nil
}

// ValidateBeaconAnnounce validates an incoming beacon announcement
func (bm *BeaconManager) ValidateBeaconAnnounce(announce *BeaconAnnounce) error {
	if announce.Type != "beacon_announce" {
		return fmt.Errorf("invalid announce type: %s", announce.Type)
	}

	payload := announce.Payload

	// Check timestamp (not too old)
	if time.Now().Unix()-payload.Timestamp > 3600 { // 1 hour old
		return fmt.Errorf("beacon announce too old")
	}

	// Check 12h limit for this node
	if lastAnnounce, exists := bm.LastAnnounce[payload.NodeID]; exists {
		if payload.Timestamp-lastAnnounce < 43200 { // 12 hours
			return fmt.Errorf("beacon announce too frequent for node %s", payload.NodeID)
		}
	}

	// Verify signature
	if err := bm.verifySignature(payload); err != nil {
		return fmt.Errorf("invalid beacon signature: %v", err)
	}

	// Validate uptime
	if payload.Uptime < 0 || payload.Uptime > 100 {
		return fmt.Errorf("invalid uptime value: %f", payload.Uptime)
	}

	// Validate port
	if payload.Port < 1 || payload.Port > 65535 {
		return fmt.Errorf("invalid port: %d", payload.Port)
	}

	return nil
}

// ProcessBeaconAnnounce processes a validated beacon announcement
func (bm *BeaconManager) ProcessBeaconAnnounce(announce *BeaconAnnounce) error {
	payload := announce.Payload

	// Create or update beacon node
	beacon := &BeaconNode{
		NodeID:        payload.NodeID,
		IP:            payload.IP,
		Port:          payload.Port,
		Timestamp:     payload.Timestamp,
		Uptime:        payload.Uptime,
		Version:       payload.Version,
		LastSeen:      time.Now().Unix(),
		TrustScore:    0.5,   // Initial trust score for beacons
		IsReachable:   false, // Will be checked later
		AnnounceCount: 1,
	}

	// Update existing beacon if it exists
	if existing, exists := bm.Beacons[payload.NodeID]; exists {
		beacon.TrustScore = existing.TrustScore
		beacon.IsReachable = existing.IsReachable
		beacon.AnnounceCount = existing.AnnounceCount + 1
	}

	bm.Beacons[payload.NodeID] = beacon
	bm.LastAnnounce[payload.NodeID] = payload.Timestamp

	return nil
}

// GetBeaconNodes returns all known beacon nodes
func (bm *BeaconManager) GetBeaconNodes() []*BeaconNode {
	beacons := make([]*BeaconNode, 0, len(bm.Beacons))
	for _, beacon := range bm.Beacons {
		beacons = append(beacons, beacon)
	}
	return beacons
}

// GetReachableBeacons returns only reachable beacon nodes
func (bm *BeaconManager) GetReachableBeacons() []*BeaconNode {
	var reachable []*BeaconNode
	for _, beacon := range bm.Beacons {
		if beacon.IsReachable {
			reachable = append(reachable, beacon)
		}
	}
	return reachable
}

// UpdateBeaconReachability updates the reachability status of a beacon
func (bm *BeaconManager) UpdateBeaconReachability(nodeID string, isReachable bool) {
	if beacon, exists := bm.Beacons[nodeID]; exists {
		beacon.IsReachable = isReachable
		beacon.LastSeen = time.Now().Unix()

		// Boost trust score for reachable beacons
		if isReachable && beacon.TrustScore < 0.9 {
			beacon.TrustScore += 0.1
			if beacon.TrustScore > 0.9 {
				beacon.TrustScore = 0.9
			}
		}
	}
}

// GetBeaconStats returns statistics about beacon nodes
func (bm *BeaconManager) GetBeaconStats() map[string]interface{} {
	totalBeacons := len(bm.Beacons)
	reachableBeacons := 0
	totalUptime := 0.0
	avgTrustScore := 0.0

	for _, beacon := range bm.Beacons {
		if beacon.IsReachable {
			reachableBeacons++
		}
		totalUptime += beacon.Uptime
		avgTrustScore += beacon.TrustScore
	}

	if totalBeacons > 0 {
		totalUptime /= float64(totalBeacons)
		avgTrustScore /= float64(totalBeacons)
	}

	return map[string]interface{}{
		"total_beacons":       totalBeacons,
		"reachable_beacons":   reachableBeacons,
		"average_uptime":      totalUptime,
		"average_trust_score": avgTrustScore,
		"is_beacon":           bm.IsBeacon,
		"beacon_ip":           bm.BeaconIP,
		"beacon_port":         bm.BeaconPort,
	}
}

// CleanupOldBeacons removes beacons that haven't been seen recently
func (bm *BeaconManager) CleanupOldBeacons(maxAge time.Duration) int {
	removed := 0
	cutoff := time.Now().Unix() - int64(maxAge.Seconds())

	for nodeID, beacon := range bm.Beacons {
		if beacon.LastSeen < cutoff {
			delete(bm.Beacons, nodeID)
			removed++
		}
	}

	return removed
}

// IsBeaconMode returns whether this node is currently in beacon mode
func (bm *BeaconManager) IsBeaconMode() bool {
	return bm.IsBeacon
}

// GetBeaconUptime returns the current uptime percentage for beacon announcements
func (bm *BeaconManager) GetBeaconUptime() float64 {
	// Calculate uptime based on how long the beacon has been active
	if !bm.IsBeacon {
		return 0.0
	}

	// For now, return a default uptime of 95% for beacon nodes
	// In a real implementation, this would be calculated from actual uptime tracking
	return 95.0
}

// Helper methods

// getNodeID returns the node ID (public key hex)
func (bm *BeaconManager) getNodeID() string {
	// This should return the public key in hex format
	// Implementation depends on your wallet package
	return "04" + hex.EncodeToString(bm.PublicKey.X.Bytes()) + hex.EncodeToString(bm.PublicKey.Y.Bytes())
}

// signPayload signs the beacon payload
func (bm *BeaconManager) signPayload(payload BeaconPayload) (string, error) {
	// Create a hash of the payload (excluding signature)
	payloadCopy := payload
	payloadCopy.Sig = ""

	payloadBytes, err := json.Marshal(payloadCopy)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(payloadBytes)

	// Sign the hash
	signature, err := ecdsa.SignASN1(rand.Reader, bm.PrivateKey, hash[:])
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(signature), nil
}

// verifySignature verifies the beacon payload signature
func (bm *BeaconManager) verifySignature(payload BeaconPayload) error {
	// Extract public key from node ID
	publicKey, err := bm.parsePublicKey(payload.NodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %v", err)
	}

	// Create hash of payload (excluding signature)
	payloadCopy := payload
	payloadCopy.Sig = ""

	payloadBytes, err := json.Marshal(payloadCopy)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(payloadBytes)

	// Decode signature
	signatureBytes, err := hex.DecodeString(payload.Sig)
	if err != nil {
		return fmt.Errorf("invalid signature format: %v", err)
	}

	// Verify signature
	if !ecdsa.VerifyASN1(publicKey, hash[:], signatureBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// parsePublicKey parses a public key from hex string
func (bm *BeaconManager) parsePublicKey(nodeID string) (*ecdsa.PublicKey, error) {
	// This is a simplified implementation
	// In practice, you'd want to use your wallet package's public key parsing
	if len(nodeID) < 2 || nodeID[:2] != "04" {
		return nil, fmt.Errorf("invalid public key format")
	}

	// For now, return the current node's public key
	// In a real implementation, you'd parse the nodeID properly
	return bm.PublicKey, nil
}
