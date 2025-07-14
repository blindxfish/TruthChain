# TruthChain v0.2.5 Release Notes

**Release Date**: December 2024  
**Version**: 0.2.5  
**Status**: Development Release - Not for Production Use

## 🎉 What's New in v0.2.5

This release represents a significant milestone in TruthChain's development, bringing major stability improvements and fixing critical issues that were preventing proper network operation. Version 0.2.5 is the most stable and reliable release to date.

## 🚀 Key Features

### ✅ **Fully Implemented & Working**

#### Core Blockchain Infrastructure
- **Complete Wallet System**: ECDSA key generation, signing, and secure storage with Base58Check addresses
- **Block & Post Logic**: Cryptographic hashing, signing, and verification with secure signature recovery
- **Transfer System**: Signed character transfers with validation, state management, and replay protection
- **Local Storage**: BoltDB-based persistent storage with mempool persistence
- **HTTP API**: Comprehensive local API interface for frontend integration
- **State Manager**: Complete wallet states, balances, and nonce tracking

#### Network Layer
- **Mesh Network**: Robust peer-to-peer communication and block synchronization
- **Beacon System**: Network discovery and public node announcements
- **Trust-based Peer Management**: Dynamic peer scoring and intelligent connection management
- **Block Synchronization**: Cross-node block sharing with validation

#### Consensus System
- **Post Gossip Protocol**: Network-wide post distribution before block creation
- **Block Proposal & Voting**: Democratic consensus through proposal submission and voting
- **Trust-based Proposer Selection**: Nodes with higher trust scores can propose blocks
- **Dynamic Trust Score Management**: Trust scores based on node behavior and reliability
- **Forkless Consensus**: No posts or burned characters are ever lost

#### User Experience
- **Interactive Setup Wizard**: Guided configuration for new users
- **Network Selection**: Mainnet, Testnet, or Local network modes
- **Node Mode Configuration**: API, Mesh, Beacon, and Mining modes
- **Wallet Management**: Create, import, backup, and restore wallets
- **Bitcoin-style Restart**: No crashes, loads existing data automatically

## 🔧 Critical Fixes in v0.2.5

### Time-Based Block Generation ✅ **FIXED**
- **Issue**: Time-based block generation was completely disabled, preventing proper block creation flow
- **Fix**: Re-enabled `checkTimeBasedBlock()` method with comprehensive conditions
- **Impact**: Nodes now properly request time-based blocks every 10+ minutes when conditions are met

### Consensus Protocol Integration ✅ **FIXED**
- **Issue**: Time-based blocks were bypassing the consensus protocol entirely
- **Fix**: Removed legacy time-based block creation logic that bypassed consensus
- **Impact**: All blocks now properly go through the consensus protocol with peer approval

### Peer Counting Logic ✅ **FIXED**
- **Issue**: Nodes incorrectly counted themselves as peers, causing vote count mismatches
- **Fix**: Updated peer counting logic to properly distinguish between voters and peers
- **Impact**: Accurate vote approval thresholds and proper consensus operation

### Mutex Locking Issues ✅ **FIXED**
- **Issue**: "sync: Unlock of unlocked RWMutex" crashes in `RequestTimeBasedBlock()`
- **Fix**: Corrected mutex locking/unlocking sequence to prevent double unlocks
- **Impact**: Stable block request handling without crashes

### Chain Tip Validation ✅ **FIXED**
- **Issue**: Fresh nodes with chain tip -1 were rejected during synchronization
- **Fix**: Updated chain tip validation to accept -1 as valid for new nodes
- **Impact**: New nodes can properly sync with the network from genesis

### Message Parsing Robustness ✅ **FIXED**
- **Issue**: JSON unmarshaling errors with "invalid character 'P' after top-level value"
- **Fix**: Enhanced mesh manager message processing for multiple JSON objects
- **Impact**: Robust message handling without parsing errors

