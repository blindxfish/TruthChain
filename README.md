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

## 🏗️ Technical Architecture

### Core Components
- **Wallet System**: ECDSA key generation, signing, storage with Base58Check addresses
- **Block & Post Logic**: Hash, sign, and verify methods with secure signature recovery
- **Transfer System**: Signed character transfers with validation and state management
- **Local Storage**: BoltDB for persistent data with mempool persistence
- **Uptime Tracker**: Character reward distribution
- **HTTP API**: Local interface for frontends
- **State Manager**: Wallet states, balances, and nonce tracking

### Security Model
- All posts and transfers signed with ECDSA private keys
- Public key recovery from compact signatures ensures authorship validation
- Local API only (127.0.0.1) - no exposed network ports by default
- Frontends act as display + signing tools
- Node is the source of truth
- Fork protection with hardcoded mainnet rules

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

## 🚀 Implementation Roadmap

### Milestone 1: Init & Wallet ✅ **COMPLETE**
- ✅ Generate and save ECDSA wallet (secp256k1)
- ✅ CLI: show wallet address (public key)
- ✅ Load or create wallet on node start
- ✅ **Bonus**: Base58Check addresses, multi-network support, metadata

### Milestone 2: Block & Post Logic ✅ **COMPLETE**
- ✅ Define Post and Block structs
- ✅ Implement hash, sign, and verify methods
- ✅ Collect valid posts in memory
- ✅ Commit block when N posts are accumulated (configurable threshold)
- ✅ **Secure signature verification with public key recovery**
- ✅ **Bonus**: Post count thresholds, automatic mempool discharge, fork protection

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

### Milestone 5: Local HTTP API ✅ **COMPLETE**
```bash
# Start the node with API server
go run cmd/main.go --api-port 8080

# The API server will be available at http://127.0.0.1:8080
```

**Available API Endpoints:**

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| `GET` | `/status` | Node and blockchain status | `curl http://127.0.0.1:8080/status` |
| `GET` | `/wallet` | Wallet information and balance | `curl http://127.0.0.1:8080/wallet` |
| `POST` | `/post` | Create and submit a new post | `curl -X POST -H "Content-Type: application/json" -d '{"content":"Hello TruthChain!"}' http://127.0.0.1:8080/post` |
| `GET` | `/posts/latest` | Latest block and pending posts | `curl http://127.0.0.1:8080/posts/latest` |
| `POST` | `/characters/send` | Send characters to another address | `curl -X POST -H "Content-Type: application/json" -d '{"to":"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa","amount":100}' http://127.0.0.1:8080/characters/send` |
| `GET` | `/uptime` | Uptime tracking and rewards info | `curl http://127.0.0.1:8080/uptime` |
| `GET` | `/balance` | Current character balance | `curl http://127.0.0.1:8080/balance` |

**Example API Usage:**
```bash
# Start API server
go run cmd/main.go --api-port 8080

# In another terminal, test the API
curl http://127.0.0.1:8080/status
curl http://127.0.0.1:8080/wallet

# Create a post (requires sufficient character balance)
curl -X POST -H "Content-Type: application/json" \
  -d '{"content":"Hello, TruthChain! This is my first post."}' \
  http://127.0.0.1:8080/post

# Send characters to another address
curl -X POST -H "Content-Type: application/json" \
  -d '{"to":"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa","amount":50}' \
  http://127.0.0.1:8080/characters/send
```

**API Response Examples:**

**GET /status**
```json
{
  "node_info": {
    "wallet_address": "1EAWe46tZvy1KGpcsU3sbJMcL7XmM7yrwT",
    "network": "mainnet",
    "uptime_24h": 95.5,
    "uptime_total": 98.2,
    "character_balance": 1250
  },
  "blockchain_info": {
    "chain_length": 5,
    "total_post_count": 23,
    "total_character_count": 1250,
    "pending_post_count": 2,
    "pending_character_count": 45
  },
  "timestamp": 1751485627
}
```

**POST /post**
```json
{
  "success": true,
  "post": {
    "hash": "abc123...",
    "author": "1EAWe46tZvy1KGpcsU3sbJMcL7XmM7yrwT",
    "content": "Hello, TruthChain!",
    "timestamp": 1751485627,
    "characters": 17
  },
  "new_balance": 1233
}
```

**POST /characters/send**
```json
{
  "success": true,
  "transfer": {
    "from": "1EAWe46tZvy1KGpcsU3sbJMcL7XmM7yrwT",
    "to": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
    "amount": 50,
    "gas_fee": 1,
    "total_cost": 51
  },
  "new_balance": 1199
}
```

### Milestone 6: Character Transfer ✅ **COMPLETE**
- ✅ **Signed transfer payload format** with ECDSA signatures
- ✅ **Public key recovery** from compact signatures for verification
- ✅ **Transfer validation** with balance and nonce checks
- ✅ **Transfer pool** for pending transactions
- ✅ **State management** with wallet states and balances
- ✅ **CLI commands** for transfer operations
- ✅ **API endpoints** for transfer functionality
- ✅ **Gas fee system** (1 character per transfer)
- ✅ **Nonce tracking** for replay protection

