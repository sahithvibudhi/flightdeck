#!/bin/bash
set -e

ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
VERSION="latest"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/nestops"

# Map architecture names
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing nestops..."

# Download binary
curl -sSL "https://github.com/nestops/nestops/releases/download/${VERSION}/nestops-${OS}-${ARCH}" \
  -o /tmp/nestops
chmod +x /tmp/nestops
mv /tmp/nestops "$INSTALL_DIR/nestops"

# Create data directory
mkdir -p "$DATA_DIR/apps"
mkdir -p "$DATA_DIR/caddy"

# Install Caddy if not present
if ! command -v caddy &>/dev/null; then
  echo "Installing Caddy..."
  curl -sSL "https://caddyserver.com/api/download?os=${OS}&arch=${ARCH}" \
    -o /usr/local/bin/caddy
  chmod +x /usr/local/bin/caddy
fi

# Install systemd service
cat > /etc/systemd/system/nestops.service <<EOF
[Unit]
Description=nestops
After=network.target

[Service]
ExecStart=/usr/local/bin/nestops
Restart=always
RestartSec=5
WorkingDirectory=/var/nestops

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable nestops

# Run setup wizard
/usr/local/bin/nestops

# Start service
systemctl start nestops

echo ""
echo "nestops is running."
echo "Access it at the address shown above."
