# TruthChain v0.2.3 Release Notes

## 🚀 New Features & Improvements

### 🔧 Dual Logging System
- **Console + File Logging**: Logs now appear both in the console AND in `logs/truthchain.log`
- **Better Debugging**: You can see real-time output while also having persistent logs
- **No More Silent Mode**: The node will always show activity on the command line

### 🌐 Sync Port Separation
- **Dedicated Sync Port**: Chain sync now uses port 9877 (separate from mesh port 9876)
- **No More Port Conflicts**: Fixed the "address already in use" error on port 9876
- **Cleaner Architecture**: Mesh communication and chain sync are now properly separated

## 🐛 Bug Fixes

### Port Conflict Resolution
- **Issue**: Sync server was trying to use the same port (9876) as the mesh network
- **Fix**: Sync server now uses dedicated port 9877 by default
- **Result**: No more "bind: address already in use" errors

### Logging Visibility
- **Issue**: Users couldn't see node activity on the command line
- **Fix**: Implemented dual logging (console + file)
- **Result**: Real-time visibility of node operations

## 📋 Configuration Changes

### New Sync Port Configuration
- **Default Sync Port**: 9877 (configurable during setup)
- **Mesh Port**: Still 9876 (unchanged)
- **API Port**: Still 8080 (unchanged)

### Setup Process
- When enabling mesh mode, you'll now be asked for both:
  - Mesh Network Port (default: 9876)
  - Chain Sync Port (default: 9877)

## 🔄 Migration from v0.2.2

### For Existing Nodes
1. **Stop the current node**
2. **Deploy the new binary** (truthchain-linux)
3. **Restart the node** - it will use the new configuration automatically

### For New Nodes
- Run the setup process normally
- The new dual logging and sync port separation will be configured automatically

## 🚨 Important Notes

### Firewall Configuration
- **New Port**: Make sure port 9877 is open for chain sync
- **Existing Ports**: Ports 8080 (API) and 9876 (mesh) remain the same

### Log Files
- **Location**: `logs/truthchain.log`
- **Rotation**: Logs are appended to the existing file
- **Console**: All logs also appear in real-time on the command line

## 🎯 What's Next

This release focuses on stability and usability improvements. The next release will likely include:
- Enhanced sync performance
- Better error handling
- Additional network features

---

**Build Date**: July 6, 2025  
**Version**: v0.2.3  
**Compatibility**: Compatible with v0.2.2 nodes (backward compatible) 