#!/bin/bash
set -e

if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root."
  exit 1
fi

ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
VERSION="${1:-latest}"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/nestops"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing flightdeck..."
echo ""

echo "Checking dependencies..."

if ! command -v git &>/dev/null; then
  echo "  Git not found. Installing..."
  if command -v apt-get &>/dev/null; then
    apt-get update -qq && apt-get install -y -qq git
  elif command -v yum &>/dev/null; then
    yum install -y -q git
  elif command -v apk &>/dev/null; then
    apk add --quiet git
  else
    echo "  Could not install git. Please install git and try again."
    exit 1
  fi
fi
echo "  git $(git --version | cut -d' ' -f3)"

if ! command -v caddy &>/dev/null; then
  echo "  Caddy not found. Installing..."
  curl -sSL "https://caddyserver.com/api/download?os=${OS}&arch=${ARCH}" \
    -o /usr/local/bin/caddy
  chmod +x /usr/local/bin/caddy
  if ! caddy version &>/dev/null; then
    echo "  Caddy installation failed."
    exit 1
  fi
fi
echo "  caddy $(caddy version | cut -d' ' -f1)"

echo ""

curl -sSL "https://github.com/nestops/nestops/releases/download/${VERSION}/nestops-${OS}-${ARCH}" \
  -o /tmp/nestops
chmod +x /tmp/nestops
mv /tmp/nestops "$INSTALL_DIR/nestops"

mkdir -p "$DATA_DIR/apps"
mkdir -p "$DATA_DIR/caddy"

cat > /etc/systemd/system/flightdeck.service <<EOF
[Unit]
Description=flightdeck
After=network.target

[Service]
ExecStart=/usr/local/bin/nestops
Restart=always
RestartSec=5
WorkingDirectory=/var/nestops
Environment=NESTOPS_DATA_DIR=/var/nestops

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable flightdeck

echo ""
echo "Run the setup wizard:"
echo "  /usr/local/bin/nestops"
echo ""
echo "Then start the service:"
echo "  systemctl start flightdeck"
