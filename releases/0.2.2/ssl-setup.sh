#!/bin/bash

# TruthChain Mainnet SSL Setup Script
# This script sets up SSL certificates using Let's Encrypt

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

DOMAIN="168.231.108.135"  # Replace with your actual domain if you have one
EMAIL="admin@example.com"  # Replace with your email

echo -e "${BLUE}=== TruthChain Mainnet SSL Setup ===${NC}"
echo -e "${YELLOW}Domain: ${DOMAIN}${NC}"
echo -e "${YELLOW}Email: ${EMAIL}${NC}"
echo ""

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root${NC}"
   exit 1
fi

# Check if nginx is installed
if ! command -v nginx &> /dev/null; then
    echo -e "${BLUE}Installing nginx...${NC}"
    apt update
    apt install -y nginx certbot python3-certbot-nginx
else
    echo -e "${GREEN}Nginx is already installed${NC}"
fi

# Install certbot if not installed
if ! command -v certbot &> /dev/null; then
    echo -e "${BLUE}Installing certbot...${NC}"
    apt install -y certbot python3-certbot-nginx
fi

# Create nginx configuration
echo -e "${BLUE}Creating nginx configuration...${NC}"
cat > /etc/nginx/sites-available/truthchain << EOF
server {
    listen 80;
    server_name ${DOMAIN};
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    location /health {
        proxy_pass http://127.0.0.1:8080/health;
        access_log off;
    }
    
    location /status {
        proxy_pass http://127.0.0.1:8080/status;
    }
    
    location /network/stats {
        proxy_pass http://127.0.0.1:8080/network/stats;
    }
    
    location /wallet {
        proxy_pass http://127.0.0.1:8080/wallet;
        # Add IP restrictions for security
        # allow 127.0.0.1;
        # deny all;
    }
}
EOF

# Enable the site
ln -sf /etc/nginx/sites-available/truthchain /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

# Test nginx configuration
echo -e "${BLUE}Testing nginx configuration...${NC}"
nginx -t

# Start nginx
echo -e "${BLUE}Starting nginx...${NC}"
systemctl enable nginx
systemctl start nginx

# Check if TruthChain is running
if ! systemctl is-active --quiet truthchain; then
    echo -e "${YELLOW}Warning: TruthChain service is not running${NC}"
    echo -e "${YELLOW}Please start it first: systemctl start truthchain${NC}"
    echo -e "${YELLOW}Then run this script again${NC}"
    exit 1
fi

# Check if domain resolves to this server
echo -e "${BLUE}Checking domain resolution...${NC}"
RESOLVED_IP=$(dig +short ${DOMAIN} | head -1)
if [ "$RESOLVED_IP" != "" ] && [ "$RESOLVED_IP" != "168.231.108.135" ]; then
    echo -e "${YELLOW}Warning: Domain ${DOMAIN} resolves to ${RESOLVED_IP}, not this server${NC}"
    echo -e "${YELLOW}SSL certificate may not work properly${NC}"
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Configure firewall for nginx
echo -e "${BLUE}Configuring firewall for nginx...${NC}"
ufw allow 'Nginx Full'

# Obtain SSL certificate
echo -e "${BLUE}Obtaining SSL certificate from Let's Encrypt...${NC}"
echo -e "${YELLOW}Note: If you don't have a domain name, SSL setup will fail${NC}"
echo -e "${YELLOW}You can still use HTTP or set up a self-signed certificate${NC}"
echo ""

# Try to get certificate
if certbot --nginx -d ${DOMAIN} --email ${EMAIL} --agree-tos --non-interactive; then
    echo -e "${GREEN}SSL certificate obtained successfully!${NC}"
    
    # Update nginx configuration with SSL
    echo -e "${BLUE}Updating nginx configuration with SSL...${NC}"
    cat > /etc/nginx/sites-available/truthchain << EOF
server {
    listen 80;
    server_name ${DOMAIN};
    return 301 https://\$server_name\$request_uri;
}

