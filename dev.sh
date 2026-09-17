#!/usr/bin/env bash
# Start JKRouter dev: backend first, wait for healthy, then frontend.
set -e

PORT="${PORT:-20127}"
DATA_DIR="${DATA_DIR:-$HOME/.jkrouter}"
BIN="${BIN:-/tmp/jkrouter}"

cd "$(dirname "$0")"

# ── Build backend if needed ───────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  echo "❌ go not found. Run: nix develop" >&2
  exit 1
fi

echo "🔨 Building jkrouter..."
make build

# ── Start backend in background ───────────────────────────────────────────
echo "🚀 Starting backend on :${PORT} ..."
${BIN} serve --port "${PORT}" --data-dir "${DATA_DIR}" &
BPID=$!
trap 'kill $BPID 2>/dev/null; exit' INT TERM EXIT

# ── Wait for backend health ──────────────────────────────────────────────
echo "⏳ Waiting for backend health check..."
for i in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${PORT}/api/dashboard/health" >/dev/null 2>&1; then
    echo "✅ Backend ready on :${PORT}"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "❌ Backend did not become healthy in 30s" >&2
    kill $BPID
    exit 1
  fi
  sleep 1
done

# ── Start frontend ────────────────────────────────────────────────────────
echo "🎨 Starting Nuxt dev server..."
cd web
bun dev
