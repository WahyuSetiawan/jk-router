#!/bin/bash
set -e

GITHUB_USER="juragankoding"
GITHUB_REPO="jkrouter"
VERSION="${1:-latest}"
TARGET_DIR="${2:-/usr/local/bin}"

case "$(uname -s)" in
  Linux)   OS=linux ;;
  Darwin)  OS=darwin ;;
  *)       echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64)  ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *)       echo "Unsupported arch: $(uname -m)" >&2; exit 1 ;;
esac

URL="https://github.com/${GITHUB_USER}/${GITHUB_REPO}/releases/download/${VERSION}/jkrouter-${OS}-${ARCH}"
echo "Downloading ${URL} ..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if curl -fSL "$URL" -o "$TMPDIR/jkrouter"; then
  chmod +x "$TMPDIR/jkrouter"
  if [ -w "$TARGET_DIR" ]; then
    mv "$TMPDIR/jkrouter" "$TARGET_DIR/jkrouter"
  else
    echo "Need sudo to write to $TARGET_DIR:"
    sudo mv "$TMPDIR/jkrouter" "$TARGET_DIR/jkrouter"
  fi
  echo "Installed to ${TARGET_DIR}/jkrouter"
  "$TARGET_DIR/jkrouter" --version
else
  echo "Download failed. Trying without version tag (latest)..."
  curl -fSL "${URL/-${VERSION}/}" -o "$TMPDIR/jkrouter" || {
    echo "Failed to download. Make sure the release exists at:"
    echo "  https://github.com/${GITHUB_USER}/${GITHUB_REPO}/releases"
    exit 1
  }
  chmod +x "$TMPDIR/jkrouter"
  if [ -w "$TARGET_DIR" ]; then
    mv "$TMPDIR/jkrouter" "$TARGET_DIR/jkrouter"
  else
    sudo mv "$TMPDIR/jkrouter" "$TARGET_DIR/jkrouter"
  fi
  echo "Installed to ${TARGET_DIR}/jkrouter"
fi
