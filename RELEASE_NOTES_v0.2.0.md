# TruthChain v0.2.0 - Bitcoin-Style Security Release

## 🚨 BREAKING CHANGES

This release introduces **Bitcoin-style security model** with strict genesis validation and enforced consensus. **All existing nodes must upgrade** to maintain network compatibility.

### ⚠️ Critical Changes
- **Genesis Block Enforcement**: Nodes now require the canonical genesis block hash
- **No Local Genesis Creation**: New nodes must sync from trusted peers
- **Strict Fork Prevention**: Invalid genesis blocks are rejected
- **Breaking API Changes**: Sync state tracking added to status endpoints

## 🛡️ New Bitcoin-Style Security Features

### Genesis Validation
- **Hardcoded Genesis Hash**: `38025032e3f12e8270d7fdb2bf2dad92b9b3d5a53967f40eeebe4e7f52c1a934`
- **Canonical Genesis Enforcement**: All nodes must validate against this hash
- **Fork Prevention**: Nodes with invalid genesis are rejected

### Bitcoin-Style Sync
- **Header-First Sync**: Download headers before full blocks (faster, safer)
- **Burn-Weight Chain Selection**: Prefer chains with higher character "burn" scores
- **Automatic Reorgs**: Switch to better chains automatically
- **Faster Sync Intervals**: 30-second sync for active nodes (was 5 minutes)

### Enhanced Startup Sequence
- **Initial Sync for New Nodes**: New nodes sync from trusted peers
- **Genesis Validation on Startup**: Existing nodes validate genesis block
- **Sync State Tracking**: API shows sync status
- **Timeout Protection**: 2-minute timeout for peer discovery

## 🔧 Technical Improvements

### Network Protocol
- **Header-Only Requests**: New `HeadersOnly` flag in sync requests
- **Enhanced Transport Layer**: Support for both header and block requests
- **Chain Validation**: Header chain validation with proper linkage
- **Fork Detection**: Improved fork detection and resolution

### Blockchain Implementation
- **Burn Score Calculation**: `CalculateChainBurnScore()` for chain comparison
- **Chain Integration**: `ValidateAndIntegrateChain()` with reorg support
- **Genesis Validation**: `ValidateCanonicalGenesis()` function
- **Header Validation**: `ValidateChainHeaders()` for header chains

### Constants & Configuration
```go
// New Bitcoin-style sync configuration
SyncIntervalFast     = 30 * time.Second  // Fast sync for active nodes
SyncIntervalNormal   = 60 * time.Second  // Normal sync interval  
SyncIntervalSlow     = 5 * time.Minute   // Slow sync for passive nodes
HeaderSyncTimeout    = 10 * time.Second  // Timeout for header-only sync
BlockSyncTimeout     = 30 * time.Second  // Timeout for full block sync
MaxHeadersPerRequest = 2000              // Maximum headers per sync request
MaxBlocksPerRequest  = 100               // Maximum blocks per sync request
ReorgThreshold       = 6                 // Blocks needed for reorg confirmation
```

## 📦 Build Information

### Linux Build
- **File**: `truthchain-v0.2.0-linux.zip`
- **Architecture**: x86_64
- **Dependencies**: None (statically linked)
- **Size**: ~6.2MB

### Windows Build
- **File**: `truthchain-v0.2.0-windows.zip`
- **Architecture**: x64
- **Dependencies**: None (statically linked)
- **Size**: ~6.2MB

## 🚀 Deployment Instructions

### New Node Setup
1. Download the appropriate build for your platform
2. Extract the archive
3. Run the binary: `./truthchain` (Linux) or `truthchain.exe` (Windows)
4. Follow the interactive setup
5. **Important**: Node will sync from trusted peers (may take time)

### Existing Node Upgrade
1. **Backup your data**: Copy your `truthchain.db` and wallet files
2. Stop the existing node
3. Replace the binary with v0.2.0
4. Start the node - it will validate genesis and continue normally
5. If genesis validation fails, you may need to sync from peers

