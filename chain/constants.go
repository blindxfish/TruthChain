package chain

import (
	"fmt"
	"time"
)

// Mainnet consensus constants - DO NOT CHANGE without network upgrade
const (
	// Block creation rules
	MainnetMinPosts    = 5                // Minimum posts per block
	MainnetMaxWait     = 30 * time.Second // Maximum time to wait for posts
	MainnetVersionByte = 0x42             // Mainnet version byte

	// Genesis block hash - DO NOT CHANGE (hardcoded for chain identity)
	MainnetGenesisHash = "dfefc56cc8f5f1f1c825dc5a97f9b4f203b04fddddec0627f0ae391003b99705"

	// Network identifiers
	MainnetNetworkID = "truthchain-mainnet"
	TestnetNetworkID = "truthchain-testnet"
)

// Genesis block timestamp (Unix timestamp when TruthChain was created)
// This must remain constant for all mainnet nodes
const MainnetGenesisTimestamp = 1751485627

// Genesis block configuration - these values must remain constant
const (
	GenesisTimestamp = MainnetGenesisTimestamp // Use the same timestamp for consistency
	GenesisHash      = "0000000000000000000000000000000000000000000000000000000000000000"
	GenesisAuthor    = "Nik"
	GenesisContent   = "Block 0 - This is where censorship died."

	// Block configuration
	MaxBlockSize = 1024 * 1024 // 1MB max block size
	MaxPostSize  = 10000       // 10KB max post size

	// Character configuration
	CharacterThreshold = 1000 // Characters needed for block creation

	// Transfer configuration
	MaxTransferAmount = 1000000 // Maximum characters per transfer
	MinTransferAmount = 1       // Minimum characters per transfer

	// Network configuration
	MaxPeers   = 50
	MaxHops    = 10
	DefaultTTL = 10

	// Timeouts
	ConnectionTimeout = 30 * time.Second
	SyncTimeout       = 60 * time.Second
	PingInterval      = 30 * time.Second

	// Trust configuration
	DefaultTrustScore = 0.5
	MinTrustScore     = 0.1
	MaxTrustScore     = 1.0

	// Beacon configuration
	BeaconRewardMultiplier = 1.5 // 50% bonus for beacon nodes
	BeaconAnnounceInterval = 12 * time.Hour
	MaxBeaconAnnounces     = 1 // Per 12-hour period

	// Mesh configuration
	DefaultMeshPort = 9876
	DefaultSyncPort = 9877
	MaxMeshPeers    = 32

	// API configuration
	DefaultAPIPort = 8080
	APITimeout     = 30 * time.Second
)

// ValidateMainnetRules checks if the given parameters match mainnet consensus
func ValidateMainnetRules(postThreshold int, networkID string) error {
	if networkID == MainnetNetworkID {
		if postThreshold != MainnetMinPosts {
			return fmt.Errorf("mainnet requires exactly %d posts per block, got %d",
				MainnetMinPosts, postThreshold)
		}
	}
	return nil
}

// IsMainnetGenesis checks if a block is the mainnet genesis block
func IsMainnetGenesis(block *Block) bool {
	return block.Index == 0 &&
		block.Hash == MainnetGenesisHash &&
		block.Timestamp == MainnetGenesisTimestamp
}
