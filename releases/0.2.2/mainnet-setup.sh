#!/bin/bash

# TruthChain Mainnet Setup Script
# This script handles the initial configuration after deployment

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

TRUTHCHAIN_HOME="/opt/truthchain"
TRUTHCHAIN_USER="truthchain"
TRUTHCHAIN_BINARY="truthchain"
MAINNET_DOMAIN="168.231.108.135"

echo -e "${BLUE}=== TruthChain Mainnet Setup ===${NC}"
echo -e "${YELLOW}This script will configure TruthChain for mainnet operation${NC}"
echo ""

# Check if running as truthchain user
if [[ $EUID -eq 0 ]]; then
   echo -e "${RED}This script should be run as the truthchain user, not root${NC}"
   echo -e "${YELLOW}Please run: sudo -u truthchain $0${NC}"
   exit 1
fi

# Check if we're the truthchain user
if [[ "$USER" != "$TRUTHCHAIN_USER" ]]; then
   echo -e "${RED}This script must be run as the $TRUTHCHAIN_USER user${NC}"
   exit 1
fi

# Navigate to TruthChain home
cd $TRUTHCHAIN_HOME

# Check if binary exists
if [ ! -f "bin/$TRUTHCHAIN_BINARY" ]; then
    echo -e "${RED}TruthChain binary not found. Please run the deployment script first.${NC}"
    exit 1
fi

echo -e "${BLUE}Starting TruthChain mainnet setup...${NC}"

# Create mainnet configuration file
echo -e "${BLUE}Creating mainnet configuration...${NC}"
cat > truthchain-config.json << EOF
{
  "DBPath": "$TRUTHCHAIN_HOME/data/truthchain.db",
  "APIPort": 8080,
  "MeshPort": 8081,
  "PostThreshold": 5,
  "NetworkID": "truthchain-mainnet",
  "BeaconMode": true,
  "MeshMode": true,
  "MiningMode": true,
  "APIMode": true,
  "Domain": "$MAINNET_DOMAIN",
  "WalletPath": "$TRUTHCHAIN_HOME/data/wallet.json",
  "ImportWallet": false,
  "PrivateKey": "",
  "ConfigureFirewall": false
}
EOF

echo -e "${GREEN}Configuration created: truthchain-config.json${NC}"

# Create data directory if it doesn't exist
mkdir -p data

# Run TruthChain for the first time to create wallet and initialize
echo -e "${BLUE}Initializing TruthChain mainnet node...${NC}"
echo -e "${YELLOW}This will create a new wallet and initialize the blockchain${NC}"
echo ""

# Run TruthChain in the background to initialize
timeout 30s ./bin/$TRUTHCHAIN_BINARY || true

# Check if wallet was created
if [ -f "data/wallet.json" ]; then
    echo -e "${GREEN}Wallet created successfully!${NC}"
    
    # Display wallet info
    echo -e "${BLUE}=== Wallet Information ===${NC}"
    if [ -f "YourWalletInfo.txt" ]; then
        cat YourWalletInfo.txt
    else
        echo "Wallet file created but info file not found"
    fi
else
    echo -e "${YELLOW}Wallet creation may have failed or timed out${NC}"
    echo -e "${YELLOW}You can manually run: ./bin/$TRUTHCHAIN_BINARY${NC}"
fi

# Check if database was created
if [ -f "data/truthchain.db" ]; then
    echo -e "${GREEN}Database initialized successfully!${NC}"
else
    echo -e "${YELLOW}Database initialization may have failed${NC}"
fi

echo ""
echo -e "${GREEN}=== Mainnet Setup Complete ===${NC}"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Review your wallet information above"
echo "2. Start the service: sudo systemctl start truthchain"
echo "3. Check status: ./bin/status.sh"
echo "4. Monitor logs: journalctl -u truthchain -f"
echo ""
echo -e "${YELLOW}Important Security Notes:${NC}"
echo "- Backup your wallet.json file securely"
echo "- Keep your private key safe and offline"
echo "- The node will start earning characters once it's online"
echo ""
echo -e "${YELLOW}Mainnet Configuration:${NC}"
echo "- Network ID: truthchain-mainnet"
echo "- Post Threshold: 5 posts per block"
echo "- API Port: 8080"
echo "- Mesh Port: 8081"
echo "- Beacon Mode: Enabled"
echo "- Mining Mode: Enabled"
echo ""
echo -e "${GREEN}Your TruthChain mainnet node is ready!${NC}" 