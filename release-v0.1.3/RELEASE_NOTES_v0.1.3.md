# TruthChain v0.1.3 - Enhanced UX and Network Improvements

**Release Date:** July 3, 2025  
**Version:** v0.1.3  
**Commit:** b90b349

## 🎉 What's New

This release brings significant improvements to user experience, network stability, and cross-platform compatibility. TruthChain is now more user-friendly and robust than ever!

## 🌱 New Features

### **Green Peer Connection Messages**
- **Visual feedback** when new nodes connect to your network
- **Green colored messages** with 🌱 emoji for easy identification
- **Trust score display** for each connected peer
- **Better network visibility** and monitoring

### **Cross-Platform Firewall Automation**
- **Windows Support**: Automatic Windows Firewall configuration using `netsh advfirewall`
- **Linux Support**: Automatic firewall configuration for:
  - `ufw` (Uncomplicated Firewall - Ubuntu/Debian)
  - `firewalld` (Red Hat/CentOS/Fedora)
  - `iptables` (fallback for other distributions)
- **Interactive Setup**: User can choose to auto-configure firewall during setup
- **Automatic Cleanup**: Firewall rules are removed when the application stops
- **Status API**: Check firewall configuration via `/network/firewall` endpoint

### **Enhanced Network Message Handling**
- **Protocol Detection**: Intelligently handles different message types
- **HTTP Request Filtering**: Clear messages when HTTP requests hit mesh port
- **Ping Message Support**: Proper handling of network ping messages
- **Eliminated Confusing Errors**: No more "invalid character 'P'" messages
- **Better Debugging**: Clear distinction between API and mesh traffic

## 🔧 Improvements

### **Bootstrap Configuration**
- **JSON-based Configuration**: Uses `bootstrap.json` instead of hardcoded defaults
- **Consistent Domain Format**: All domains use `truth-chain.org` format
- **Easy Customization**: Modify bootstrap nodes without recompiling
- **Network Flexibility**: Easy to update for different network environments

### **Interactive Setup Enhancements**
- **Firewall Configuration Option**: Choose to auto-configure firewall
- **Better Port Messaging**: Clear explanation of port usage
- **Improved User Experience**: More intuitive setup process
- **Cross-Platform Awareness**: Different options for Windows vs Linux

### **Code Quality**
- **Cleaner Architecture**: Better separation of concerns
- **Improved Error Handling**: More informative error messages
- **Code Optimization**: Better performance and maintainability
- **Removed Legacy Code**: Cleaned up unused test files and configurations

## 🚀 Getting Started

### **Quick Start**
1. Download `TruthChain.exe` (Windows) or build for your platform
2. Run the executable: `./TruthChain.exe`
3. Follow the interactive setup wizard
4. Choose whether to auto-configure firewall
5. Your node will start automatically!

### **Firewall Configuration**
- **Windows**: Automatically configures Windows Firewall rules
- **Linux**: Detects and configures your firewall (ufw/firewalld/iptables)
- **Manual**: You can still configure firewall manually if preferred

### **Network Ports**
- **API Server**: 8080 (default) - HTTP API for posts and transfers
- **Mesh Network**: 9876 (default) - Peer-to-peer communication and chain sync

## 🔍 API Endpoints

### **New Endpoints**
- `GET /network/firewall` - Check firewall configuration status
- Enhanced `/network/stats` - More detailed network information

### **Existing Endpoints**
- `GET /status` - Node status and blockchain info
- `POST /posts` - Create new posts
- `POST /transfers` - Send characters to other addresses
- `GET /wallets/{address}/balance` - Check wallet balance

## 🐛 Bug Fixes

- **Fixed**: Confusing mesh network error messages
- **Fixed**: Inconsistent domain format in bootstrap configuration
- **Fixed**: Missing firewall cleanup on application shutdown
- **Fixed**: Protocol detection issues in mesh network
- **Fixed**: Build issues with multiple Go files

## 📋 System Requirements

### **Windows**
- Windows 10 or later
- Administrator privileges (for firewall configuration)
- .NET Framework (not required - standalone executable)

### **Linux**
- Any modern Linux distribution
- `ufw`, `firewalld`, or `iptables` (for firewall automation)
- Root/sudo privileges (for firewall configuration)

### **Network**
- Port 8080 open for API access
- Port 9876 open for mesh network communication
- Internet connection for peer discovery

## 🔄 Migration from v0.1.2

### **Automatic Migration**
- Existing wallets and databases are fully compatible
- No data migration required
- Bootstrap configuration will be updated automatically

### **New Features to Try**
1. **Enable firewall automation** during setup
2. **Watch for green connection messages** when peers join
3. **Check firewall status** via API endpoint
4. **Customize bootstrap nodes** by editing `bootstrap.json`

## 🎯 What's Next

- **Enhanced Peer Discovery**: Improved node finding and connection
- **Mobile Support**: TruthChain mobile applications
- **Advanced Mining**: More sophisticated uptime mining algorithms
- **Network Monitoring**: Web-based dashboard for node management
- **Cross-Platform GUI**: Graphical user interface for all platforms

## 📞 Support

- **GitHub Issues**: Report bugs and request features
- **Documentation**: Check `HowToUse.txt` for detailed instructions
- **Community**: Join the TruthChain community discussions

## 🙏 Acknowledgments

Thank you to all contributors and users who provided feedback and helped improve TruthChain. Your input has been invaluable in making this release possible!

---

**TruthChain - Building a truthful, distributed world one character at a time.** 🌐✨ 