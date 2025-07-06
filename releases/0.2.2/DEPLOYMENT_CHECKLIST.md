# TruthChain Mainnet Deployment Checklist

## Pre-Deployment Checklist

### VPS Requirements ✅
- [ ] Debian 12 installed
- [ ] Root access available
- [ ] At least 2GB RAM
- [ ] At least 20GB disk space
- [ ] Static IP: 168.231.108.135
- [ ] SSH access configured
- [ ] Internet connectivity confirmed

### Local Preparation ✅
- [ ] Deployment scripts downloaded
- [ ] SSH key configured (recommended)
- [ ] Domain name ready (optional, for SSL)

## Deployment Steps

### Step 1: Upload Scripts
```bash
# Upload deployment scripts to VPS
scp deploy/mainnet-deploy.sh root@168.231.108.135:/tmp/
scp deploy/mainnet-setup.sh root@168.231.108.135:/tmp/
```

### Step 2: Run Deployment Script
```bash
# SSH into VPS
ssh root@168.231.108.135

# Make executable and run
chmod +x /tmp/mainnet-deploy.sh
/tmp/mainnet-deploy.sh
```

**Checklist for deployment script:**
- [ ] System packages updated
- [ ] Required packages installed (Go, Git, UFW, etc.)
- [ ] TruthChain user created
- [ ] Directories created (/opt/truthchain)
- [ ] TruthChain v0.1.4 cloned and built
- [ ] Firewall configured (ports 8080, 8081)
- [ ] Systemd service created
- [ ] Monitoring scripts created
- [ ] Backup scripts created
- [ ] Cron jobs configured
- [ ] Service enabled

### Step 3: Run Setup Script
```bash
# Run as truthchain user
sudo -u truthchain /tmp/mainnet-setup.sh
```

**Checklist for setup script:**
- [ ] Mainnet configuration created
- [ ] Wallet created successfully
- [ ] Database initialized
- [ ] Wallet information displayed
- [ ] Configuration file saved

### Step 4: Start Service
```bash
# Start TruthChain service
sudo systemctl start truthchain

# Enable auto-start
sudo systemctl enable truthchain

# Check status
sudo systemctl status truthchain
```

## Post-Deployment Verification

### Service Status ✅
- [ ] Service is running: `systemctl is-active truthchain`
- [ ] Service auto-starts: `systemctl is-enabled truthchain`
- [ ] No errors in logs: `journalctl -u truthchain -n 20`

### Network Connectivity ✅
- [ ] API port listening: `netstat -tlnp | grep :8080`
- [ ] Mesh port listening: `netstat -tlnp | grep :8081`
- [ ] Firewall configured: `ufw status`
- [ ] External access: `curl http://168.231.108.135:8080/status`

### Data Files ✅
- [ ] Wallet created: `ls -la /opt/truthchain/data/wallet.json`
- [ ] Database exists: `ls -la /opt/truthchain/data/truthchain.db`
- [ ] Config saved: `ls -la /opt/truthchain/truthchain-config.json`
- [ ] Wallet info file: `ls -la /opt/truthchain/YourWalletInfo.txt`

### API Endpoints ✅
- [ ] Status endpoint: `curl http://168.231.108.135:8080/status`
- [ ] Health endpoint: `curl http://168.231.108.135:8080/health`
- [ ] Network stats: `curl http://168.231.108.135:8080/network/stats`
- [ ] Wallet info: `curl http://168.231.108.135:8080/wallet`

### Monitoring ✅
- [ ] Status script works: `/opt/truthchain/bin/status.sh`
- [ ] Monitor script executable: `ls -la /opt/truthchain/bin/monitor.sh`
- [ ] Backup script executable: `ls -la /opt/truthchain/bin/backup.sh`
- [ ] Cron jobs active: `crontab -u truthchain -l`

## Optional Enhancements

### SSL/HTTPS Setup (Optional)
```bash
# Upload SSL script
scp deploy/ssl-setup.sh root@168.231.108.135:/tmp/

# Run SSL setup
chmod +x /tmp/ssl-setup.sh
/tmp/ssl-setup.sh
```

**SSL Checklist:**
- [ ] Nginx installed and configured
- [ ] SSL certificate obtained (Let's Encrypt or self-signed)
- [ ] HTTPS redirect working
- [ ] Security headers configured
- [ ] Auto-renewal configured (if Let's Encrypt)

### Performance Optimization (Optional)
```bash
# System tuning
echo "truthchain soft nofile 65536" >> /etc/security/limits.conf
echo "truthchain hard nofile 65536" >> /etc/security/limits.conf

# Network optimization
echo "net.core.rmem_max = 16777216" >> /etc/sysctl.conf
echo "net.core.wmem_max = 16777216" >> /etc/sysctl.conf
sysctl -p
```

## Security Checklist

### File Permissions ✅
- [ ] Wallet file: 600 permissions
- [ ] Config file: 644 permissions
- [ ] Binary: 755 permissions
- [ ] Data directory: 700 permissions

### Network Security ✅
- [ ] UFW firewall enabled
- [ ] Only necessary ports open
- [ ] SSH key authentication (recommended)
- [ ] Root login disabled (recommended)

### Wallet Security ✅
- [ ] Wallet backed up securely
- [ ] Private key stored offline
- [ ] Wallet info file reviewed
- [ ] Backup location documented

## Maintenance Tasks

### Daily Monitoring
- [ ] Check service status: `systemctl status truthchain`
- [ ] Review logs: `journalctl -u truthchain --since "1 day ago"`
- [ ] Check resource usage: `htop`, `df -h`
- [ ] Verify API health: `curl http://168.231.108.135:8080/health`

### Weekly Tasks
- [ ] Review backup logs: `tail -50 /opt/truthchain/logs/backup.log`
- [ ] Check disk space: `df -h /opt/truthchain`
- [ ] Review monitoring logs: `tail -50 /opt/truthchain/logs/monitor.log`
- [ ] Update system packages: `apt update && apt upgrade`

### Monthly Tasks
- [ ] Test backup restoration
- [ ] Review SSL certificate expiration
- [ ] Check for TruthChain updates
- [ ] Review firewall rules

## Troubleshooting Quick Reference

### Service Issues
```bash
# Check service status
sudo systemctl status truthchain

# View recent logs
sudo journalctl -u truthchain -n 50

# Restart service
sudo systemctl restart truthchain
```

### Network Issues
```bash
# Check port status
sudo netstat -tlnp | grep :8080
sudo netstat -tlnp | grep :8081

# Check firewall
sudo ufw status

# Test connectivity
curl -v http://168.231.108.135:8080/status
```

### Data Issues
```bash
# Check data files
ls -la /opt/truthchain/data/

# Backup before changes
sudo systemctl stop truthchain
cp /opt/truthchain/data/truthchain.db /opt/truthchain/data/truthchain.db.backup
sudo systemctl start truthchain
```

## Success Criteria

Your TruthChain mainnet node is successfully deployed when:

✅ **Service Status**
- Service is running and enabled
- No errors in systemd logs
- Auto-restart working

✅ **Network Status**
- API responding on port 8080
- Mesh network listening on port 8081
- External access confirmed

✅ **Data Integrity**
- Wallet created and backed up
- Database initialized
- Configuration saved

✅ **Monitoring**
- Status script working
- Health checks passing
- Resource usage normal

✅ **Security**
- Firewall configured
- File permissions correct
- Wallet secured

Once all checkboxes are marked, your TruthChain mainnet node is ready to earn characters and participate in the network! 