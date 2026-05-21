#!/usr/bin/env bash
# install-agent.sh — One-line agent installer for shiguang-vps
#
# Usage (on the VPS you want to monitor):
#   curl -fsSL https://raw.githubusercontent.com/shiguang-vps/shiguang-vps/main/deploy/install-agent.sh \
#     | bash -s -- --hub-url wss://hub.example.com/api/agent/ws \
#                  --token   <token-from-hub> \
#                  --agent-id <uuid>
#
# Supported: Linux amd64 / arm64 with systemd
# NOT supported: macOS (use Docker), Windows (use Docker)
#
# Template version: v1  (T-28 compatible)
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
GITHUB_REPO="${GITHUB_REPO:-shiguang-vps/shiguang-vps}"
BINARY_NAME="shiguang-vps-agent"
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/shiguang-vps-agent"
SERVICE_USER="shiguang-agent"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"
VERSION=""
MODE="install"  # install | uninstall | upgrade

# Required fields (populated from CLI or env)
HUB_URL="${SHIGUANG_HUB_URL:-}"
TOKEN="${SHIGUANG_AGENT_TOKEN:-}"
AGENT_ID="${SHIGUANG_AGENT_ID:-}"
AGENT_TAGS="${SHIGUANG_AGENT_TAGS:-}"
HEARTBEAT_SECONDS="${SHIGUANG_AGENT_HEARTBEAT_SECONDS:-30}"

# ---------------------------------------------------------------------------
# Helpers (identical to install.sh for maintainability)
# ---------------------------------------------------------------------------
log()  { printf '\033[1;32m[shiguang-agent]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[shiguang-agent]\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[1;31m[shiguang-agent]\033[0m ERROR: %s\n' "$*" >&2; exit 1; }

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
    Darwin) err "macOS is not supported by this installer. Use Docker." ;;
    MINGW*|MSYS*|CYGWIN*) err "Windows is not supported. Use Docker." ;;
    *) err "Unsupported OS: $os" ;;
  esac
}

check_systemd() {
  if ! command -v systemctl &>/dev/null; then
    err "systemd is required. OpenRC / runit / SysV are not supported."
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
    err "Neither curl nor wget is available."
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
Description=Shiguang VPS Agent
Documentation=https://github.com/${GITHUB_REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
ExecStart=${INSTALL_DIR}/${BINARY_NAME}
WorkingDirectory=${DATA_DIR}
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${BINARY_NAME}
Environment=SHIGUANG_HUB_URL=${HUB_URL}
Environment=SHIGUANG_AGENT_TOKEN=${TOKEN}
Environment=SHIGUANG_AGENT_ID=${AGENT_ID}
Environment=SHIGUANG_AGENT_TAGS=${AGENT_TAGS}
Environment=SHIGUANG_AGENT_HEARTBEAT_SECONDS=${HEARTBEAT_SECONDS}
Environment=SHIGUANG_LOG_LEVEL=info
Environment=SHIGUANG_LOG_FORMAT=json
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=${DATA_DIR}
ProtectKernelTunables=yes
ProtectControlGroups=yes
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
      --hub-url)     HUB_URL="$2";           shift 2 ;;
      --token)       TOKEN="$2";             shift 2 ;;
      --agent-id)    AGENT_ID="$2";          shift 2 ;;
      --tags)        AGENT_TAGS="$2";        shift 2 ;;
      --interval)    HEARTBEAT_SECONDS="$2"; shift 2 ;;
      --version)     VERSION="$2";           shift 2 ;;
      --data-dir)    DATA_DIR="$2";          shift 2 ;;
      --uninstall)   MODE="uninstall";       shift ;;
      --upgrade)     MODE="upgrade";         shift ;;
      -h|--help)
        printf 'Usage: %s --hub-url <wss://...> --token <tok> --agent-id <uuid> [--tags a,b] [--interval 30]\n' "$0"
        exit 0 ;;
      *)
        err "Unknown argument: $1. Run with --help." ;;
    esac
  done
}

validate_required() {
  [[ -z "$HUB_URL" ]]  && err "--hub-url (or SHIGUANG_HUB_URL) is required"
  [[ -z "$TOKEN" ]]    && err "--token (or SHIGUANG_AGENT_TOKEN) is required"
  [[ -z "$AGENT_ID" ]] && err "--agent-id (or SHIGUANG_AGENT_ID) is required"
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
  log "Agent removed. Data directory ${DATA_DIR} is preserved."
}

do_install() {
  local os arch download_url checksum_url tmpdir binary_file checksum_file

  validate_required
  os="$(detect_os)"
  arch="$(detect_arch)"

  [[ -z "$VERSION" ]] && VERSION="$(fetch_latest_version)"
  [[ -z "$VERSION" ]] && err "Could not determine latest version. Set VERSION manually."

  log "Installing ${BINARY_NAME} ${VERSION} (${os}/${arch})..."

  download_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/agent-${os}-${arch}"
  checksum_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/checksums.txt"

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  binary_file="${tmpdir}/${BINARY_NAME}"
  checksum_file="${tmpdir}/checksums.txt"

  log "Downloading agent binary..."
  download_file "$download_url" "$binary_file"

  log "Downloading checksums..."
  download_file "$checksum_url" "$checksum_file"

  log "Verifying integrity..."
  cp "$binary_file" "${tmpdir}/agent-${os}-${arch}"
  verify_sha256 "${tmpdir}/agent-${os}-${arch}" "$checksum_file"

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

  # Write systemd unit (contains hub URL + token in env)
  write_service_file

  # Enable and start
  systemctl daemon-reload
  systemctl enable --now "${BINARY_NAME}"

  log ""
  log "Agent installed and running!"
  log "Status:  systemctl status ${BINARY_NAME}"
  log "Logs:    journalctl -u ${BINARY_NAME} -f"
}

do_upgrade() {
  log "Upgrading ${BINARY_NAME}..."
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
