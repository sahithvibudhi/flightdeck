#!/bin/bash
set -e

ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
VERSION="latest"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/nestops"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing nestops..."
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
    echo "  Could not install git automatically. Please install git and try again."
    exit 1
  fi
fi
echo "  git $(git --version | cut -d' ' -f3)"

if ! command -v caddy &>/dev/null; then
  echo "  Caddy not found. Installing..."
  curl -sSL "https://caddyserver.com/api/download?os=${OS}&arch=${ARCH}" \
    -o /usr/local/bin/caddy
  chmod +x /usr/local/bin/caddy
fi
echo "  caddy $(caddy version | cut -d' ' -f1)"

echo ""

curl -sSL "https://github.com/nestops/nestops/releases/download/${VERSION}/nestops-${OS}-${ARCH}" \
  -o /tmp/nestops
chmod +x /tmp/nestops
mv /tmp/nestops "$INSTALL_DIR/nestops"

mkdir -p "$DATA_DIR/apps"
mkdir -p "$DATA_DIR/caddy"

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

/usr/local/bin/nestops

systemctl start nestops

echo ""
echo "nestops is running."
echo "Access it at the address shown above."
