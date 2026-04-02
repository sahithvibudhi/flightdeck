#!/bin/bash
set -e

if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root."
  exit 1
fi

ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
VERSION="${1:-latest}"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing flightdeck..."

curl -sSL "https://github.com/sahithvibudhi/flightdeck/releases/download/${VERSION}/flightdeck-${OS}-${ARCH}" \
  -o /tmp/flightdeck
chmod +x /tmp/flightdeck
mv /tmp/flightdeck /usr/local/bin/flightdeck

mkdir -p /var/flightdeck/apps
mkdir -p /var/flightdeck/caddy

cat > /etc/systemd/system/flightdeck.service <<EOF
[Unit]
Description=flightdeck
After=network.target

[Service]
ExecStart=/usr/local/bin/flightdeck
Restart=always
RestartSec=5
WorkingDirectory=/var/flightdeck
Environment=FLIGHTDECK_DATA_DIR=/var/flightdeck

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable flightdeck

echo ""
echo "Installed. Run the setup wizard:"
echo ""
echo "  flightdeck"
echo ""
echo "Then start the service:"
echo ""
echo "  systemctl start flightdeck"
