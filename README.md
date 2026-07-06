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

## 🏗️ Current Implementation Status

### ✅ **IMPLEMENTED & WORKING**

#### Core Infrastructure
- **Wallet System**: ECDSA key generation, signing, storage with Base58Check addresses
- **Block & Post Logic**: Hash, sign, and verify methods with secure signature recovery
- **Transfer System**: Signed character transfers with validation and state management
- **Local Storage**: BoltDB for persistent data with mempool persistence
- **HTTP API**: Local interface for frontends with comprehensive endpoints
- **State Manager**: Wallet states, balances, and nonce tracking

#### Network Layer
- **Mesh Network**: Peer-to-peer communication and block synchronization
- **Beacon System**: Network discovery and public node announcements
- **Trust-based Peer Management**: Dynamic peer scoring and connection management
- **Block Synchronization**: Cross-node block sharing and validation

#### Consensus System
- **Post Gossip Protocol**: Network-wide post distribution before block creation
- **Block Proposal & Voting**: Consensus through proposal submission and voting
- **Trust-based Proposer Selection**: Nodes with higher trust scores can propose blocks
- **Dynamic Trust Score Management**: Trust scores based on node behavior
- **Forkless Consensus**: Single-proposer-per-height design (no forks by construction)

> ⚠️ The consensus layer is **implemented but not yet hardened**. A security
> audit found that vote quorum is not yet enforced against network size and
> votes are not authenticated — see [Security Status & Known Limitations](#-security-status--known-limitations)
> below. Do **not** rely on the consensus/mainnet guarantees yet.

#### User Experience
- **Interactive Setup Wizard**: Guided configuration for new users
- **Network Selection**: Mainnet, Testnet, or Local network modes
- **Node Mode Configuration**: API, Mesh, Beacon, and Mining modes
- **Wallet Management**: Create, import, backup, and restore wallets
- **Bitcoin-style Restart**: No crashes, loads existing data automatically

## 🔒 Security Status & Known Limitations

TruthChain is a **working prototype under active hardening**. A full security &
correctness audit (crypto, consensus, networking, storage/API) was performed and
the highest-severity issues were fixed, but several architectural items remain
open. **The network is not production-ready and must not be used on a public
mainnet or exposed to the internet until the outstanding items below are closed.**

### ✅ Recently fixed (audit pass)
- **Build restored** — the repository previously did not compile (a required
  source file had been deleted); it now builds, vets, and tests clean.
- **Block hash verified on ingest** — the block hash is now recomputed and
  checked on every sync/gossip path (previously it was attacker-controlled).
- **Post & transfer signatures verified on ingest** — authorship is now checked
  on the block-sync and gossip paths, not just at local creation.
- **Transfer amount bounds** — an upper bound is enforced so `amount + gas`
  cannot overflow and mint characters.
- **Chain-continuity check** on the gossip block path (prevents chain-tip
  poisoning by injecting a disconnected block).
- **Concurrency crash fixed** — `NetworkTopology` maps are now mutex-protected
  (they previously caused `concurrent map writes` panics under normal load).
- **API hardening** — the HTTP API binds to `127.0.0.1` only, the wallet-backup
  endpoint is loopback-guarded, the private key is no longer written into the
  config file, request bodies are size-limited, and the database open has a
  timeout.

### ⚠️ Outstanding before mainnet (do not deploy yet)
- **Consensus quorum & vote authentication** — approval is measured against
  received votes rather than network size, and votes are not signed; a single
  node can currently force block creation. Needs a real authenticated quorum.
- **API transaction model** — state-mutating endpoints sign with the node's own
  key for any local caller. This should move to **client-side signing** (the
  browser/wallet signs; the node only validates and relays). Also, `POST /posts`
  does not yet persist the created post.
- **Chain reorganization** — the rollback path is currently a no-op; reorgs do
  not correctly revert state.
- **Network robustness** — TCP message framing, per-connection read deadlines,
  connection limits, and receive-path rate limiting need work before public
  exposure.
- **Uptime mining** trusts unsigned local heartbeats; reward issuance is not yet
  consensus-validated.
- **Testing/process** — add a `-race` build and external security review to CI
  before any launch.

See the roadmap's **Phase 5** for how these map to production readiness.

## 🔧 Recent Fixes & Improvements

### Security & Correctness Audit ✅ **FIXED**
- **Issue**: `master` did not compile (deleted `chain/consensus_integration.go`),
  and an audit found critical bugs: unverified block hashes, forgeable post
  authorship, an integer-overflow minting path, a chain-tip poisoning vector, and
  an unsynchronized topology map that crashed nodes.
- **Fix**: Restored the build and closed the above issues; added block-hash and
  signature verification on all ingest paths, transfer amount bounds, a
  chain-continuity check, a topology mutex, and API/storage hardening.
- **Result**: Builds green with new regression tests; see
  [Security Status & Known Limitations](#-security-status--known-limitations)
  for what is fixed vs. still open.

### Time-Based Block Generation ✅ **FIXED**
- **Issue**: Time-based block generation was disabled, preventing proper block creation flow
- **Fix**: Re-enabled `checkTimeBasedBlock()` method with proper conditions:
  - Checks if 10+ minutes have passed since last block
  - Ensures no pending posts exist
  - Prevents duplicate requests or proposals
  - Requests approval from all active peers before block creation

### Consensus Protocol Integration ✅ **FIXED**
- **Issue**: Time-based blocks were bypassing the consensus protocol
- **Fix**: Removed legacy time-based block creation logic that bypassed consensus
- **Result**: All blocks now properly go through the consensus protocol with peer approval

### Peer Counting Logic ✅ **FIXED**
- **Issue**: Nodes incorrectly counted themselves as peers, causing vote count mismatches
- **Fix**: Updated peer counting logic to:
  - Count the node itself as a voter (not as a peer)
  - Use sync manager's peer query provider for accurate peer counts
  - Ensure proper vote approval thresholds

### Mutex Locking Issues ✅ **FIXED**
- **Issue**: "sync: Unlock of unlocked RWMutex" crashes in `RequestTimeBasedBlock()`
- **Fix**: Corrected mutex locking/unlocking sequence to prevent double unlocks
- **Result**: Stable block request handling without crashes

### Chain Tip Validation ✅ **FIXED**
- **Issue**: Fresh nodes with chain tip -1 were rejected during synchronization
- **Fix**: Updated chain tip validation to accept -1 as valid for new nodes
- **Result**: New nodes can properly sync with the network from genesis

### Message Parsing Robustness ✅ **FIXED**
- **Issue**: JSON unmarshaling errors with "invalid character 'P' after top-level value"
- **Fix**: Enhanced mesh manager message processing to:
  - Parse multiple JSON objects from single TCP reads
  - Handle string escapes and partial messages properly
  - Prevent JSON parsing errors from concatenated messages

### Time Synchronization ✅ **FIXED**
- **Issue**: Race conditions between block builder's internal time and blockchain state
- **Fix**: Modified `BuildTimeBasedBlock` to accept actual latest block timestamp
- **Result**: Consistent time checks across the system, preventing premature block requests

### Network Stability ✅ **IMPROVED**
- **Issue**: Crashes in `handleGossipMessage` due to nil pointer dereferences
- **Fix**: Added comprehensive nil checks for `msg.Source` in all message handlers
- **Result**: Robust message handling without crashes

### 🔄 **IN PROGRESS / PARTIALLY IMPLEMENTED**

#### Consensus Protocol
- **Block Creation Flow**: The consensus protocol is implemented and time-based blocks now properly use it
- **Post Threshold Enforcement**: System needs more posts to trigger proper consensus flow
- **Trust Score Integration**: Trust scores exist but need better integration with block proposal logic

#### Sync System
- **Block Synchronization**: Basic sync exists and chain tip validation has been improved
- **Peer Discovery**: Working but could be more robust
- **Network Resilience**: Basic fault tolerance implemented with improved stability

### ❌ **NOT YET IMPLEMENTED**

#### Production Readiness
- **Live Network**: The blockchain is not live yet - currently in development/testing phase
- **Genesis Block**: No genesis block has been created for mainnet
- **Network Bootstrapping**: No public mainnet nodes are running yet
- **Production Consensus**: Consensus rules need finalization and testing

#### Advanced Features
- **Frontend Applications**: No web or mobile interfaces yet
- **Block Explorer**: No public block explorer for viewing the blockchain
- **Advanced API Features**: Some API endpoints are placeholder implementations
- **Performance Optimization**: System needs optimization for high transaction volumes
- **Security Audits**: No formal security audits have been conducted

#### Economic Model
- **Uptime Mining**: Character reward system is implemented but not active on mainnet
- **Transfer Economy**: Transfer system works but no active economy yet
- **Incentive Mechanisms**: Beacon rewards and uptime requirements exist but not active

## 🌐 Network Modes

### Mainnet (Production) - **NOT LIVE YET**
- **Status**: Development complete, awaiting launch
- **Post Threshold**: 5 posts per block
- **Network ID**: `truthchain-mainnet`
- **Consensus Rules**: Fixed for compatibility
- **Use Case**: Production environment (when launched)

### Testnet (Development)
- **Status**: Available for testing
- **Post Threshold**: 3 posts per block
- **Network ID**: `truthchain-testnet`
- **Consensus Rules**: Relaxed for testing
- **Use Case**: Development and testing

### Local (Isolated)
- **Status**: Fully functional
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
- **Status**: Fully implemented and functional

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

## 🔐 Security Features

### Implemented Security
- **ECDSA Signatures**: All posts and transfers cryptographically signed
- **Public Key Recovery**: Signature verification with authorship validation
- **Local API Only**: No exposed network ports by default (127.0.0.1)
- **Wallet Security**: Proper file permissions and backup functionality
- **Nonce Protection**: Replay attack prevention
- **Hash Verification**: Block and post integrity validation

> ℹ️ These are the security mechanisms that exist today. For what is **not** yet
> guaranteed (consensus quorum, API auth model, reorg handling, network
> hardening), see [Security Status & Known Limitations](#-security-status--known-limitations).

### Security Best Practices
- **Backup your wallet**: Save `YourWalletInfo.txt` in multiple secure locations
- **Protect your private key**: Never share it with anyone
- **Use secure environments**: Clean computers with updated software
- **Do not expose the API port**: The HTTP API (8080) binds to `127.0.0.1` and
  has no authentication — keep it local. Only the mesh port (9876) is intended
  to face the network. Put an authenticating reverse proxy in front if you need
  remote API access.

## 🚀 Development Roadmap

### Phase 1: Core Infrastructure ✅ **COMPLETE**
- ✅ Wallet system with ECDSA signing
- ✅ Block and post logic with validation
- ✅ Local storage with BoltDB
- ✅ HTTP API with comprehensive endpoints
- ✅ Transfer system with state management

### Phase 2: Network Layer ✅ **COMPLETE**
- ✅ Mesh network for peer-to-peer communication
- ✅ Beacon system for network discovery
- ✅ Trust-based peer management
- ✅ Block synchronization across nodes

### Phase 3: Consensus System ✅ **COMPLETE**
- ✅ Forkless consensus with no data loss
- ✅ Post gossip protocol
- ✅ Block proposal and voting mechanism
- ✅ Trust-based proposer selection
- ✅ Dynamic trust score management

### Phase 4: User Experience ✅ **COMPLETE**
- ✅ Interactive setup wizard
- ✅ Network and mode configuration
- ✅ Wallet management
- ✅ Bitcoin-style restart system

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

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Links

- **Repository**: https://github.com/blindxfish/truthchain
- **Documentation**: See `HowToUse.txt` for detailed usage instructions
- **White Paper**: See `WhitePaper.txt` for technical details
- **Network Design**: See `NetworkDesign.txt` for network architecture
- **Consensus Protocol**: See `Consensus.txt` for consensus details

---

**⚠️ IMPORTANT**: TruthChain is currently in development. The blockchain is not live yet, and no mainnet exists. This is a working prototype with all core features implemented, but it is **not ready for production use** and has known open security items — see [Security Status & Known Limitations](#-security-status--known-limitations). Use **testnet or local mode** for testing and development, and do not expose a node to the public internet yet.

**TruthChain**: Where truth is permanent, and history cannot be rewritten.