server {
    listen 443 ssl http2;
    server_name ${DOMAIN};
    
    ssl_certificate /etc/letsencrypt/live/${DOMAIN}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${DOMAIN}/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    location /health {
        proxy_pass http://127.0.0.1:8080/health;
        access_log off;
    }
    
    location /status {
        proxy_pass http://127.0.0.1:8080/status;
    }
    
    location /network/stats {
        proxy_pass http://127.0.0.1:8080/network/stats;
    }
    
    location /wallet {
        proxy_pass http://127.0.0.1:8080/wallet;
        # Add IP restrictions for security
        # allow 127.0.0.1;
        # deny all;
    }
    
    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
}
EOF
    
    # Test and reload nginx
    nginx -t
    systemctl reload nginx
    
    # Set up auto-renewal
    echo -e "${BLUE}Setting up SSL certificate auto-renewal...${NC}"
    (crontab -l 2>/dev/null; echo "0 12 * * * /usr/bin/certbot renew --quiet") | crontab -
    
    echo -e "${GREEN}SSL setup complete!${NC}"
    echo -e "${YELLOW}Your TruthChain node is now accessible via HTTPS:${NC}"
    echo -e "${GREEN}https://${DOMAIN}${NC}"
    
else
    echo -e "${YELLOW}SSL certificate setup failed${NC}"
    echo -e "${YELLOW}This is normal if you don't have a domain name${NC}"
    echo -e "${YELLOW}Your node will still work with HTTP${NC}"
    
    # Set up self-signed certificate as fallback
    echo -e "${BLUE}Setting up self-signed certificate as fallback...${NC}"
    
    # Generate self-signed certificate
    mkdir -p /etc/ssl/truthchain
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout /etc/ssl/truthchain/private.key \
        -out /etc/ssl/truthchain/certificate.crt \
        -subj "/C=US/ST=State/L=City/O=TruthChain/CN=${DOMAIN}"
    
    # Update nginx configuration with self-signed certificate
    cat > /etc/nginx/sites-available/truthchain << EOF
server {
    listen 80;
    server_name ${DOMAIN};
    return 301 https://\$server_name\$request_uri;
}

server {
    listen 443 ssl http2;
    server_name ${DOMAIN};
    
    ssl_certificate /etc/ssl/truthchain/certificate.crt;
    ssl_certificate_key /etc/ssl/truthchain/private.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    location /health {
        proxy_pass http://127.0.0.1:8080/health;
        access_log off;
    }
    
    location /status {
        proxy_pass http://127.0.0.1:8080/status;
    }
    
    location /network/stats {
        proxy_pass http://127.0.0.1:8080/network/stats;
    }
    
    location /wallet {
        proxy_pass http://127.0.0.1:8080/wallet;
    }
    
    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
}
EOF
    
    # Test and reload nginx
    nginx -t
    systemctl reload nginx
    
    echo -e "${GREEN}Self-signed SSL setup complete!${NC}"
    echo -e "${YELLOW}Your TruthChain node is now accessible via HTTPS:${NC}"
    echo -e "${GREEN}https://${DOMAIN}${NC}"
    echo -e "${YELLOW}Note: Browsers will show a security warning for self-signed certificates${NC}"
fi

echo ""
echo -e "${GREEN}=== SSL Setup Complete ===${NC}"
echo ""
echo -e "${YELLOW}Access your TruthChain node:${NC}"
echo -e "${GREEN}HTTP:  http://${DOMAIN}${NC}"
echo -e "${GREEN}HTTPS: https://${DOMAIN}${NC}"
echo ""
echo -e "${YELLOW}Nginx Status:${NC}"
echo "systemctl status nginx"
echo ""
echo -e "${YELLOW}SSL Certificate Status:${NC}"
echo "certbot certificates"
echo ""
echo -e "${YELLOW}Test SSL:${NC}"
echo "curl -I https://${DOMAIN}/status" 