### Mainnet Deployment
Use the updated `mainnet-deploy.sh` script for VPS deployment:
```bash
chmod +x mainnet-deploy.sh
sudo ./mainnet-deploy.sh
```

## 🔄 Migration Guide

### For Existing Nodes
- **Automatic Migration**: Most nodes will upgrade seamlessly
- **Genesis Validation**: Existing chains will be validated on startup
- **Sync Continuation**: Normal sync will continue after validation

### For New Nodes
- **No Local Genesis**: New nodes cannot create local genesis blocks
- **Peer Discovery**: Must discover and sync from trusted peers
- **Timeout Handling**: 2-minute timeout for peer discovery

## 🐛 Bug Fixes
- Fixed genesis hash mismatch between constants and actual block
- Removed legacy blockchain implementation (cleaner codebase)
- Fixed sync manager method visibility issues
- Improved error handling in startup sequence

## 📊 Performance Improvements
- **Faster Sync**: Header-first sync reduces bandwidth usage
- **Better Fork Resolution**: Burn-weight comparison prevents unnecessary reorgs
- **Reduced Memory Usage**: Cleaner codebase with removed legacy code
- **Improved Logging**: Better status messages and error reporting

## 🔍 Monitoring & Debugging

### New API Endpoints
- **Sync Status**: `/status` now includes `"syncing": boolean`
- **Enhanced Logging**: Better startup and sync progress messages
- **Error Reporting**: Clear error messages for sync failures

### Log Messages
- `⚠️ No blockchain found - starting Bitcoin-style initial sync`
- `✅ Genesis block validated - starting normally`
- `🔄 Starting initial sync from trusted peers...`
- `✅ Initial sync completed - chain length: X`

## 🛠️ Development Changes

### Removed Files
- `chain/blockchain.go` - Legacy in-memory implementation
- `chain/chain_test.go` - Legacy test files
- `chain/sync.go` - Legacy sync manager
- `chain/sync_test.go` - Legacy sync tests

### Updated Files
- `chain/constants.go` - Added Bitcoin-style constants
- `chain/types.go` - Added header-only sync types
- `blockchain/blockchain.go` - Bitcoin-style startup and validation
- `network/sync.go` - Header-first sync implementation
- `network/transport.go` - Header-only request support
- `cmd/main.go` - Enhanced startup sequence

## 🔐 Security Notes

### Genesis Block Security
- **Immutable Genesis**: Genesis block hash is hardcoded and cannot be changed
- **Network Consensus**: All nodes must agree on the same genesis block
- **Fork Prevention**: Invalid genesis blocks are rejected immediately

### Sync Security
- **Header Validation**: All headers are validated before downloading blocks
- **Burn-Weight Consensus**: Chains with higher character burn are preferred
- **Reorg Protection**: Automatic reorgs only when better chains are found

## 📋 Compatibility

### Breaking Changes
- **Genesis Block**: Must match canonical hash
- **Sync Protocol**: New header-first sync protocol
- **Startup Sequence**: New validation and sync requirements
- **API Changes**: Status endpoint includes sync state

### Backward Compatibility
- **Database Format**: Existing databases are compatible
- **Wallet Format**: Existing wallets work without changes
- **Network Protocol**: Enhanced but backward compatible

## 🎯 Next Steps

### Immediate Actions
1. **Upgrade All Nodes**: Deploy v0.2.0 to all mainnet nodes
2. **Monitor Network**: Watch for sync issues or genesis validation failures
3. **Update Documentation**: Ensure all documentation reflects new behavior

### Future Enhancements
- **Enhanced Reorg Logic**: More sophisticated chain reorganization
- **Peer Discovery**: Improved peer discovery mechanisms
- **Sync Optimization**: Further sync performance improvements

## 📞 Support

If you encounter issues during the upgrade:
1. Check the logs for specific error messages
2. Verify your genesis block matches the canonical hash
3. Ensure you have network connectivity for peer discovery
4. Contact the development team if problems persist

---

**Release Date**: December 2024  
**Version**: 0.2.0  
**Network**: TruthChain Mainnet  
**Security Level**: Bitcoin-style consensus enforcement 