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
const MainnetGenesisTimestamp = 1751485627

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
