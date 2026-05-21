#!/usr/bin/env bash
# build-release.sh — Multi-platform release build for hub and agent binaries.
#
# Targets: linux/amd64  linux/arm64  darwin/amd64  darwin/arm64  windows/amd64
# Outputs: dist/<binary>-<os>-<arch>[.exe]  +  dist/checksums.txt
#
# Usage:
#   VERSION=v1.2.3 ./scripts/build-release.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$REPO_ROOT/dist"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X main.version=$VERSION"

TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

BINARIES=(
  "hub:./cmd/server"
  "agent:./cmd/agent"
)

mkdir -p "$DIST"

echo "[build-release] Version: $VERSION"
echo "[build-release] Output:  $DIST"
echo ""

for target in "${TARGETS[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"

  for binary_def in "${BINARIES[@]}"; do
    BIN_NAME="${binary_def%:*}"
    BIN_PKG="${binary_def#*:}"

    OUT_NAME="${BIN_NAME}-${OS}-${ARCH}"
    [[ "$OS" == "windows" ]] && OUT_NAME="${OUT_NAME}.exe"
    OUT_PATH="$DIST/$OUT_NAME"

    echo "[build-release] Building $OUT_NAME ..."
    GOOS="$OS" GOARCH="$ARCH" go build \
      -ldflags="$LDFLAGS" \
      -trimpath \
      -o "$OUT_PATH" \
      "$BIN_PKG"
  done
done

# ---------------------------------------------------------------------------
# SHA-256 checksums
# ---------------------------------------------------------------------------
echo ""
echo "[build-release] Generating checksums..."
cd "$DIST"
CHECKSUM_FILE="checksums.txt"
rm -f "$CHECKSUM_FILE"

if command -v sha256sum &>/dev/null; then
  sha256sum ./* >"$CHECKSUM_FILE"
elif command -v shasum &>/dev/null; then
  shasum -a 256 ./* >"$CHECKSUM_FILE"
else
  echo "[build-release] ERROR: neither sha256sum nor shasum found." >&2
  exit 1
fi

echo "[build-release] Checksums written to $DIST/$CHECKSUM_FILE"
echo ""
cat "$CHECKSUM_FILE"
echo ""
echo "[build-release] Done."
