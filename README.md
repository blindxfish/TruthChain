# TruthChain

A decentralized blockchain protocol designed to permanently record and preserve historical statements, posts, and texts. TruthChain provides a censorship-resistant, tamper-proof mechanism for publishing and archiving information, replacing traditional financial tokens with a finite, cryptographically-earned unit of information: the character.

## 🎯 Vision

In a world where political figures, corporations, and media entities frequently erase or alter their past claims, TruthChain creates a globally distributed system where statements, news, or posts can be published and preserved forever, immune to modification or deletion. This supports a truthful public record and counteracts historical revisionism.

## 🔑 Key Concepts

### Characters as Currency
- **One "character"** = one UTF-8 text character stored on-chain
- **Earned** by keeping the network alive (running a node)
- **Burned** to post messages
- **Transferable** between users

### Daily Character Cap
- **280,000 characters per day** (≈1,000 Twitter-length posts)
- Shared among all online nodes with logarithmic decay
- Early adopters earn more, encouraging network bootstrapping

### Immutable Posts
- All posts are cryptographically signed
- Stored permanently on-chain
- Cannot be modified or deleted
- Verifiable authorship and timestamp

## 🏗️ Technical Architecture

### Core Components
- **Wallet System**: ECDSA key generation, signing, storage
- **Block & Post Logic**: Hash, sign, and verify methods
- **Local Storage**: BoltDB for persistent data
- **Uptime Tracker**: Character reward distribution
- **HTTP API**: Local interface for frontends
- **Character Transfer**: User-to-user transactions

### Security Model
- All posts and transfers signed with private keys
- Local API only (127.0.0.1) - no exposed network ports by default
- Frontends act as display + signing tools
- Node is the source of truth

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

### Incentive Structure
- Characters become scarcer and more valuable over time
- Users must run a node or obtain characters from others to post
- Early adoption is rewarded with higher daily earnings

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
- ✅ Commit block when N characters are accumulated
- ✅ **Secure signature verification with public key recovery**

### Milestone 3: Local Storage (BoltDB) ⏳ **PENDING**
- ⏳ Save/load blocks
- ⏳ Save posts by hash
- ⏳ Track current block index
- ⏳ Track total characters owned per user

### Milestone 4: Uptime Tracker ⏳ **PENDING**
- ⏳ Node logs uptime (heartbeats)
- ⏳ Every 24h: divide 280,000 characters among all active nodes
- ⏳ Reward characters to the wallet

### Milestone 5: Local HTTP API ⏳ **PENDING**
- ⏳ Expose endpoints:
  - `GET /status` – node info
  - `GET /wallet` – address, char balance
  - `POST /post` – submit signed post
  - `GET /posts/latest` – recent posts
  - `POST /characters/send` – send characters

### Milestone 6: Character Transfer ⏳ **PENDING**
- ⏳ Add signed transfer payload format
- ⏳ Update balances on both sides

### Milestone 7: Post Validator & Chain Sync ⏳ **PENDING**
- ⏳ Validate signature and balance for incoming posts
- ⏳ Store valid ones
- ⏳ Prepare later: sync posts/blocks with peers

**Legend**: ✅ Complete | 🔄 In Progress | ⏳ Pending

## 📁 Project Structure

```
truthchain/
├── cmd/            # main.go entry point
├── api/            # Local HTTP API for frontends
├── chain/          # Block, post, hash logic
├── wallet/         # Key management, signing
├── store/          # BoltDB logic
├── miner/          # Uptime tracker & reward logic
└── utils/          # Hashing, encoding, common tools
```

## 🔧 Core Data Structures

```go
type Post struct {
    Author    string // public key
    Signature string // signed content hash
    Content   string // text (counted in chars)
    Timestamp int64
}

type Block struct {
    Index     int
    Timestamp int64
    PrevHash  string
    Hash      string
    Posts     []Post
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

- **Cryptographic Signing**: All posts and transfers signed with private keys
- **Secure Signature Verification**: Public key recovery from compact ECDSA signatures ensures authorship validation
- **Immutable Storage**: Once posted, content cannot be modified
- **Censorship Resistance**: Distributed network prevents single points of failure
- **Verifiable History**: Complete audit trail of all posts and transfers

## 📚 Documentation

- [System Description](SystemDescription.txt) - Technical implementation details
- [White Paper](WhitePaper.txt) - Comprehensive project overview

## 🖥️ CLI Usage

### Wallet Management (Milestone 1 ✅)

The TruthChain CLI provides comprehensive wallet management capabilities:

#### Basic Wallet Operations
```bash
# Show wallet information
go run cmd/main.go --show-wallet

# Show detailed wallet information including metadata
go run cmd/main.go --show-wallet --debug

# Create a new mainnet wallet (default)
go run cmd/main.go --wallet my-wallet.key

# Create a named wallet
go run cmd/main.go --wallet my-wallet.key --name "My TruthChain Wallet"
```

#### Multi-Network Support
```bash
# Create mainnet wallet (default)
go run cmd/main.go --network mainnet --name "Mainnet Wallet"

# Create testnet wallet for development/testing
go run cmd/main.go --network testnet --name "Testnet Wallet"

# Create multisig wallet (placeholder for future implementation)
go run cmd/main.go --network multisig --name "Multisig Wallet"
```

#### Command Line Options
| Flag | Description | Default |
|------|-------------|---------|
| `--wallet` | Path to wallet file | `wallet.key` |
| `--show-wallet` | Show wallet address and exit | `false` |
| `--debug` | Show additional wallet information | `false` |
| `--network` | Network type: mainnet, testnet, multisig | `mainnet` |
| `--name` | Wallet name for new wallets | `""` |

#### Example Output
```bash
$ go run cmd/main.go --show-wallet --debug --network testnet --name "Test Wallet"
Wallet Address: 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
Wallet File: wallet.key
Network: testnet
Public Key (compressed): 02a1b2c3d4e5f6...
Public Key (uncompressed): 04a1b2c3d4e5f6...
Version Byte: 0x6F
Address Valid: true
Wallet Name: Test Wallet
Created: 2024-01-15 10:30:45
Last Used: 2024-01-15 10:30:45
```

### CLI Features

#### Milestone 2: Block & Post Logic ✅ **AVAILABLE**
```bash
# Post a message to the blockchain
go run cmd/main.go --post "Hello, TruthChain!"

# View recent posts
go run cmd/main.go --posts

# View blockchain status
go run cmd/main.go --status
```

#### Milestone 5: Local HTTP API
```bash
# Start the node with API server
go run cmd/main.go --api-port 8080

# View API documentation
go run cmd/main.go --api-docs
```

#### Milestone 6: Character Transfer
```bash
# Send characters to another address
go run cmd/main.go --send 1000 --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa

# Check character balance
go run cmd/main.go --balance
```

## 🤝 Contributing

This project is in active development. Contributions are welcome! Please refer to the implementation roadmap above to understand the current development status.

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

**TruthChain** - Preserving truth, one character at a time.