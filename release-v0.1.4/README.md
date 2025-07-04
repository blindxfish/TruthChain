# TruthChain v0.1.4 - Bitcoin-Style Node

## 🚀 Quick Start

1. **Download** `truthchain.exe` and `bootstrap.json`
2. **Run** `truthchain.exe` 
3. **Follow setup** (first time only)
4. **Enjoy** - App remembers your settings!

## ✨ What's New in v0.1.4

### 🎯 Bitcoin-Style Restart
- **No more crashes** - App loads existing data automatically
- **Skip setup** - Configuration saved and reused
- **Persistent settings** - Your choices are remembered

### 🔗 Self-Connection Detection  
- **No self-pinging** - Prevents nodes from connecting to themselves
- **Accurate peers** - Real peer count in `/network/peers` API
- **Clean connections** - Only connects to actual external peers

### 📊 Enhanced Peer Tracking
- **Fixed peer list** - Shows real connected peers
- **Real-time info** - Peer addresses, trust scores, latency
- **Better monitoring** - Accurate network statistics

## 📁 Files Included

- `truthchain.exe` - TruthChain node executable
- `bootstrap.json` - Network bootstrap configuration
- `RELEASE_NOTES_v0.1.4.md` - Detailed release notes

## 🔧 Installation

### Windows
1. Download all files to a folder
2. Run `truthchain.exe`
3. Follow the interactive setup
4. Your configuration is automatically saved

### First Run
```
🌐 TruthChain Node Setup
=========================

💰 Wallet Configuration
=======================
Your wallet is your identity on TruthChain...

🌍 Select Network:
1. Mainnet (Production - Real TruthChain network)
2. Testnet (Development - Testing environment)  
3. Local (Isolated - Your own private network)
```

### Subsequent Runs
```
Found existing TruthChain data:
  Database: truthchain.db
  Wallet: wallet.json
  Network: truthchain-mainnet
  API Port: 8080
  Mesh Port: 9876
Found existing TruthChain data, starting with saved configuration
```

## 🌐 Network Modes

### Mainnet (Production)
- **Post threshold**: 5 posts per block
- **Real network**: Connect to actual TruthChain nodes
- **Real rewards**: Earn actual characters

### Testnet (Development)
- **Post threshold**: 3 posts per block  
- **Test environment**: Safe for testing
- **Test rewards**: Earn test characters

### Local (Private)
- **Post threshold**: 2 posts per block
- **Isolated network**: Your own private blockchain
- **Development**: Perfect for development and testing

## 🔌 API Endpoints

Once running, access these endpoints:

- `http://localhost:8080/status` - Node status
- `http://localhost:8080/network/peers` - Connected peers
- `http://localhost:8080/blockchain/latest` - Latest block
- `http://localhost:8080/wallets/{address}/balance` - Wallet balance

## 🛠️ Features

### ✅ What Works
- **Bitcoin-style restart** - No crashes, loads existing data
- **Self-connection detection** - No duplicate peer counting
- **Peer tracking** - Accurate peer list and statistics
- **Network sync** - Connect to TruthChain network
- **Uptime mining** - Earn characters by keeping node online
- **Post creation** - Create immutable posts
- **Character transfers** - Send characters to other wallets
- **API server** - Full REST API for interactions
- **Mesh network** - Peer-to-peer communication
- **Beacon mode** - Announce your node to the network

### 🔄 Migration from v0.1.3
- **Automatic** - Your existing data works unchanged
- **No setup required** - App detects and loads existing configuration
- **Backward compatible** - All features from v0.1.3 still work

## 📊 Performance

- **Startup time**: ~2-3 seconds (vs 30+ seconds with setup)
- **Memory usage**: ~50MB typical
- **Network traffic**: Reduced (no self-connections)
- **Reliability**: Bitcoin-style data handling

## 🔒 Security

- **Wallet encryption** - Private keys stored securely
- **Network validation** - Peer verification and trust scoring
- **Signature verification** - All posts and transfers signed
- **Firewall integration** - Automatic Windows firewall configuration

## 🆘 Troubleshooting

### App won't start
- Check if `truthchain.exe` is in the same folder as `bootstrap.json`
- Ensure you have write permissions in the folder
- Check Windows Defender isn't blocking the executable

### No peers connected
- Check your internet connection
- Verify firewall allows TruthChain (ports 8080 and 9876)
- Check `bootstrap.json` contains valid peer addresses

### API not responding
- Verify the node is running (check console output)
- Check if port 8080 is available
- Try `http://localhost:8080/health` for basic connectivity

## 📞 Support

- **GitHub Issues**: Report bugs and feature requests
- **Documentation**: Check the main repository README
- **Community**: Join TruthChain discussions

## 📄 License

TruthChain is open source software. See LICENSE file for details.

---

**Version**: v0.1.4  
**Release Date**: July 3, 2025  
**Platform**: Windows x64  
**Network**: Mainnet, Testnet, Local 