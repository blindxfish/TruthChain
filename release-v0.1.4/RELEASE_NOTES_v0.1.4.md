# TruthChain v0.1.4 Release Notes

## 🎉 Major Improvements

### Bitcoin-Style Restart System
- **No more crashes on restart** - App now checks for existing data and loads saved configuration
- **Skip setup on restart** - Configuration is saved to `truthchain-config.json` and reused
- **Persistent settings** - Your network choice, ports, modes, and wallet are remembered
- **First-time vs returning users** - Only new users see the interactive setup

### Self-Connection Detection
- **Prevents self-pinging** - Nodes no longer connect to themselves
- **Accurate peer counting** - `/network/peers` endpoint shows real connected peers
- **Domain-aware detection** - Recognizes when connecting to your own domain
- **No duplicate connections** - Eliminates self-connection loops

### Enhanced Peer Tracking
- **Fixed peer list API** - `/network/peers` now returns actual connected peers
- **Mesh-to-topology sync** - Mesh connections properly added to topology
- **Real-time peer info** - Shows peer addresses, trust scores, latency, and connection status

### Network-Aware Blockchain
- **Bitcoin-style genesis handling** - Let network consensus determine valid chain
- **Network-specific validation** - Only strict validation for mainnet
- **Graceful restart** - Works with existing database regardless of network type

## 🔧 Technical Improvements

### Configuration Management
- **Automatic config saving** - Settings saved to `truthchain-config.json`
- **Config file loading** - App loads saved configuration on restart
- **Backward compatibility** - Works with existing databases and wallets

### API Server Compatibility
- **Fixed blockchain signature** - Updated API server for new blockchain constructor
- **Network parameter support** - API server now accepts network ID parameter

### Mesh Network Enhancements
- **Self-connection prevention** - Detects and skips self-connections
- **Improved peer management** - Better connection lifecycle handling
- **Enhanced logging** - Clear messages for connection events

## 📁 Files Created

- `truthchain-config.json` - Saved configuration (created after first setup)
- `truthchain.db` - Blockchain database
- `wallet.json` - Your wallet (private key + address)
- `YourWalletInfo.txt` - Wallet information and security tips

## 🚀 How It Works Now

### First Time (New User)
1. Run `truthchain.exe`
2. Complete interactive setup
3. Configuration saved to `truthchain-config.json`
4. Database and wallet created
5. Node starts with your settings

### Subsequent Starts (Returning User)
1. Run `truthchain.exe`
2. App detects existing data
3. Loads saved configuration
4. Skips setup entirely
5. Node starts with saved settings

### Self-Connection Prevention
1. Bootstrap tries to connect to `mainnet.truth-chain.org:9876`
2. Self-detection recognizes this is your own node
3. Logs "Skipping self-connection to mainnet.truth-chain.org:9876"
4. Only connects to real external peers

## 🐛 Bug Fixes

- **Fixed restart crashes** - No more validation errors on restart
- **Fixed peer counting** - Accurate peer list in API
- **Fixed self-pinging** - No more duplicate connections to self
- **Fixed API compatibility** - API server works with new blockchain signature

## 🔄 Migration

**Existing Users**: 
- Your existing database and wallet will work
- App will detect existing data and load it
- No migration needed

**New Users**: 
- Fresh setup with improved experience
- Configuration automatically saved for future starts

## 📊 Performance

- **Faster startup** - No setup required on restart
- **Reduced network traffic** - No self-connections
- **Better resource usage** - Accurate peer tracking
- **Improved reliability** - Bitcoin-style data handling

## 🔗 Compatibility

- **Full backward compatibility** - Works with existing data
- **Network compatibility** - Supports mainnet, testnet, and local networks
- **API compatibility** - All existing API endpoints work
- **Peer compatibility** - Connects to existing TruthChain nodes

---

**Version**: v0.1.4  
**Release Date**: July 3, 2025  
**Compatibility**: Windows x64  
**Network**: Mainnet, Testnet, Local  
**API**: Full compatibility maintained 