### Milestone 7: Post Validator & Chain Sync ⏳ **PENDING**
- ⏳ Validate signature and balance for incoming posts
- ⏳ Store valid ones
- ⏳ Prepare later: sync posts/blocks with peers

**Legend**: ✅ Complete | 🔄 In Progress | ⏳ Pending

## 📁 Project Structure

```
truthchain/
├── cmd/            # main.go entry point with comprehensive CLI
├── api/            # Local HTTP API for frontends
├── chain/          # Block, post, transfer logic and state management
├── wallet/         # Key management, signing, address derivation
├── store/          # BoltDB logic for persistent storage
├── miner/          # Uptime tracker & reward logic
└── utils/          # Hashing, encoding, common tools
```

## 🔧 Core Data Structures

```go
type Post struct {
    Author    string // public key address
    Signature string // signed content hash
    Content   string // text (counted in chars)
    Timestamp int64
}

type Transfer struct {
    From      string // sender address
    To        string // recipient address
    Amount    int    // number of characters
    GasFee    int    // gas fee (always 1)
    Timestamp int64  // unix timestamp
    Nonce     int64  // unique transaction number
    Hash      string // transaction hash
    Signature string // ECDSA signature
}

type Block struct {
    Index       int
    Timestamp   int64
    PrevHash    string
    Hash        string
    Posts       []Post
    Transfers   []Transfer
    StateRoot   *StateRoot
}

type WalletState struct {
    Address    string
    Balance    int
    Nonce      int64
    LastTxTime int64
}
```

## 📈 Scalability

Daily cap ensures chain size grows at a predictable rate:
- **~102 MB/year** uncompressed (~280,000 chars/day)
- **~1 GB/decade** per node (compressed)
- Enables fully decentralized storage without cloud reliance

## 🔮 Future Enhancements

### P2P Node Networking
- Peer discovery via known seed nodes
- Gossip protocol for new blocks/posts
- Sync missing blocks
- Anti-spam & replay protection

### Optional Features
- Compression algorithms
- Pruned nodes (header-only)
- IPFS/Arweave integration for snapshots
- Web interface for viewing, searching, posting
- Browser extension wallet
- Governance, tipping, reputation systems

## 🛡️ Security Features

- **Cryptographic Signing**: All posts and transfers signed with ECDSA private keys
- **Secure Signature Verification**: Public key recovery from compact ECDSA signatures ensures authorship validation
- **Address Derivation**: Consistent Base58Check address generation from public keys
- **Immutable Storage**: Once posted, content cannot be modified
- **Censorship Resistance**: Distributed network prevents single points of failure
- **Verifiable History**: Complete audit trail of all posts and transfers
- **Fork Protection**: Hardcoded mainnet rules prevent malicious forks
- **Genesis Block Validation**: Ensures all nodes start from the same chain
- **Post Count Thresholds**: Configurable block creation rules with validation
- **Mempool Persistence**: Pending posts survive node restarts
- **Transfer Security**: Nonce-based replay protection and balance validation
- **State Management**: Real-time wallet state tracking with pending transaction consideration

## 📚 Documentation

- [System Description](SystemDescription.txt) - Technical implementation details
- [White Paper](WhitePaper.txt) - Comprehensive project overview

## 🖥️ CLI Usage

### Quick Start

```bash
# Build the application
go build -o truthchain.exe cmd/main.go

# Show wallet information
./truthchain.exe --show-wallet --debug

# Add balance for testing
./truthchain.exe --add-balance 1000

# Create a post
./truthchain.exe --post "Hello, TruthChain!"

# Send characters to another address
./truthchain.exe --send 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 100

# Start API server
./truthchain.exe --api-port 8080

# Monitor mode
./truthchain.exe --monitor
```

### Wallet Management ✅ **COMPLETE**

The TruthChain CLI provides comprehensive wallet management capabilities:

#### Basic Wallet Operations
```bash
# Show wallet information
./truthchain.exe --show-wallet

# Show detailed wallet information including metadata
./truthchain.exe --show-wallet --debug

# Create a new mainnet wallet (default)
./truthchain.exe --wallet my-wallet.key

# Create a named wallet
./truthchain.exe --wallet my-wallet.key --name "My TruthChain Wallet"

# Add balance for testing
./truthchain.exe --add-balance 1000
```

#### Multi-Network Support
```bash
# Create mainnet wallet (default)
./truthchain.exe --network mainnet --name "Mainnet Wallet"

# Create testnet wallet for development/testing
./truthchain.exe --network testnet --name "Testnet Wallet"

# Create multisig wallet (placeholder for future implementation)
./truthchain.exe --network multisig --name "Multisig Wallet"
```