### Time Synchronization ✅ **FIXED**
- **Issue**: Race conditions between block builder's internal time and blockchain state
- **Fix**: Modified `BuildTimeBasedBlock` to accept actual latest block timestamp
- **Impact**: Consistent time checks across the system

### Network Stability ✅ **IMPROVED**
- **Issue**: Crashes in `handleGossipMessage` due to nil pointer dereferences
- **Fix**: Added comprehensive nil checks for `msg.Source` in all message handlers
- **Impact**: Robust message handling without crashes

## 🌐 Network Modes

### Mainnet (Production) - **NOT LIVE YET**
- **Status**: Development complete, awaiting launch
- **Post Threshold**: 5 posts per block
- **Network ID**: `truthchain-mainnet`
- **Consensus Rules**: Fixed for compatibility
- **Use Case**: Production environment (when launched)

### Testnet (Development) ✅ **READY FOR TESTING**
- **Status**: Available for testing with improved stability
- **Post Threshold**: 3 posts per block
- **Network ID**: `truthchain-testnet`
- **Consensus Rules**: Relaxed for testing
- **Use Case**: Development and testing

### Local (Isolated) ✅ **FULLY FUNCTIONAL**
- **Status**: Fully functional with all features working
- **Post Threshold**: 2 posts per block
- **Network ID**: `truthchain-local`
- **Consensus Rules**: Minimal for local testing
- **Use Case**: Local development and testing

## 🔧 Node Modes

### API Mode ✅ **WORKING**
- **Purpose**: HTTP API server for frontend integration
- **Port**: 8080 (default)
- **Features**: Post creation, balance checking, transfers, wallet management
- **Status**: Fully implemented and functional

### Mesh Mode ✅ **WORKING**
- **Purpose**: Peer-to-peer network communication and consensus
- **Port**: 9876 (default)
- **Features**: Block sync, post propagation, peer discovery, consensus voting
- **Status**: Fully implemented and functional with improved stability

### Beacon Mode ✅ **WORKING**
- **Purpose**: Network discovery and public announcements
- **Requirements**: Public IP and domain
- **Features**: +50% character reward bonus (when mainnet is live)
- **Status**: Implemented but not active on mainnet

### Mining Mode ✅ **WORKING**
- **Purpose**: Uptime-based character mining
- **Requirements**: 80% uptime over 24 hours
- **Features**: Automatic character rewards every 10 minutes
- **Status**: Implemented but not active on mainnet

## 📡 API Reference

**Available API Endpoints:**

| Method | Endpoint | Description | Status |
|--------|----------|-------------|---------|
| `GET` | `/status` | Node and blockchain status | ✅ Working |
| `GET` | `/health` | Health check | ✅ Working |
| `GET` | `/info` | Node information | ✅ Working |
| `GET` | `/wallets/{address}` | Wallet information | ✅ Working |
| `GET` | `/wallets/{address}/balance` | Wallet balance | ✅ Working |
| `GET` | `/wallets/{address}/backup` | Download wallet backup | ✅ Working |
| `POST` | `/posts` | Create a new post | ✅ Working |
| `GET` | `/posts/pending` | Get pending posts | ✅ Working |
| `POST` | `/transfers` | Send characters | ✅ Working |
| `GET` | `/transfers/pending` | Get pending transfers | ✅ Working |
| `GET` | `/blockchain/latest` | Latest block | ✅ Working |
| `GET` | `/blockchain/length` | Chain length | ✅ Working |
| `GET` | `/network/stats` | Network statistics | ✅ Working |
| `GET` | `/sync/status` | Sync status | ✅ Working |

## 🚀 Getting Started

### Prerequisites
- Go 1.19 or higher
- Windows, Linux, or macOS

