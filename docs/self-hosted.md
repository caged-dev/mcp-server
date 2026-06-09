# Self-Hosted Deployment Guide

Run Caged on your own server with Firecracker microVMs.

## Requirements

- **Linux server** with KVM support (bare metal recommended)
- **CPU**: 4+ cores (each sandbox uses 1-4 vCPUs)
- **RAM**: 16GB minimum (each sandbox uses 512MB-4GB)
- **Disk**: 100GB+ SSD (rootfs images + sandbox workspaces)
- **OS**: Ubuntu 22.04+ or Debian 12+

## Quick Start (Docker Compose)

```bash
git clone https://github.com/caged-dev/caged.git
cd caged
cp .env.example .env
# Edit .env with your settings
docker compose up -d
```

## Manual Setup

### 1. Install Firecracker

```bash
# Download latest release
ARCH=$(uname -m)
VERSION="1.11.0"
curl -fsSL "https://github.com/firecracker-microvm/firecracker/releases/download/v${VERSION}/firecracker-v${VERSION}-${ARCH}.tgz" | tar xz
sudo mv release-v${VERSION}-${ARCH}/firecracker-v${VERSION}-${ARCH} /usr/local/bin/firecracker
sudo chmod +x /usr/local/bin/firecracker

# Verify KVM access
ls -la /dev/kvm
# Should show: crw-rw---- 1 root kvm 10, 232 ...
```

### 2. Install Dependencies

```bash
# PostgreSQL
sudo apt install -y postgresql postgresql-contrib
sudo -u postgres createuser caged
sudo -u postgres createdb caged -O caged

# Redis
sudo apt install -y redis-server
sudo systemctl enable redis-server

# Networking tools
sudo apt install -y iptables bridge-utils
```

### 3. Download Caged Server

```bash
# Download the latest server binary
curl -fsSL https://github.com/caged-dev/caged/releases/latest/download/caged-server-linux-amd64 \
  -o /opt/caged/bin/server
chmod +x /opt/caged/bin/server
```

### 4. Download VM Images

```bash
mkdir -p /opt/caged/kernels /opt/caged/rootfs

# Linux kernel for microVMs
curl -fsSL https://github.com/caged-dev/vm-images/releases/latest/download/vmlinux-6.1.bin \
  -o /opt/caged/kernels/vmlinux-6.1.bin

# Base rootfs (Ubuntu 24.04, ~2GB)
curl -fsSL https://github.com/caged-dev/vm-images/releases/latest/download/ubuntu-24.04.ext4 \
  -o /opt/caged/rootfs/ubuntu-24.04.ext4
```

### 5. Configure Networking

```bash
# Create bridge for sandbox networking
sudo ip link add caged-br0 type bridge
sudo ip addr add 172.30.0.1/16 dev caged-br0
sudo ip link set caged-br0 up

# Enable IP forwarding
echo 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-caged.conf
sudo sysctl -p /etc/sysctl.d/99-caged.conf

# NAT for sandbox internet access
sudo iptables -t nat -A POSTROUTING -s 172.30.0.0/16 -o eth0 -j MASQUERADE
sudo iptables -A FORWARD -i caged-br0 -o eth0 -j ACCEPT
sudo iptables -A FORWARD -i eth0 -o caged-br0 -m state --state RELATED,ESTABLISHED -j ACCEPT

# Persist iptables
sudo apt install -y iptables-persistent
sudo netfilter-persistent save
```

### 6. Configure Environment

```bash
mkdir -p /opt/caged/bin /var/lib/caged/vms /var/run/caged/sockets

cat > /opt/caged/.env << 'EOF'
PORT=8080
ENV=production
DATABASE_URL=postgres://caged:@localhost:5432/caged?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=$(openssl rand -hex 32)
LOG_LEVEL=info
RUNTIME_TYPE=firecracker
EOF
```

### 7. Create Systemd Service

```bash
cat > /etc/systemd/system/caged-server.service << 'EOF'
[Unit]
Description=Caged API Server
After=network.target postgresql.service redis.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/opt/caged/bin/server
WorkingDirectory=/opt/caged
Restart=always
RestartSec=5
EnvironmentFile=/opt/caged/.env
StandardOutput=journal
StandardError=journal
SyslogIdentifier=caged-server
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable caged-server
systemctl start caged-server
```

### 8. Verify

```bash
# Check server is running
systemctl status caged-server
journalctl -t caged-server -f

# Test API
curl http://localhost:8080/health
```

## Reverse Proxy (Optional)

For HTTPS, put nginx or Caddy in front:

```bash
# Caddy (automatic HTTPS)
sudo apt install -y caddy

cat > /etc/caddy/Caddyfile << 'EOF'
api.yourdomain.com {
    reverse_proxy localhost:8080
}
EOF

sudo systemctl restart caddy
```

## Updating

```bash
# Download new binary
curl -fsSL https://github.com/caged-dev/caged/releases/latest/download/caged-server-linux-amd64 \
  -o /opt/caged/bin/server-new
chmod +x /opt/caged/bin/server-new

# Swap and restart
mv /opt/caged/bin/server /opt/caged/bin/server-old
mv /opt/caged/bin/server-new /opt/caged/bin/server
systemctl restart caged-server
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `/dev/kvm` not found | Enable virtualization in BIOS, or use a bare metal server |
| Socket timeout | Check Firecracker logs in `/var/lib/caged/vms/<id>/firecracker.log` |
| Network not working | Verify bridge exists: `ip addr show caged-br0` |
| Permission denied | Ensure server runs as root (needed for TAP devices + KVM) |