#### Command Line Options
| Flag | Description | Default |
|------|-------------|---------|
| `--wallet` | Path to wallet file | `wallet.key` |
| `--show-wallet` | Show wallet address and exit | `false` |
| `--debug` | Show additional wallet information | `false` |
| `--network` | Network type: mainnet, testnet, multisig | `mainnet` |
| `--name` | Wallet name for new wallets | `""` |
| `--add-balance` | Add balance to current wallet (for testing) | `0` |

#### Example Output
```bash
$ ./truthchain.exe --show-wallet --debug
Wallet Address: 1EAWe46tZvy1KGpcsU3sbJMcL7XmM7yrwT
Wallet File: wallet.key
Network: mainnet
Public Key (compressed): 034cda2828b115a2faaf67cbb3a64d434dbe59e8a7382fe289837e9c2428d1ccd9
Public Key (uncompressed): 044cda2828b115a2faaf67cbb3a64d434dbe59e8a7382fe289837e9c2428d1ccd9c1f4a44cba4d05c2aa28b15dfd3039ac875759a64b50bb47213efac95661a37b
Version Byte: 0x00
Address Valid: true
Wallet Name: wallet.key
Created: 2025-07-03 10:48:45
Last Used: 2025-07-03 10:48:45
```

### Blockchain Operations ✅ **COMPLETE**

#### Posts and Blocks
```bash
# Post a message to the blockchain (5 posts trigger block creation by default)
./truthchain.exe --post "Hello, TruthChain!"

# View recent posts and pending mempool
./truthchain.exe --posts

# View blockchain status and statistics
./truthchain.exe --status

# View recent blocks
./truthchain.exe --blocks

# View mempool (pending posts)
./truthchain.exe --mempool

# Force create a block from pending posts
./truthchain.exe --force-block

# Use custom post threshold (for testing)
./truthchain.exe --post-threshold 3 --post "Test post"
```

### Transfer System ✅ **COMPLETE**

#### Character Transfers
```bash
# Send characters to another address
./truthchain.exe --send 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 100

# Show transfer pool information
./truthchain.exe --show-transfers

# Process pending transfers
./truthchain.exe --process-transfers

# Show current state and wallet information
./truthchain.exe --show-state

# Show all wallet states
./truthchain.exe --show-wallets
```

#### Transfer Command Options
| Flag | Description | Example |
|------|-------------|---------|
| `--send` | Recipient address | `--send 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa` |
| `--amount` | Amount of characters to send | `--amount 100` |
| `--show-transfers` | Show transfer pool information | `--show-transfers` |
| `--process-transfers` | Process transfers | `--process-transfers` |
| `--show-state` | Show blockchain state | `--show-state` |
| `--show-wallets` | Show wallet states | `--show-wallets` |

#### Example Transfer Output
```bash
$ ./truthchain.exe --send 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 100
✅ Transfer created successfully!
From: 1EAWe46tZvy1KGpcsU3sbJMcL7XmM7yrwT
To: 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
Amount: 100 characters
Gas Fee: 1 character
Total Cost: 101 characters
Hash: abc123def456...
Nonce: 1
```

### Monitoring and API ✅ **COMPLETE**

#### Live Monitoring
```bash
# Start live monitoring dashboard
./truthchain.exe --monitor
```

#### API Server
```bash
# Start HTTP API server
./truthchain.exe --api-port 8080

# Test API endpoints
curl http://localhost:8080/status
curl http://localhost:8080/wallet
curl http://localhost:8080/balance
```

### Complete Command Reference

| Command | Description | Example |
|---------|-------------|---------|
| `--show-wallet` | Display wallet information | `--show-wallet --debug` |
| `--add-balance` | Add balance for testing | `--add-balance 1000` |
| `--post` | Create a post | `--post "Hello, TruthChain!"` |
| `--posts` | Show recent posts | `--posts` |
| `--status` | Show blockchain status | `--status` |
| `--blocks` | Show recent blocks | `--blocks` |
| `--mempool` | Show pending posts | `--mempool` |
| `--send` | Send characters | `--send <address> --amount <amount>` |
| `--show-transfers` | Show transfer pool | `--show-transfers` |
| `--process-transfers` | Process transfers | `--process-transfers` |
| `--show-state` | Show blockchain state | `--show-state` |
| `--show-wallets` | Show wallet states | `--show-wallets` |
| `--api-port` | Start API server | `--api-port 8080` |
| `--monitor` | Live monitoring | `--monitor` |

## 🧪 Testing

The system includes comprehensive tests for all components:

```bash
# Run all tests
go test ./...

# Run specific test suites
go test ./blockchain/...
go test ./chain/...
go test ./wallet/...
go test ./store/...
go test ./api/...

# Run transfer tests specifically
go test ./blockchain/... -run "Test.*Transfer"
```

## 🤝 Contributing

This project is in active development. Contributions are welcome! Please refer to the implementation roadmap above to understand the current development status.

### Development Guidelines
- Follow Go best practices and conventions
- Add tests for new features
- Update documentation for API changes
- Ensure all tests pass before submitting PRs

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

**MIT License**

Copyright (c) 2024 TruthChain Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

**TruthChain** - Preserving truth, one character at a time. 🚀