### Installation & Setup
```bash
# Clone the repository
git clone https://github.com/blindxfish/truthchain.git
cd truthchain

# Build the application
go build -o truthchain_server cmd/main.go

# Run TruthChain (interactive setup)
./truthchain_server

# First time: Interactive setup guides you through:
# 1. Create or import a wallet
# 2. Select network (Mainnet/Testnet/Local)
# 3. Choose node modes (API/Mesh/Beacon/Mining)
# 4. Configure ports and settings
# 5. Join the decentralized consensus network
# 6. Configuration automatically saved for future starts
```

## 🔐 Security Features

### Implemented Security
- **ECDSA Signatures**: All posts and transfers cryptographically signed
- **Public Key Recovery**: Signature verification with authorship validation
- **Local API Only**: No exposed network ports by default (127.0.0.1)
- **Wallet Security**: Proper file permissions and backup functionality
- **Nonce Protection**: Replay attack prevention
- **Hash Verification**: Block and post integrity validation

### Security Best Practices
- **Backup your wallet**: Save `YourWalletInfo.txt` in multiple secure locations
- **Protect your private key**: Never share it with anyone
- **Use secure environments**: Clean computers with updated software
- **Firewall configuration**: Only open necessary ports (8080 for API, 9876 for mesh)

## 📊 Performance Expectations

### Current Capabilities
- **Block Creation**: Time-based blocks every 10+ minutes (when conditions met)
- **Post Processing**: Immediate post acceptance and network propagation
- **Network Sync**: Full chain synchronization for new nodes
- **Peer Connections**: Dynamic peer management with trust scoring
- **API Response**: Sub-second response times for most operations

### Limitations
- **Not Production Ready**: This is a development release
- **No Live Mainnet**: Mainnet is not yet launched
- **Limited Testing**: Network behavior under high load not fully tested
- **No Frontend**: Web/mobile interfaces not yet implemented

## 🐛 Known Issues

### Minor Issues
- **Post Threshold**: System may need more posts to trigger proper consensus flow
- **Trust Score Integration**: Trust scores exist but need better integration with block proposal logic
- **Peer Discovery**: Working but could be more robust in some network conditions

### Not Yet Implemented
- **Live Network**: The blockchain is not live yet - currently in development/testing phase
- **Genesis Block**: No genesis block has been created for mainnet
- **Network Bootstrapping**: No public mainnet nodes are running yet
- **Frontend Applications**: No web or mobile interfaces yet
- **Block Explorer**: No public block explorer for viewing the blockchain

## 🔮 What's Next

### Phase 5: Production Readiness 🔄 **IN PROGRESS**
- 🔄 Finalize consensus rules
- 🔄 Create genesis block for mainnet
- 🔄 Launch public mainnet nodes
- 🔄 Activate uptime mining and rewards
- 🔄 Complete security audits

### Phase 6: Ecosystem Development ❌ **NOT STARTED**
- ❌ Frontend applications (web/mobile)
- ❌ Block explorer
- ❌ Advanced API features
- ❌ Performance optimization
- ❌ Developer tools and SDKs

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.

## 🔗 Links

- **Repository**: https://github.com/blindxfish/truthchain
- **Documentation**: See `HowToUse.txt` for detailed usage instructions
- **White Paper**: See `WhitePaper.txt` for technical details
- **Network Design**: See `NetworkDesign.txt` for network architecture
- **Consensus Protocol**: See `Consensus.txt` for consensus details

---

## ⚠️ Important Notes

**TRUTCHAIN v0.2.5 IS A DEVELOPMENT RELEASE**

- **Not for Production Use**: This version is for development and testing only
- **No Live Mainnet**: The blockchain is not live yet - currently in development/testing phase
- **Use Testnet or Local**: For testing and development, use testnet or local mode
- **Backup Your Data**: Always backup your wallet and configuration files
- **Report Issues**: Please report any bugs or issues on GitHub

**TruthChain v0.2.5**: The most stable development release yet, with major improvements in network stability, consensus reliability, and overall system robustness. Ready for serious testing and development work.

**TruthChain**: Where truth is permanent, and history cannot be rewritten. 