#!/bin/bash

# TruthChain Mainnet Deployment Script for Debian 12
# VPS IP: 168.231.108.135

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TRUTHCHAIN_VERSION="v0.2.0"
TRUTHCHAIN_USER="truthchain"
TRUTHCHAIN_HOME="/opt/truthchain"
TRUTHCHAIN_SERVICE="truthchain"
TRUTHCHAIN_BINARY="truthchain"
TRUTHCHAIN_REPO="https://github.com/blindxfish/truthchain.git"

# Network configuration for mainnet
MAINNET_API_PORT=8080
MAINNET_MESH_PORT=9876
MAINNET_DOMAIN="mainnet.truth-chain.org"

echo -e "${BLUE}=== TruthChain Mainnet Deployment ===${NC}"
echo -e "${YELLOW}Target VPS: ${MAINNET_DOMAIN}${NC}"
echo -e "${YELLOW}Version: ${TRUTHCHAIN_VERSION}${NC}"
echo ""

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root${NC}"
   exit 1
fi

# Update system
echo -e "${BLUE}Updating system packages...${NC}"
apt update && apt upgrade -y

# Install required packages
echo -e "${BLUE}Installing required packages...${NC}"
apt install -y \
    git \
    build-essential \
    ufw \
    curl \
    wget \
    supervisor \
    logrotate \
    htop \
    unzip \
    cron \
    jq

# Ensure cron is installed and running
echo "Installing and starting cron service..."
sudo apt update
sudo apt install -y cron
sudo systemctl enable cron
sudo systemctl start cron

# Create truthchain user
echo -e "${BLUE}Creating truthchain user...${NC}"
if ! id "$TRUTHCHAIN_USER" &>/dev/null; then
    useradd -r -s /bin/bash -d $TRUTHCHAIN_HOME -m $TRUTHCHAIN_USER
    echo -e "${GREEN}Created user: $TRUTHCHAIN_USER${NC}"
else
    echo -e "${YELLOW}User $TRUTHCHAIN_USER already exists${NC}"
fi

# Create directories
echo -e "${BLUE}Creating directories...${NC}"
mkdir -p $TRUTHCHAIN_HOME/{bin,data,logs,config}
chown -R $TRUTHCHAIN_USER:$TRUTHCHAIN_USER $TRUTHCHAIN_HOME

# Clone and build TruthChain
echo -e "${BLUE}Cloning TruthChain repository...${NC}"
cd $TRUTHCHAIN_HOME
if [ ! -d "truthchain" ]; then
    git clone $TRUTHCHAIN_REPO
    cd truthchain
    git checkout $TRUTHCHAIN_VERSION
else
    cd truthchain
    git fetch --all
    git checkout $TRUTHCHAIN_VERSION
fi

echo -e "${BLUE}Building TruthChain...${NC}"
go mod download
go build -o $TRUTHCHAIN_HOME/bin/$TRUTHCHAIN_BINARY ./cmd/main.go

# Set permissions
chown $TRUTHCHAIN_USER:$TRUTHCHAIN_USER $TRUTHCHAIN_HOME/bin/$TRUTHCHAIN_BINARY
chmod +x $TRUTHCHAIN_HOME/bin/$TRUTHCHAIN_BINARY

