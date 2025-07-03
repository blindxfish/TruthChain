package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"
)

func TestBeaconManager(t *testing.T) {
	// Generate test keys
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)

	// Test initial state
	if bm.IsBeacon {
		t.Error("Beacon should be disabled initially")
	}

	if len(bm.Beacons) != 0 {
		t.Error("Should start with no beacons")
	}
}

func TestEnableDisableBeacon(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)

	// Test enabling beacon
	bm.EnableBeacon("192.168.1.100", 8080)

	if !bm.IsBeacon {
		t.Error("Beacon should be enabled")
	}

	if bm.BeaconIP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", bm.BeaconIP)
	}

	if bm.BeaconPort != 8080 {
		t.Errorf("Expected port 8080, got %d", bm.BeaconPort)
	}

	// Test disabling beacon
	bm.DisableBeacon()

	if bm.IsBeacon {
		t.Error("Beacon should be disabled")
	}

	if bm.BeaconIP != "" {
		t.Error("Beacon IP should be empty")
	}

	if bm.BeaconPort != 0 {
		t.Error("Beacon port should be 0")
	}
}

func TestCreateBeaconAnnounce(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)
	bm.EnableBeacon("192.168.1.100", 8080)

	// Test creating beacon announce
	announce, err := bm.CreateBeaconAnnounce(95.5)
	if err != nil {
		t.Fatalf("Failed to create beacon announce: %v", err)
	}

	if announce.Type != "beacon_announce" {
		t.Errorf("Expected type 'beacon_announce', got %s", announce.Type)
	}

	payload := announce.Payload
	if payload.IP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", payload.IP)
	}

	if payload.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", payload.Port)
	}

	if payload.Uptime != 95.5 {
		t.Errorf("Expected uptime 95.5, got %f", payload.Uptime)
	}

	if payload.Version != "v1.0.0" {
		t.Errorf("Expected version v1.0.0, got %s", payload.Version)
	}

	if payload.Sig == "" {
		t.Error("Signature should not be empty")
	}

	// Test creating announce when not in beacon mode
	bm.DisableBeacon()
	_, err = bm.CreateBeaconAnnounce(95.5)
	if err == nil {
		t.Error("Should fail to create announce when not in beacon mode")
	}
}

func TestValidateBeaconAnnounce(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)
	bm.EnableBeacon("192.168.1.100", 8080)

	// Create a valid announce
	announce, err := bm.CreateBeaconAnnounce(95.5)
	if err != nil {
		t.Fatalf("Failed to create beacon announce: %v", err)
	}

	// Use a fresh BeaconManager for validation to avoid anti-spam logic
	bm2 := NewBeaconManager(privateKey, &privateKey.PublicKey)
	err = bm2.ValidateBeaconAnnounce(announce)
	if err != nil {
		t.Errorf("Valid announce should pass validation: %v", err)
	}

	// Test invalid type
	invalidAnnounce := *announce
	invalidAnnounce.Type = "invalid_type"
	err = bm2.ValidateBeaconAnnounce(&invalidAnnounce)
	if err == nil {
		t.Error("Invalid type should fail validation")
	}

	// Test old timestamp
	oldAnnounce := *announce
	oldAnnounce.Payload.Timestamp = time.Now().Unix() - 7200 // 2 hours ago
	err = bm2.ValidateBeaconAnnounce(&oldAnnounce)
	if err == nil {
		t.Error("Old announce should fail validation")
	}

	// Test invalid uptime
	invalidUptimeAnnounce := *announce
	invalidUptimeAnnounce.Payload.Uptime = 150.0 // > 100%
	err = bm2.ValidateBeaconAnnounce(&invalidUptimeAnnounce)
	if err == nil {
		t.Error("Invalid uptime should fail validation")
	}

	// Test invalid port
	invalidPortAnnounce := *announce
	invalidPortAnnounce.Payload.Port = 70000 // > 65535
	err = bm2.ValidateBeaconAnnounce(&invalidPortAnnounce)
	if err == nil {
		t.Error("Invalid port should fail validation")
	}
}

func TestProcessBeaconAnnounce(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)

	// Create a valid announce
	bm.EnableBeacon("192.168.1.100", 8080)
	announce, err := bm.CreateBeaconAnnounce(95.5)
	if err != nil {
		t.Fatalf("Failed to create beacon announce: %v", err)
	}

	// Process the announce
	err = bm.ProcessBeaconAnnounce(announce)
	if err != nil {
		t.Fatalf("Failed to process beacon announce: %v", err)
	}

	// Check that beacon was added
	beacons := bm.GetBeaconNodes()
	if len(beacons) != 1 {
		t.Errorf("Expected 1 beacon, got %d", len(beacons))
	}

	beacon := beacons[0]
	if beacon.IP != "192.168.1.100" {
		t.Errorf("Expected IP 192.168.1.100, got %s", beacon.IP)
	}

	if beacon.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", beacon.Port)
	}

	if beacon.Uptime != 95.5 {
		t.Errorf("Expected uptime 95.5, got %f", beacon.Uptime)
	}

	if beacon.TrustScore != 0.5 {
		t.Errorf("Expected initial trust score 0.5, got %f", beacon.TrustScore)
	}

	if beacon.IsReachable {
		t.Error("New beacon should not be reachable initially")
	}
}

