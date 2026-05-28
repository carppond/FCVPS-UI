#!/bin/sh
# Start Nginx in background, then hub in foreground.
# tini (PID 1) handles signal forwarding to both.
set -e

# Ensure data dir is writable
mkdir -p "${SHIGUANG_DATA_DIR:-/data}"

# Start Nginx in background
nginx

# Start hub (foreground); on exit, container exits
exec /usr/local/bin/hub \
    --http-addr "${SHIGUANG_HTTP_ADDR:-127.0.0.1:8080}" \
    --data-dir "${SHIGUANG_DATA_DIR:-/data}"
