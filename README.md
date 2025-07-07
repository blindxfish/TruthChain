# TruthChain

A decentralized blockchain protocol designed to permanently record and preserve historical statements, posts, and texts. TruthChain provides a censorship-resistant, tamper-proof mechanism for publishing and archiving information, replacing traditional financial tokens with a finite, cryptographically-earned unit of information: the character.

## 🎯 Vision

In a world where political figures, corporations, and media entities frequently erase or alter their past claims, TruthChain creates a globally distributed system where statements, news, or posts can be published and preserved forever, immune to modification or deletion. This supports a truthful public record and counteracts historical revisionism.

## 🔑 Key Concepts

### Characters as Currency
- **One "character"** = one UTF-8 text character stored on-chain
- **Earned** by keeping the network alive (running a node)
- **Burned** to post messages
- **Transferable** between users with secure ECDSA signatures

### Daily Character Cap
- **280,000 characters per day** (≈1,000 Twitter-length posts)
- Shared among all online nodes with logarithmic decay
- Early adopters earn more, encouraging network bootstrapping

### Immutable Posts
- All posts are cryptographically signed with ECDSA
- Stored permanently on-chain
- Cannot be modified or deleted
- Verifiable authorship and timestamp

### Secure Transfers
- Character transfers signed with ECDSA private keys
- Public key recovery for signature verification
- Nonce-based replay protection
- Gas fees for network sustainability (1 character)

## 🚀 Quick Start

### Decentralized Consensus Setup (v0.3.0 - Forkless Consensus)
TruthChain v0.3.0 features **decentralized, forkless consensus** ensuring no posts or burned characters are ever lost:

```bash
# Download and run TruthChain
./TruthChain.exe

# First time: Interactive setup guides you through:
# 1. Create or import a wallet
# 2. Select network (Mainnet/Testnet/Local)
# 3. Choose node modes (API/Mesh/Beacon/Mining)
# 4. Configure ports and settings
# 5. Join the decentralized consensus network
# 6. Configuration automatically saved for future starts

# Subsequent runs: App joins consensus and loads existing data
# No setup required - just works with decentralized consensus!
```

### 🛡️ New Consensus Features (v0.3.0)
- **Forkless Consensus**: No posts or burned characters are ever lost
- **Decentralized Block Creation**: Single proposer per block with voting
- **Trust-Based Proposer Selection**: Nodes with higher trust scores can propose
- **Post Gossip Protocol**: Posts spread across the network before block creation
- **Block Proposal & Voting**: Consensus through proposal submission and voting
- **Trust Score Management**: Dynamic trust scores based on node behavior

### What You'll Get
- ✅ **Bitcoin-Style Restart**: No crashes, loads existing data automatically
- ✅ **Persistent Configuration**: Settings saved to `truthchain-config.json`
- ✅ **Self-Connection Detection**: No duplicate peer counting or self-pinging
- ✅ **Wallet Creation/Import**: Create new wallet or import existing one
- ✅ **Network Selection**: Choose between Mainnet, Testnet, or Local
- ✅ **Node Modes**: Enable API, Mesh, Beacon, and Mining features
- ✅ **Port Configuration**: Simple port setup for network communication
- ✅ **Wallet Info File**: Comprehensive wallet information with security warnings
- ✅ **Mainnet Compatibility**: Fixed consensus rules for network compatibility

### Wallet Management
- **New Wallet**: Automatically created and saved to `wallet.json`
- **Import Wallet**: Import existing wallet using private key
- **Wallet Info**: `YourWalletInfo.txt` file with security instructions
- **Backup/Restore**: Complete wallet backup and restore functionality

## 🏗️ Technical Architecture

### Core Components
- **Wallet System**: ECDSA key generation, signing, storage with Base58Check addresses
- **Block & Post Logic**: Hash, sign, and verify methods with secure signature recovery
- **Transfer System**: Signed character transfers with validation and state management
- **Local Storage**: BoltDB for persistent data with mempool persistence
- **Uptime Tracker**: Character reward distribution with 80% uptime requirement
- **HTTP API**: Local interface for frontends
- **State Manager**: Wallet states, balances, and nonce tracking
- **Mesh Network**: Peer-to-peer communication and block synchronization
- **Beacon System**: Network discovery and public node announcements

### Security Model
- All posts and transfers signed with ECDSA private keys
- Public key recovery from compact signatures ensures authorship validation
- Local API only (127.0.0.1) - no exposed network ports by default
- Frontends act as display + signing tools
- Node is the source of truth
- Fork protection with hardcoded mainnet rules
- Wallet files with proper permissions (600)