# Create mainnet configuration
echo -e "${BLUE}Creating mainnet configuration...${NC}"
cat > $TRUTHCHAIN_HOME/config/mainnet-config.json << EOF
{
  "DBPath": "$TRUTHCHAIN_HOME/data/truthchain.db",
  "APIPort": $MAINNET_API_PORT,
  "MeshPort": $MAINNET_MESH_PORT,
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

# Create bootstrap.json for peer discovery
echo -e "${BLUE}Creating bootstrap configuration...${NC}"
cat > $TRUTHCHAIN_HOME/bin/bootstrap.json << EOF
{
  "nodes": [
    {
      "address": "$MAINNET_DOMAIN:$MAINNET_MESH_PORT",
      "description": "TruthChain Mainnet Node",
      "region": "Global",
      "is_beacon": true,
      "trust_score": 0.9,
      "last_seen": 0
    }
  ]
}
EOF

chown $TRUTHCHAIN_USER:$TRUTHCHAIN_USER $TRUTHCHAIN_HOME/bin/bootstrap.json

chown $TRUTHCHAIN_USER:$TRUTHCHAIN_USER $TRUTHCHAIN_HOME/config/mainnet-config.json

# Create systemd service
echo -e "${BLUE}Creating systemd service...${NC}"
cat > /etc/systemd/system/$TRUTHCHAIN_SERVICE.service << EOF
[Unit]
Description=TruthChain Mainnet Node
After=network.target
Wants=network.target

[Service]
Type=simple
User=$TRUTHCHAIN_USER
Group=$TRUTHCHAIN_USER
WorkingDirectory=$TRUTHCHAIN_HOME/bin
ExecStart=$TRUTHCHAIN_HOME/bin/$TRUTHCHAIN_BINARY
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$TRUTHCHAIN_SERVICE

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$TRUTHCHAIN_HOME/data

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

# Create logrotate configuration
echo -e "${BLUE}Creating logrotate configuration...${NC}"
cat > /etc/logrotate.d/$TRUTHCHAIN_SERVICE << EOF
/var/log/$TRUTHCHAIN_SERVICE/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 $TRUTHCHAIN_USER $TRUTHCHAIN_USER
    postrotate
        systemctl reload $TRUTHCHAIN_SERVICE > /dev/null 2>&1 || true
    endscript
}
EOF

# Configure firewall
echo -e "${BLUE}Configuring firewall...${NC}"
ufw --force enable
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow $MAINNET_API_PORT/tcp
ufw allow $MAINNET_MESH_PORT/tcp
ufw allow $MAINNET_MESH_PORT/udp

# Create monitoring script
echo -e "${BLUE}Creating monitoring script...${NC}"
cat > $TRUTHCHAIN_HOME/bin/monitor.sh << 'EOF'
#!/bin/bash

TRUTHCHAIN_HOME="/opt/truthchain"
TRUTHCHAIN_SERVICE="truthchain"
API_PORT=8080

# Check if service is running
if ! systemctl is-active --quiet $TRUTHCHAIN_SERVICE; then
    echo "$(date): TruthChain service is not running. Attempting restart..."
    systemctl restart $TRUTHCHAIN_SERVICE
    sleep 10
    
    if systemctl is-active --quiet $TRUTHCHAIN_SERVICE; then
        echo "$(date): Service restarted successfully"
    else
        echo "$(date): Failed to restart service"
    fi
fi

# Check API health
if curl -s http://localhost:$API_PORT/health > /dev/null 2>&1; then
    echo "$(date): API health check passed"
else
    echo "$(date): API health check failed"
fi

# Check disk space
DISK_USAGE=$(df $TRUTHCHAIN_HOME | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 90 ]; then
    echo "$(date): WARNING: Disk usage is ${DISK_USAGE}%"
fi

# Check memory usage
MEM_USAGE=$(free | grep Mem | awk '{printf("%.0f", $3/$2 * 100.0)}')
if [ $MEM_USAGE -gt 90 ]; then
    echo "$(date): WARNING: Memory usage is ${MEM_USAGE}%"
fi
EOF

chmod +x $TRUTHCHAIN_HOME/bin/monitor.sh
chown $TRUTHCHAIN_USER:$TRUTHCHAIN_USER $TRUTHCHAIN_HOME/bin/monitor.sh

# Create backup script
echo -e "${BLUE}Creating backup script...${NC}"
cat > $TRUTHCHAIN_HOME/bin/backup.sh << 'EOF'
#!/bin/bash

TRUTHCHAIN_HOME="/opt/truthchain"
BACKUP_DIR="/opt/truthchain/backups"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

# Stop service
systemctl stop truthchain