func TestBeaconReachability(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)

	// Add a beacon
	bm.EnableBeacon("192.168.1.100", 8080)
	announce, err := bm.CreateBeaconAnnounce(95.5)
	if err != nil {
		t.Fatalf("Failed to create beacon announce: %v", err)
	}

	err = bm.ProcessBeaconAnnounce(announce)
	if err != nil {
		t.Fatalf("Failed to process beacon announce: %v", err)
	}

	nodeID := announce.Payload.NodeID

	// Test updating reachability
	bm.UpdateBeaconReachability(nodeID, true)

	beacons := bm.GetBeaconNodes()
	if len(beacons) != 1 {
		t.Fatalf("Expected 1 beacon, got %d", len(beacons))
	}

	beacon := beacons[0]
	if !beacon.IsReachable {
		t.Error("Beacon should be reachable")
	}

	if beacon.TrustScore <= 0.5 {
		t.Error("Trust score should be boosted for reachable beacon")
	}

	// Test getting reachable beacons
	reachableBeacons := bm.GetReachableBeacons()
	if len(reachableBeacons) != 1 {
		t.Errorf("Expected 1 reachable beacon, got %d", len(reachableBeacons))
	}
}

func TestBeaconStats(t *testing.T) {
	privateKey1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	bm1 := NewBeaconManager(privateKey1, &privateKey1.PublicKey)

	privateKey2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	bm2 := NewBeaconManager(privateKey2, &privateKey2.PublicKey)

	// Add first beacon
	bm1.EnableBeacon("192.168.1.100", 8080)
	announce1, _ := bm1.CreateBeaconAnnounce(95.5)
	bm1.ProcessBeaconAnnounce(announce1)

	// Add second beacon
	bm2.EnableBeacon("192.168.1.101", 8081)
	announce2, _ := bm2.CreateBeaconAnnounce(98.0)
	bm1.ProcessBeaconAnnounce(announce2) // Add to bm1's list for stats

	// Update reachability
	bm1.UpdateBeaconReachability(announce1.Payload.NodeID, true)
	bm1.UpdateBeaconReachability(announce2.Payload.NodeID, true)

	// Get stats
	stats := bm1.GetBeaconStats()

	if stats["total_beacons"].(int) != 2 {
		t.Errorf("Expected 2 total beacons, got %d", stats["total_beacons"])
	}

	if stats["reachable_beacons"].(int) != 2 {
		t.Errorf("Expected 2 reachable beacons, got %d", stats["reachable_beacons"])
	}

	if stats["is_beacon"].(bool) != true {
		t.Error("Should be in beacon mode")
	}

	if stats["beacon_ip"].(string) != "192.168.1.100" {
		t.Errorf("Expected beacon IP 192.168.1.100, got %s", stats["beacon_ip"])
	}

	if stats["beacon_port"].(int) != 8080 {
		t.Errorf("Expected beacon port 8080, got %d", stats["beacon_port"])
	}

	// Test average uptime calculation
	avgUptime := stats["average_uptime"].(float64)
	expectedUptime := (95.5 + 98.0) / 2.0
	if avgUptime != expectedUptime {
		t.Errorf("Expected average uptime %f, got %f", expectedUptime, avgUptime)
	}
}

func TestCleanupOldBeacons(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)

	// Add a beacon
	bm.EnableBeacon("192.168.1.100", 8080)
	announce, _ := bm.CreateBeaconAnnounce(95.5)
	bm.ProcessBeaconAnnounce(announce)

	// Manually set last seen to old time
	nodeID := announce.Payload.NodeID
	if beacon, exists := bm.Beacons[nodeID]; exists {
		beacon.LastSeen = time.Now().Unix() - 86400 // 1 day ago
	}

	// Cleanup old beacons (older than 12 hours)
	removed := bm.CleanupOldBeacons(12 * time.Hour)
	if removed != 1 {
		t.Errorf("Expected 1 beacon removed, got %d", removed)
	}

	if len(bm.Beacons) != 0 {
		t.Error("All old beacons should be removed")
	}
}

func TestBeaconAnnounceLimit(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	bm := NewBeaconManager(privateKey, &privateKey.PublicKey)
	bm.EnableBeacon("192.168.1.100", 8080)

	// Create first announce
	announce1, err := bm.CreateBeaconAnnounce(95.5)
	if err != nil {
		t.Fatalf("Failed to create first beacon announce: %v", err)
	}

	// Try to create second announce immediately (should fail)
	_, err = bm.CreateBeaconAnnounce(95.5)
	if err == nil {
		t.Error("Should fail to create second announce within 12 hours")
	}

	// Process the first announce
	bm.ProcessBeaconAnnounce(announce1)

	// Check that the announce was recorded
	nodeID := announce1.Payload.NodeID
	if lastAnnounce, exists := bm.LastAnnounce[nodeID]; !exists {
		t.Error("Last announce time should be recorded")
	} else if lastAnnounce != announce1.Payload.Timestamp {
		t.Error("Last announce time should match announce timestamp")
	}
}
