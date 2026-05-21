#!/usr/bin/env bash
# install.sh — One-line hub installer for shiguang-vps
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/shiguang-vps/shiguang-vps/main/deploy/install.sh | bash
#   # Or with options:
#   bash install.sh [--version v1.2.3] [--data-dir /var/lib/shiguang-vps] [--uninstall] [--upgrade]
#
# Supported: Linux amd64 / arm64 with systemd
# NOT supported: macOS (use Docker), Windows (use Docker)
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
GITHUB_REPO="${GITHUB_REPO:-shiguang-vps/shiguang-vps}"
BINARY_NAME="shiguang-vps"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/shiguang-vps"
SERVICE_USER="shiguang"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"
VERSION=""
MODE="install"  # install | uninstall | upgrade

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log()  { printf '\033[1;32m[shiguang]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[shiguang]\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31m[shiguang]\033[0m ERROR: %s\n' "$*" >&2; exit 1; }

require_root() {
  if [[ "$EUID" -ne 0 ]]; then
    err "This script must be run as root. Try: sudo bash $0 $*"
  fi
}

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "$machine" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) err "Unsupported architecture: $machine. Use Docker on this platform." ;;
  esac
}

detect_os() {
  local os
  os="$(uname -s)"
  case "$os" in
    Linux) echo "linux" ;;
    Darwin) err "macOS is not supported by this installer. Please use Docker: https://ghcr.io/shiguang-vps/shiguang-vps" ;;
    MINGW*|MSYS*|CYGWIN*) err "Windows is not supported by this installer. Please use Docker: https://ghcr.io/shiguang-vps/shiguang-vps" ;;
    *) err "Unsupported OS: $os" ;;
  esac
}

check_systemd() {
  if ! command -v systemctl &>/dev/null; then
    err "systemd is required. OpenRC / runit / SysV are not supported by this installer."
  fi
}

fetch_latest_version() {
  if command -v curl &>/dev/null; then
    curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/'
  elif command -v wget &>/dev/null; then
    wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/'
  else
    err "Neither curl nor wget is available. Install one and retry."
  fi
}

download_file() {
  local url="$1"
  local dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL --retry 3 -o "$dest" "$url"
  else
    wget -qO "$dest" "$url"
  fi
}

verify_sha256() {
  local file="$1"
  local checksum_file="$2"
  local basename
  basename="$(basename "$file")"

  if command -v sha256sum &>/dev/null; then
    grep " ${basename}$" "$checksum_file" | sha256sum --check --status
  elif command -v shasum &>/dev/null; then
    grep " ${basename}$" "$checksum_file" | shasum -a 256 --check --status
  else
    warn "Cannot verify checksum: sha256sum / shasum not found. Proceeding without verification."
  fi
}

write_service_file() {
  cat >"$SERVICE_FILE" <<EOF
[Unit]
Description=Shiguang VPS Hub
Documentation=https://github.com/${GITHUB_REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
WorkingDirectory=${DATA_DIR}
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${BINARY_NAME}
Environment=SHIGUANG_DATA_DIR=${DATA_DIR}
Environment=SHIGUANG_HTTP_ADDR=:8080
Environment=SHIGUANG_LOG_LEVEL=info
Environment=SHIGUANG_LOG_FORMAT=json
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=${DATA_DIR}
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
}

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        VERSION="$2"; shift 2 ;;
      --data-dir)
        DATA_DIR="$2"; shift 2 ;;
      --uninstall)
        MODE="uninstall"; shift ;;
      --upgrade)
        MODE="upgrade"; shift ;;
      -h|--help)
        printf 'Usage: %s [--version v1.x.x] [--data-dir /path] [--uninstall] [--upgrade]\n' "$0"
        exit 0 ;;
      *)
        err "Unknown argument: $1. Run with --help for usage." ;;
    esac
  done
}

# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------
do_uninstall() {
  log "Stopping and disabling ${BINARY_NAME} service..."
  systemctl stop "${BINARY_NAME}" 2>/dev/null || true
  systemctl disable "${BINARY_NAME}" 2>/dev/null || true
  rm -f "$SERVICE_FILE"
  systemctl daemon-reload
  rm -f "${INSTALL_DIR}/${BINARY_NAME}"
  log "Service removed. Data directory ${DATA_DIR} is preserved."
  log "To delete data: rm -rf ${DATA_DIR}"
}

do_install() {
  local os arch download_url checksum_url tmpdir binary_file checksum_file

  os="$(detect_os)"
  arch="$(detect_arch)"

  [[ -z "$VERSION" ]] && VERSION="$(fetch_latest_version)"
  [[ -z "$VERSION" ]] && err "Could not determine latest version. Set VERSION manually."

  log "Installing ${BINARY_NAME} ${VERSION} (${os}/${arch})..."

  download_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/hub-${os}-${arch}"
  checksum_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/checksums.txt"

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  binary_file="${tmpdir}/${BINARY_NAME}"
  checksum_file="${tmpdir}/checksums.txt"

  log "Downloading binary..."
  download_file "$download_url" "$binary_file"

  log "Downloading checksums..."
  download_file "$checksum_url" "$checksum_file"

  log "Verifying integrity..."
  # Rename for checksum match (checksums.txt lists hub-linux-amd64)
  cp "$binary_file" "${tmpdir}/hub-${os}-${arch}"
  verify_sha256 "${tmpdir}/hub-${os}-${arch}" "$checksum_file"

  # Create service user
  if ! id -u "$SERVICE_USER" &>/dev/null; then
    log "Creating system user: ${SERVICE_USER}"
    useradd --system --no-create-home --shell /sbin/nologin "$SERVICE_USER"
  fi

  # Data directory
  mkdir -p "$DATA_DIR"
  chown "${SERVICE_USER}:${SERVICE_USER}" "$DATA_DIR"
  chmod 750 "$DATA_DIR"

  # Install binary
  install -o root -g root -m 755 "$binary_file" "${INSTALL_DIR}/${BINARY_NAME}"

  # Write systemd unit
  write_service_file

  # Enable and start
  systemctl daemon-reload
  systemctl enable --now "${BINARY_NAME}"

  log ""
  log "Installation complete!"
  log "Service status: systemctl status ${BINARY_NAME}"
  log ""
  log "Waiting 10 s then showing the admin bootstrap password..."
  sleep 10
  log ""
  log ">>> Admin credentials (first boot only) <<<"
  journalctl -u "${BINARY_NAME}" --no-pager -n 50 | grep -i "ADMIN" || \
    warn "Password line not found in logs yet. Try: journalctl -u ${BINARY_NAME} | grep ADMIN"
}

do_upgrade() {
  log "Upgrading ${BINARY_NAME}..."
  # Stop service, re-run install (binary replacement), restart
  systemctl stop "${BINARY_NAME}" 2>/dev/null || true
  do_install
  systemctl start "${BINARY_NAME}"
  log "Upgrade complete."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  parse_args "$@"
  require_root
  check_systemd

  case "$MODE" in
    install)   do_install ;;
    uninstall) do_uninstall ;;
    upgrade)   do_upgrade ;;
    *) err "Unknown mode: $MODE" ;;
  esac
}

main "$@"