## 📊 Economic Model

### Node Rewards
Nodes earn characters based on uptime, not proof-of-work. Character issuance decreases logarithmically as node count grows:

| Nodes Online | Characters per Node/day | Total Characters Emitted |
|--------------|-------------------------|--------------------------|
| 1            | 1,120                   | 1,120                    |
| 10           | 1,037                   | 10,370                   |
| 100          | 800                     | 80,000                   |
| 500          | 451                     | 225,500                  |
| 1,000        | 280                     | 280,000 (hard cap)       |
| 10,000       | ~27                     | 280,000                  |
| 100,000      | ~2.7                    | 280,000                  |

### Uptime Requirements
- **80% uptime over 24 hours** required to receive rewards
- Rewards distributed every 10 minutes when requirements are met
- Heartbeats logged every hour for uptime tracking

### Transfer Economy
- **Gas Fee**: 1 character per transfer (fixed)
- **Transfer Cost**: Amount + 1 character gas fee
- **Nonce System**: Prevents replay attacks and ensures transaction ordering
- **State Management**: Real-time balance tracking with pending transaction consideration

### Incentive Structure
- Characters become scarcer and more valuable over time
- Users must run a node or obtain characters from others to post
- Early adoption is rewarded with higher daily earnings
- Transfer fees provide network sustainability
- **Beacon nodes receive +50% character reward** for acting as public entry points and increasing network stability

## 🚀 Implementation Roadmap

### Milestone 1: Init & Wallet ✅ **COMPLETE**
- ✅ Generate and save ECDSA wallet (secp256k1)
- ✅ CLI: show wallet address (public key)
- ✅ Load or create wallet on node start
- ✅ **Bonus**: Base58Check addresses, multi-network support, metadata
- ✅ **NEW**: Interactive wallet creation and import functionality
- ✅ **NEW**: Comprehensive wallet info file generation

### Milestone 2: Block & Post Logic ✅ **COMPLETE**
- ✅ Define Post and Block structs
- ✅ Implement hash, sign, and verify methods
- ✅ Collect valid posts in memory
- ✅ Commit block when N posts are accumulated (configurable threshold)
- ✅ **Secure signature verification with public key recovery**
- ✅ **Bonus**: Post count thresholds, automatic mempool discharge, fork protection
- ✅ **NEW**: Fixed consensus rules for mainnet compatibility

### Milestone 3: Local Storage (BoltDB) ✅ **COMPLETE**
- ✅ Save/load blocks with persistent storage
- ✅ Save posts by hash with duplicate detection
- ✅ Track current block index and chain length
- ✅ Track pending posts in mempool with persistence
- ✅ **Bonus**: Mempool discharge, post count thresholds, fork protection

### Milestone 4: Uptime Tracker ✅ **COMPLETE**
- ✅ Node logs uptime (heartbeats)
- ✅ Every 10 minutes share the calculated amount of the characters among all active nodes based on the reward table.
- ✅ Reward characters to the wallet
- ✅ Live monitoring dashboard with `--monitor` flag
- ✅ **Beacon Incentive**: Nodes running in beacon mode receive **+50% character reward** per interval as an incentive for public discoverability and network stability.
- ✅ **NEW**: 80% uptime requirement with clear messaging

### Milestone 5: Local HTTP API ✅ **COMPLETE**
- ✅ RESTful API endpoints for all node operations
- ✅ JSON responses with proper error handling
- ✅ **Wallet backup/restore via API** for frontend integration
- ✅ **Secure wallet backup download** with proper headers

### Milestone 6: Network Layer ✅ **COMPLETE**
- ✅ Mesh network for peer-to-peer communication
- ✅ Beacon system for network discovery
- ✅ Trust-based peer management
- ✅ Block synchronization across nodes

### Milestone 8: Decentralized Consensus ✅ **COMPLETE**
- ✅ Forkless consensus system with no data loss
- ✅ Post gossip protocol for network-wide post distribution
- ✅ Block proposal and voting mechanism
- ✅ Trust-based proposer selection
- ✅ Dynamic trust score management
- ✅ Single proposer per block with voting consensus
- ✅ Integration with existing mesh network