# Create backup
tar -czf $BACKUP_DIR/truthchain_backup_$DATE.tar.gz \
    -C $TRUTHCHAIN_HOME data/ \
    -C $TRUTHCHAIN_HOME config/

# Start service
systemctl start truthchain

# Keep only last 7 backups
find $BACKUP_DIR -name "truthchain_backup_*.tar.gz" -mtime +7 -delete

echo "Backup completed: truthchain_backup_$DATE.tar.gz"
EOF

chmod +x $TRUTHCHAIN_HOME/bin/backup.sh
chown $TRUTHCHAIN_USER:$TRUTHCHAIN_USER $TRUTHCHAIN_HOME/bin/backup.sh

# Create cron jobs
echo -e "${BLUE}Setting up cron jobs...${NC}"
(crontab -u $TRUTHCHAIN_USER -l 2>/dev/null; echo "*/5 * * * * $TRUTHCHAIN_HOME/bin/monitor.sh >> $TRUTHCHAIN_HOME/logs/monitor.log 2>&1") | crontab -u $TRUTHCHAIN_USER -
(crontab -u $TRUTHCHAIN_USER -l 2>/dev/null; echo "0 2 * * * $TRUTHCHAIN_HOME/bin/backup.sh >> $TRUTHCHAIN_HOME/logs/backup.log 2>&1") | crontab -u $TRUTHCHAIN_USER -

# Reload systemd and enable service
echo -e "${BLUE}Enabling TruthChain service...${NC}"
systemctl daemon-reload
systemctl enable $TRUTHCHAIN_SERVICE

# Create status script
echo -e "${BLUE}Creating status script...${NC}"
cat > $TRUTHCHAIN_HOME/bin/status.sh << 'EOF'
#!/bin/bash

echo "=== TruthChain Mainnet Status ==="
echo "Service Status: $(systemctl is-active truthchain)"
echo "Service Enabled: $(systemctl is-enabled truthchain)"
echo ""

echo "=== Network Status ==="
echo "API Port (8080): $(netstat -tlnp | grep :8080 || echo 'Not listening')"
echo "Mesh Port (9876): $(netstat -tlnp | grep :9876 || echo 'Not listening')"
echo ""

echo "=== Blockchain Status ==="
if curl -s http://localhost:8080/status > /dev/null 2>&1; then
    echo "API Status:"
    curl -s http://localhost:8080/status | jq '.' 2>/dev/null || curl -s http://localhost:8080/status
else
    echo "API Status: Not responding"
fi
echo ""

echo "=== Resource Usage ==="
echo "Disk Usage:"
df -h /opt/truthchain
echo ""
echo "Memory Usage:"
free -h
echo ""

echo "=== Recent Logs ==="
journalctl -u truthchain --no-pager -n 20
EOF

chmod +x $TRUTHCHAIN_HOME/bin/status.sh
chown $TRUTHCHAIN_USER:$TRUTHCHAIN_USER $TRUTHCHAIN_HOME/bin/status.sh

echo -e "${GREEN}=== Deployment Complete ===${NC}"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Start the service: systemctl start truthchain"
echo "2. Check status: $TRUTHCHAIN_HOME/bin/status.sh"
echo "3. Monitor logs: journalctl -u truthchain -f"
echo "4. Check API: curl http://localhost:8080/status"
echo ""
echo -e "${YELLOW}Important Files:${NC}"
echo "- Binary: $TRUTHCHAIN_HOME/bin/$TRUTHCHAIN_BINARY"
echo "- Config: $TRUTHCHAIN_HOME/config/mainnet-config.json"
echo "- Data: $TRUTHCHAIN_HOME/data/"
echo "- Logs: journalctl -u truthchain"
echo ""
echo -e "${YELLOW}Firewall Status:${NC}"
echo "- SSH: Allowed"
echo "- API Port: $MAINNET_API_PORT (TCP)"
echo "- Mesh Port: $MAINNET_MESH_PORT (TCP/UDP)"
echo ""
echo -e "${GREEN}TruthChain mainnet node is ready for deployment!${NC}" 