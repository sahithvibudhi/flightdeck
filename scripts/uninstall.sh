#!/bin/bash
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root (try: sudo bash)."
  exit 1
fi

PURGE=0
for arg in "$@"; do
  case "$arg" in
    --purge) PURGE=1 ;;
    *)       echo "Unknown option: $arg"; echo "Usage: uninstall.sh [--purge]"; exit 1 ;;
  esac
done

echo "Uninstalling flightdeck..."

# Stop and disable the systemd service if it exists.
if command -v systemctl >/dev/null 2>&1; then
  if systemctl list-unit-files flightdeck.service 2>/dev/null | grep -q '^flightdeck.service'; then
    echo "Stopping and disabling the flightdeck service..."
    systemctl stop flightdeck 2>/dev/null || true
    systemctl disable flightdeck 2>/dev/null || true
  else
    echo "No flightdeck service registered; nothing to stop."
  fi
else
  echo "systemctl not found; skipping service stop."
fi

if [ -f /etc/systemd/system/flightdeck.service ]; then
  rm -f /etc/systemd/system/flightdeck.service
  echo "Removed /etc/systemd/system/flightdeck.service"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload
  fi
else
  echo "No service file at /etc/systemd/system/flightdeck.service; nothing to remove."
fi

if [ -f /usr/local/bin/flightdeck ]; then
  rm -f /usr/local/bin/flightdeck
  echo "Removed /usr/local/bin/flightdeck"
else
  echo "No binary at /usr/local/bin/flightdeck; nothing to remove."
fi

if [ "$PURGE" -eq 1 ]; then
  if [ -d /var/flightdeck ]; then
    rm -rf /var/flightdeck
    echo "Removed /var/flightdeck (--purge)"
  else
    echo "No data directory at /var/flightdeck; nothing to purge."
  fi
else
  if [ -d /var/flightdeck ]; then
    echo "Kept the data directory /var/flightdeck (app sources, logs, database)."
    echo "Remove it with: sudo rm -rf /var/flightdeck (or re-run this script with --purge)."
  fi
fi

echo ""
echo "flightdeck is uninstalled. Two things are intentionally left alone:"
echo ""
echo "  - Apps that are currently running keep running. flightdeck never kills"
echo "    your apps on exit; stop them yourself if you want them gone."
echo "  - The Caddy binary at /usr/local/bin/caddy is left in place, since other"
echo "    services may use it. Remove it with: sudo rm -f /usr/local/bin/caddy"
