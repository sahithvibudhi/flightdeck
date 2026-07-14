#!/bin/bash
set -euo pipefail

REPO="sahithvibudhi/flightdeck"

if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root (try: sudo bash)."
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required. Install it first (e.g. apt install curl) and re-run."
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

if [ "$OS" != "linux" ]; then
  echo "flightdeck runs on Linux servers. Detected: $OS"
  exit 1
fi

ASSET="flightdeck-${OS}-${ARCH}"
if [ "$VERSION" = "latest" ]; then
  BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
fi

echo "Installing flightdeck (${VERSION}, ${OS}/${ARCH})..."

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

if ! curl -fsSL "${BASE_URL}/${ASSET}" -o "${TMP_DIR}/flightdeck"; then
  echo ""
  echo "Download failed: ${BASE_URL}/${ASSET}"
  echo "No published release may exist yet. You can build from source instead:"
  echo ""
  echo "  git clone https://github.com/${REPO}.git && cd flightdeck && make build"
  echo ""
  exit 1
fi

if curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt" 2>/dev/null; then
  (cd "$TMP_DIR" && grep " ${ASSET}\$" checksums.txt | sed "s| ${ASSET}| flightdeck|" | sha256sum -c -) \
    || { echo "Checksum verification failed."; exit 1; }
else
  echo "Warning: checksums.txt not found in release; skipping verification."
fi

chmod +x "${TMP_DIR}/flightdeck"
mv "${TMP_DIR}/flightdeck" /usr/local/bin/flightdeck

mkdir -p /var/flightdeck/apps
mkdir -p /var/flightdeck/caddy

# Best-effort dependencies: git (for GitHub deploys) and Caddy (for
# domains + automatic SSL). Failures are not fatal — both can be
# installed later from the Settings page.
if ! command -v git >/dev/null 2>&1; then
  echo "Installing git..."
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -qq && apt-get install -y -qq git || echo "  git install failed — install it later from Settings."
  elif command -v yum >/dev/null 2>&1; then
    yum install -y -q git || echo "  git install failed — install it later from Settings."
  elif command -v apk >/dev/null 2>&1; then
    apk add --quiet git || echo "  git install failed — install it later from Settings."
  else
    echo "  No supported package manager found — install git later from Settings."
  fi
fi

if ! command -v caddy >/dev/null 2>&1; then
  echo "Installing Caddy (for domains + automatic SSL)..."
  if curl -fsSL "https://caddyserver.com/api/download?os=linux&arch=${ARCH}" -o /usr/local/bin/caddy; then
    chmod +x /usr/local/bin/caddy
    echo "  Installed $(caddy version 2>/dev/null | head -c 40)"
  else
    rm -f /usr/local/bin/caddy
    echo "  Caddy download failed — install it later from Settings."
  fi
fi

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
systemctl restart flightdeck

IP=$(hostname -I 2>/dev/null | awk '{print $1}')

echo ""
echo "flightdeck is installed and running."
echo ""
echo "Finish setup in your browser:"
echo ""
echo "  http://${IP:-<your-server-ip>}:3000"
echo ""
echo "(You can also run 'sudo flightdeck' in a terminal for interactive setup.)"
