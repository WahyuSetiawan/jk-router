#!/bin/bash
set -e
ARCH=$(uname -m)
case $ARCH in
  x86_64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac
URL="https://github.com/juragankoding/jkrouter/releases/latest/download/jkrouter-linux-${GOARCH}"
echo "Downloading $URL ..."
curl -fsSL "$URL" -o /tmp/jkrouter
chmod +x /tmp/jkrouter
sudo mv /tmp/jkrouter /usr/local/bin/jkrouter
echo "Installed to /usr/local/bin/jkrouter"
jkrouter --version || true
