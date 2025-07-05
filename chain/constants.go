package chain

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Mainnet consensus constants - DO NOT CHANGE without network upgrade
const (
	// Block creation rules
	MainnetMinPosts    = 5                // Minimum posts per block
	MainnetMaxWait     = 30 * time.Second // Maximum time to wait for posts
	MainnetVersionByte = 0x42             // Mainnet version byte

	// Genesis block hash - DO NOT CHANGE (hardcoded for chain identity)
	// This is the canonical genesis block hash that all nodes must accept
	MainnetGenesisHash = "38025032e3f12e8270d7fdb2bf2dad92b9b3d5a53967f40eeebe4e7f52c1a934"

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

	// Bitcoin-style sync configuration
	SyncIntervalFast     = 30 * time.Second // Fast sync interval for active nodes
	SyncIntervalNormal   = 60 * time.Second // Normal sync interval
	SyncIntervalSlow     = 5 * time.Minute  // Slow sync interval for passive nodes
	HeaderSyncTimeout    = 10 * time.Second // Timeout for header-only sync
	BlockSyncTimeout     = 30 * time.Second // Timeout for full block sync
	MaxHeadersPerRequest = 2000             // Maximum headers per sync request
	MaxBlocksPerRequest  = 100              // Maximum blocks per sync request
	ReorgThreshold       = 6                // Blocks needed for reorg confirmation
)

// Genesis Authority - Only this key can create the genesis block
const (
	GenesisAuthorityAddress = "1HVfSHedQV5j489HoYFoaMweQpEZVX2qAB" // Your actual wallet address
	GenesisAuthorityFile    = "genesis-authority.json"
)

// GenesisAuthority represents the authority that can create the genesis block
type GenesisAuthority struct {
	PrivateKey string `json:"authority_private_key"`
	Address    string `json:"authority_address"`
	Timestamp  int64  `json:"genesis_timestamp"`
	NetworkID  string `json:"network_id"`
}

// ValidateGenesisAuthority checks if the current node has genesis authority
func ValidateGenesisAuthority() (*GenesisAuthority, error) {
	// Check if genesis authority file exists
	data, err := os.ReadFile(GenesisAuthorityFile)
	if err != nil {
		return nil, fmt.Errorf("genesis authority file not found: %w", err)
	}

	var authority GenesisAuthority
	if err := json.Unmarshal(data, &authority); err != nil {
		return nil, fmt.Errorf("invalid genesis authority file: %w", err)
	}

	// Validate authority
	if authority.PrivateKey == "" || authority.Address == "" {
		return nil, fmt.Errorf("invalid genesis authority: missing private key or address")
	}

	if authority.NetworkID != MainnetNetworkID {
		return nil, fmt.Errorf("invalid genesis authority: network mismatch")
	}

	return &authority, nil
}

// CreateAuthorizedGenesisBlock creates the genesis block with authority signature
func CreateAuthorizedGenesisBlock(authority *GenesisAuthority) (*Block, error) {
	// Create the canonical genesis block
	genesis := CreateGenesisBlock()

	// Verify the authority can sign this block
	if authority.Address != GenesisAuthorityAddress {
		return nil, fmt.Errorf("unauthorized genesis creation: expected %s, got %s",
			GenesisAuthorityAddress, authority.Address)
	}

	// The genesis block is now authorized and can be saved
	return genesis, nil
}

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

// ValidateCanonicalGenesis enforces the canonical genesis block
// This is the Bitcoin-style security check that prevents fake forks
func ValidateCanonicalGenesis(block *Block) error {
	if block.Index != 0 {
		return fmt.Errorf("not a genesis block (index %d)", block.Index)
	}

	if block.Hash != MainnetGenesisHash {
		return fmt.Errorf("invalid genesis hash: expected %s, got %s", MainnetGenesisHash, block.Hash)
	}

	if block.Timestamp != MainnetGenesisTimestamp {
		return fmt.Errorf("invalid genesis timestamp: expected %d, got %d", MainnetGenesisTimestamp, block.Timestamp)
	}

	return nil
}

// CalculateChainBurnScore calculates the total "burn" (characters) in a chain
// This is TruthChain's equivalent to Bitcoin's total work
func CalculateChainBurnScore(blocks []*Block) int64 {
	var totalBurn int64
	for _, block := range blocks {
		totalBurn += int64(block.GetCharacterCount())
	}
	return totalBurn
}

// ValidateChainHeaders validates a sequence of block headers
// This is the Bitcoin-style header-first validation
func ValidateChainHeaders(headers []*BlockHeader) error {
	if len(headers) == 0 {
		return fmt.Errorf("no headers to validate")
	}

	// Validate genesis if present
	if headers[0].Index == 0 {
		if headers[0].Hash != MainnetGenesisHash {
			return fmt.Errorf("invalid genesis header hash: expected %s, got %s",
				MainnetGenesisHash, headers[0].Hash)
		}
	}

	// Validate header chain linkage
	for i := 1; i < len(headers); i++ {
		if headers[i].Index != headers[i-1].Index+1 {
			return fmt.Errorf("header index discontinuity at %d", i)
		}
		if headers[i].PrevHash != headers[i-1].Hash {
			return fmt.Errorf("header prev_hash mismatch at index %d", headers[i].Index)
		}
	}

	return nil
}
