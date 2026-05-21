#!/usr/bin/env bash
# dev.sh — Start hub backend + Vite dev server concurrently.
#
# Hub runs in background; Vite runs in foreground.
# On exit (Ctrl-C or error), the hub process is killed cleanly.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

HUB_PID=""

cleanup() {
  echo ""
  echo "[dev] Shutting down..."
  if [ -n "$HUB_PID" ] && kill -0 "$HUB_PID" 2>/dev/null; then
    echo "[dev] Stopping hub (PID $HUB_PID)..."
    kill "$HUB_PID" 2>/dev/null || true
    wait "$HUB_PID" 2>/dev/null || true
    echo "[dev] Hub stopped."
  fi
}

trap cleanup EXIT INT TERM

# ---------------------------------------------------------------------------
# Start hub in background
# ---------------------------------------------------------------------------
echo "[dev] Starting hub..."
cd "$REPO_ROOT"
go run ./cmd/server &
HUB_PID=$!
echo "[dev] Hub PID: $HUB_PID"

# Give the hub a moment to start
sleep 1

if ! kill -0 "$HUB_PID" 2>/dev/null; then
  echo "[dev] ERROR: Hub failed to start." >&2
  exit 1
fi

echo "[dev] Hub running."
echo ""

# ---------------------------------------------------------------------------
# Start Vite dev server in foreground
# ---------------------------------------------------------------------------
echo "[dev] Starting Vite dev server..."
cd "$REPO_ROOT/web"
exec pnpm dev