### Milestone 7: User Experience ✅ **COMPLETE**
- ✅ Bitcoin-style restart system
- ✅ Self-connection detection
- ✅ Interactive setup wizard
- ✅ Network selection (Mainnet/Testnet/Local)
- ✅ Node mode configuration
- ✅ Port configuration
- ✅ Wallet import/creation
- ✅ Comprehensive documentation

## 🌐 Network Modes

### Mainnet (Production)
- **Post Threshold**: 5 posts per block
- **Network ID**: `truthchain-mainnet`
- **Consensus Rules**: Fixed for compatibility
- **Use Case**: Production environment

### Testnet (Development)
- **Post Threshold**: 3 posts per block
- **Network ID**: `truthchain-testnet`
- **Consensus Rules**: Relaxed for testing
- **Use Case**: Development and testing

### Local (Isolated)
- **Post Threshold**: 2 posts per block
- **Network ID**: `truthchain-local`
- **Consensus Rules**: Minimal for local testing
- **Use Case**: Local development and testing

## 🔧 Node Modes

### API Mode
- **Purpose**: HTTP API server for frontend integration
- **Port**: 8080 (default)
- **Features**: Post creation, balance checking, transfers
- **Required**: For web interfaces and external tools

### Mesh Mode
- **Purpose**: Peer-to-peer network communication and consensus
- **Port**: 9876 (default)
- **Features**: Block sync, post propagation, peer discovery, consensus voting
- **Required**: For network participation and consensus

### Beacon Mode
- **Purpose**: Network discovery and public announcements
- **Requirements**: Public IP and domain
- **Features**: +50% character reward bonus
- **Use Case**: Public entry points for the network

### Mining Mode
- **Purpose**: Uptime-based character mining
- **Requirements**: 80% uptime over 24 hours
- **Features**: Automatic character rewards every 10 minutes
- **Use Case**: Earning characters for posting

## 📡 API Reference

**Start the node with API server:**
```bash
# Interactive setup (recommended)
./TruthChain.exe

# Or command line
./TruthChain.exe --api-port 8080
```

**Available API Endpoints:**

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| `GET` | `/status` | Node and blockchain status | `curl http://127.0.0.1:8080/status` |
| `GET` | `/health` | Health check | `curl http://127.0.0.1:8080/health` |
| `GET` | `/info` | Node information | `curl http://127.0.0.1:8080/info` |
| `GET` | `/wallets/{address}` | Wallet information | `curl http://127.0.0.1:8080/wallets/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa` |
| `GET` | `/wallets/{address}/balance` | Wallet balance | `curl http://127.0.0.1:8080/wallets/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa/balance` |
| `GET` | `/wallets/{address}/backup` | Download wallet backup | `curl http://127.0.0.1:8080/wallets/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa/backup` |
| `POST` | `/posts` | Create a new post | `curl -X POST -H "Content-Type: application/json" -d '{"content":"Hello TruthChain!"}' http://127.0.0.1:8080/posts` |
| `GET` | `/posts/pending` | Get pending posts | `curl http://127.0.0.1:8080/posts/pending` |
| `POST` | `/transfers` | Send characters | `curl -X POST -H "Content-Type: application/json" -d '{"to":"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa","amount":100}' http://127.0.0.1:8080/transfers` |
| `GET` | `/transfers/pending` | Get pending transfers | `curl http://127.0.0.1:8080/transfers/pending` |
| `GET` | `/blockchain/latest` | Latest block | `curl http://127.0.0.1:8080/blockchain/latest` |
| `GET` | `/blockchain/length` | Chain length | `curl http://127.0.0.1:8080/blockchain/length` |
| `GET` | `/network/stats` | Network statistics | `curl http://127.0.0.1:8080/network/stats` |

## 🔐 Security Best Practices

### Wallet Security
- **Backup your wallet**: Save `YourWalletInfo.txt` in multiple secure locations
- **Protect your private key**: Never share it with anyone
- **Use secure environments**: Clean computers with updated software
- **Regular backups**: Test your backup by importing on a test system

### Network Security
- **Firewall configuration**: Only open necessary ports (8080 for API, 9876 for mesh)
- **Domain security**: Use secure domains for beacon mode
- **Regular updates**: Keep your node software updated

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Links

- **Repository**: https://github.com/blindxfish/truthchain
- **Documentation**: See `HowToUse.txt` for detailed usage instructions
- **White Paper**: See `WhitePaper.txt` for technical details
- **Network Design**: See `NetworkDesign.txt` for network architecture

---

**TruthChain**: Where truth is permanent, and history cannot be rewritten.