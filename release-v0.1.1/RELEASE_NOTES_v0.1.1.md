# TruthChain v0.1.0 - First Mainnet Release

## 🎉 Welcome to TruthChain Mainnet!

TruthChain is a decentralized blockchain network for immutable posts and character-based currency. This is the first mainnet release with a fully functional node implementation.

## ✨ Key Features

### 🔗 **Decentralized Network**
- **Mesh Network**: Peer-to-peer connections with trust scoring
- **Beacon System**: Network discovery and announcements
- **Bootstrap Nodes**: Automatic network joining for new nodes

### ⛏️ **Character Mining**
- **Uptime Mining**: Earn characters by keeping your node online
- **Beacon Rewards**: Bonus rewards for beacon node operators
- **Hybrid Block Creation**: Blocks created every 10 minutes OR when 5 posts are pending

### 💰 **Character Economy**
- **Character Currency**: Text-based currency (1 character = 1 character in posts)
- **Wallet System**: Secure wallet with backup/restore functionality
- **Transfer System**: Send characters between wallets

### 📝 **Content System**
- **Immutable Posts**: Permanent, tamper-proof content storage
- **Character Counting**: Posts cost characters based on content length
- **Digital Signatures**: Cryptographic verification of post authenticity

### 🔧 **Node Features**
- **REST API**: Full HTTP API for interaction
- **Web Interface**: Built-in web dashboard
- **Cross-Platform**: Windows, Linux, macOS support

## 🚀 Getting Started

### Quick Start
```bash
# Clone the repository
git clone https://github.com/blindxfish/TruthChain.git
cd TruthChain

# Build the node
go build -o truthchain cmd/main.go

# Run a full node
./truthchain -beacon -mesh -mining -api -domain your-domain.com
```

### Node Modes
- **Beacon Mode**: Announce your node for network discovery
- **Mesh Mode**: Participate in peer-to-peer network
- **Mining Mode**: Earn characters through uptime
- **API Mode**: Enable web interface and REST API

## 📊 Network Status

- **Current Network**: 1 mainnet node (first node)
- **Block Time**: 10 minutes (or when 5 posts are pending)
- **Character Supply**: Dynamic based on uptime mining
- **Post Threshold**: 5 posts per block

## 🔐 Security Features

- **Cryptographic Signatures**: All transactions are cryptographically signed
- **Wallet Backup**: Secure backup/restore with hash verification
- **Genesis Block**: Immutable genesis block for network integrity
- **Trust Scoring**: Peer trust evaluation system

## 🌐 API Endpoints

### Node Information
- `GET /status` - Node status and blockchain info
- `GET /info` - Detailed node information
- `GET /health` - Health check

### Blockchain
- `GET /blockchain/latest` - Latest block
- `GET /blockchain/length` - Chain length

### Wallet
- `GET /wallets/{address}` - Wallet information
- `GET /wallets/{address}/balance` - Wallet balance
- `GET /wallets/{address}/backup` - Download wallet backup

### Content
- `POST /posts` - Create a new post
- `GET /posts/pending` - Pending posts
- `POST /transfers` - Create a transfer
- `GET /transfers/pending` - Pending transfers

### Network
- `GET /network/stats` - Network statistics
- `GET /network/peers` - Connected peers

## 🛠️ Technical Details

### System Requirements
- **Go**: 1.19 or higher
- **Storage**: 1GB+ for blockchain data
- **Network**: Port 8080 (API), 9876 (Mesh), 9877 (Sync)
- **Memory**: 512MB+ RAM

### Architecture
- **Blockchain**: Custom implementation with persistent storage
- **Network**: TCP-based mesh network with trust scoring
- **Storage**: BoltDB for blockchain and wallet data
- **API**: RESTful HTTP API with CORS support

## 🔄 What's New in v0.1.0

### Major Features
- ✅ **First Mainnet Node**: Fully operational mainnet node
- ✅ **Hybrid Block Creation**: Solves Catch-22 problem for inactive networks
- ✅ **Wallet Backup System**: Secure backup/restore functionality
- ✅ **Mesh Network**: Peer-to-peer connections with trust scoring
- ✅ **Beacon System**: Network discovery and announcements
- ✅ **Uptime Mining**: Character rewards for node operators
- ✅ **REST API**: Complete HTTP API for all operations

### Bug Fixes
- ✅ **Double-start Issue**: Fixed TrustNetwork initialization
- ✅ **Genesis Block**: Consistent genesis block handling
- ✅ **Database Locking**: Improved concurrent access handling

## 🎯 Roadmap

### v0.2.0 (Planned)
- **Web Dashboard**: Enhanced web interface
- **Mobile App**: Mobile wallet application
- **Smart Contracts**: Basic smart contract support
- **Network Explorer**: Block explorer and analytics

### v0.3.0 (Planned)
- **Advanced Mining**: Proof-of-stake consensus
- **Governance**: On-chain governance system
- **Privacy**: Optional privacy features
- **Interoperability**: Cross-chain bridges

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### Development Setup
```bash
git clone https://github.com/blindxfish/TruthChain.git
cd TruthChain
go mod download
go test ./...
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Bitcoin**: Inspiration for the blockchain architecture
- **Go Community**: Excellent tooling and libraries
- **Open Source**: Built on the shoulders of giants

## 📞 Support

- **GitHub Issues**: [Report bugs and feature requests](https://github.com/blindxfish/TruthChain/issues)
- **Discussions**: [Join the community](https://github.com/blindxfish/TruthChain/discussions)
- **Documentation**: [Read the docs](https://github.com/blindxfish/TruthChain/blob/master/README.md)

---

**TruthChain v0.1.0** - Building the future of decentralized truth! 🚀

*Released on July 3, 2025* 