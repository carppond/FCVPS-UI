#!/usr/bin/env bash
# deploy.sh — 一键编译 + 上传 + 部署拾光VPS 到远程 VPS
#
# 用法:
#   ./scripts/deploy.sh           # 首次部署（含 Nginx + SSL）
#   ./scripts/deploy.sh --update  # 更新代码（只替换二进制+前端，重启服务）
#
# 不影响已有的 3X-UI 等服务
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ── 颜色 ──
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log()  { echo -e "${GREEN}[deploy]${NC} $*"; }
warn() { echo -e "${YELLOW}[deploy]${NC} $*"; }
err()  { echo -e "${RED}[deploy]${NC} $*" >&2; }

# ── 模式判断 ──
UPDATE_ONLY=false
if [[ "${1:-}" == "--update" || "${1:-}" == "-u" ]]; then
  UPDATE_ONLY=true
fi

# ── 交互输入 ──
echo ""
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
if $UPDATE_ONLY; then
  echo -e "${CYAN}   拾光VPS 更新部署（跳过 Nginx/SSL）${NC}"
else
  echo -e "${CYAN}   拾光VPS 首次部署${NC}"
fi
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo ""

read -rp "VPS IP 地址: " VPS_IP
read -rp "SSH 端口 [22]: " SSH_PORT
SSH_PORT="${SSH_PORT:-22}"
read -rp "SSH 用户 [root]: " SSH_USER
SSH_USER="${SSH_USER:-root}"
read -rp "VPS 架构 amd64/arm64 [amd64]: " VPS_ARCH
VPS_ARCH="${VPS_ARCH:-amd64}"

if ! $UPDATE_ONLY; then
  read -rp "域名 [your-hub.example.com]: " DOMAIN
  DOMAIN="${DOMAIN:-your-hub.example.com}"
  read -rp "邮箱 (SSL 证书用) [admin@example.com]: " EMAIL
  EMAIL="${EMAIL:-admin@example.com}"
fi

REMOTE_DIR="/opt/shiguang-vps"

# SSH ControlMaster — 只输一次密码，后续复用连接
SSH_SOCKET="/tmp/shiguang-deploy-$$"
SSH_COMMON="-o StrictHostKeyChecking=accept-new -o ControlMaster=auto -o ControlPath=${SSH_SOCKET} -o ControlPersist=300"
SSH_CMD="ssh -p $SSH_PORT ${SSH_COMMON} ${SSH_USER}@${VPS_IP}"
SCP_OPTS="-P $SSH_PORT ${SSH_COMMON}"

cleanup() { ssh -p "$SSH_PORT" -O exit -o ControlPath="${SSH_SOCKET}" "${SSH_USER}@${VPS_IP}" 2>/dev/null || true; }
trap cleanup EXIT

echo ""
log "目标: ${SSH_USER}@${VPS_IP}:${SSH_PORT}"
log "架构: linux/${VPS_ARCH}"
log "模式: $($UPDATE_ONLY && echo '更新（跳过 Nginx/SSL）' || echo '首次部署')"
echo ""
read -rp "确认开始? [Y/n] " CONFIRM
CONFIRM="${CONFIRM:-Y}"
[[ "$CONFIRM" =~ ^[Yy]$ ]] || { warn "已取消"; exit 0; }

# 建立 SSH 连接（输一次密码）
log "建立 SSH 连接..."
$SSH_CMD echo connected

# ── Step 1: 编译后端 ──
log "编译后端 (linux/${VPS_ARCH})..."
GOOS=linux GOARCH="$VPS_ARCH" go build \
  -ldflags="-s -w" -trimpath \
  -o dist/hub-linux-"$VPS_ARCH" ./cmd/server

GOOS=linux GOARCH="$VPS_ARCH" go build \
  -ldflags="-s -w" -trimpath \
  -o dist/agent-linux-"$VPS_ARCH" ./cmd/agent

log "后端编译完成"

# ── Step 2: 编译前端 ──
log "编译前端..."
cd web
if command -v pnpm &>/dev/null; then
  pnpm install --frozen-lockfile 2>/dev/null || pnpm install
  pnpm build
elif command -v npm &>/dev/null; then
  npm ci 2>/dev/null || npm install
  npm run build
else
  err "需要 pnpm 或 npm"
  exit 1
fi
cd "$REPO_ROOT"
log "前端编译完成"

# ── Step 3: 创建远程目录 ──
log "创建远程目录..."
$SSH_CMD "mkdir -p ${REMOTE_DIR}/data ${REMOTE_DIR}/web"

# ── Step 4: 停止服务 + 上传文件 ──
log "停止服务（避免覆盖运行中的二进制）..."
$SSH_CMD "systemctl stop shiguang-vps 2>/dev/null || true"

log "上传后端二进制..."
scp ${SCP_OPTS} dist/hub-linux-"$VPS_ARCH" "${SSH_USER}@${VPS_IP}:${REMOTE_DIR}/hub"
scp ${SCP_OPTS} dist/agent-linux-"$VPS_ARCH" "${SSH_USER}@${VPS_IP}:${REMOTE_DIR}/agent"

log "上传前端文件..."
tar -czf /tmp/shiguang-web.tar.gz -C web/dist .
scp ${SCP_OPTS} /tmp/shiguang-web.tar.gz "${SSH_USER}@${VPS_IP}:/tmp/shiguang-web.tar.gz"
$SSH_CMD "rm -rf ${REMOTE_DIR}/web/* && tar -xzf /tmp/shiguang-web.tar.gz -C ${REMOTE_DIR}/web && rm -f /tmp/shiguang-web.tar.gz"
rm -f /tmp/shiguang-web.tar.gz

$SSH_CMD "chmod +x ${REMOTE_DIR}/hub ${REMOTE_DIR}/agent"
log "文件上传完成"

# ── Step 5: 创建/更新 systemd 服务 ──
log "配置 systemd 服务..."
$SSH_CMD "cat > /etc/systemd/system/shiguang-vps.service << 'UNIT'
[Unit]
Description=Shiguang VPS Hub
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${REMOTE_DIR}
ExecStart=${REMOTE_DIR}/hub --http-addr 127.0.0.1:8080 --data-dir ${REMOTE_DIR}/data
Restart=always
RestartSec=5
Environment=SHIGUANG_LOG_LEVEL=info
Environment=SHIGUANG_LOG_FORMAT=json

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable shiguang-vps
systemctl restart shiguang-vps
sleep 2
"
log "服务已启动"

# ── Step 6: 获取初始密码 ──
log "获取初始管理员密码..."
ADMIN_LINE=$($SSH_CMD "journalctl -u shiguang-vps --no-pager -n 50 2>/dev/null | grep 'ADMIN BOOTSTRAPPED' | tail -1" || true)
if [[ -n "$ADMIN_LINE" ]]; then
  echo ""
  echo -e "${YELLOW}════════════════════════════════════════════${NC}"
  echo -e "${YELLOW}  初始管理员账号（仅显示一次，请保存！）${NC}"
  echo -e "${YELLOW}════════════════════════════════════════════${NC}"
  echo "$ADMIN_LINE"
  echo -e "${YELLOW}════════════════════════════════════════════${NC}"
  echo ""
else
  warn "未找到初始密码（可能非首次部署，密码沿用之前的）"
fi

# ── 更新模式到此结束 ──
if $UPDATE_ONLY; then
  STATUS=$($SSH_CMD "systemctl is-active shiguang-vps" || true)
  if [[ "$STATUS" == "active" ]]; then
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  更新完成！服务已重启${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
    echo ""
  else
    err "服务未正常启动，请检查: ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -n 30"
  fi
  exit 0
fi

# ── Step 7: 配置 Nginx（仅首次） ──
log "配置 Nginx..."
# 生成 Nginx 配置到本地临时文件，避免远程 shell 转义问题
NGINX_CONF=$(mktemp)
cat > "$NGINX_CONF" << 'NGINXEOF'
server {
    server_name DOMAIN_PLACEHOLDER;

    root /opt/shiguang-vps/web;
    index index.html;

    client_max_body_size 50m;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }

    location /download/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location ~ ^/s/[A-Za-z0-9_-]+$ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/v1/nezha {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }

    location /install-agent.sh {
        proxy_pass http://127.0.0.1:8080;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }

    listen 80;
}
NGINXEOF

# 替换域名占位符
sed -i.bak "s/DOMAIN_PLACEHOLDER/${DOMAIN}/g" "$NGINX_CONF"
rm -f "${NGINX_CONF}.bak"

$SSH_CMD "
if ! command -v nginx &>/dev/null; then
  apt-get update -qq && apt-get install -y -qq nginx
fi
if ! command -v certbot &>/dev/null; then
  apt-get install -y -qq certbot python3-certbot-nginx
fi
"

# 上传 Nginx 配置（避免 shell 转义问题）
scp ${SCP_OPTS} "$NGINX_CONF" "${SSH_USER}@${VPS_IP}:/etc/nginx/sites-available/shiguang-vps"
rm -f "$NGINX_CONF"

$SSH_CMD "
ln -sf /etc/nginx/sites-available/shiguang-vps /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
nginx -t && systemctl reload nginx
"
log "Nginx 配置完成"

# ── Step 8: SSL 证书（仅首次） ──
log "申请 SSL 证书..."
$SSH_CMD "certbot --nginx -d ${DOMAIN} --non-interactive --agree-tos -m ${EMAIL} 2>&1 || echo '[deploy] certbot: 可能已有证书或域名未解析'"

# ── Step 9: 验证 ──
log "验证部署..."
STATUS=$($SSH_CMD "systemctl is-active shiguang-vps" || true)
if [[ "$STATUS" == "active" ]]; then
  echo ""
  echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
  echo -e "${GREEN}  部署成功！${NC}"
  echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
  echo ""
  echo -e "  面板地址:  ${CYAN}https://${DOMAIN}${NC}"
  echo -e "  后端状态:  ${GREEN}running${NC}"
  echo -e "  数据目录:  ${REMOTE_DIR}/data/"
  echo -e "  日志查看:  ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -f"
  echo ""
  echo -e "  后续更新:  ${YELLOW}./scripts/deploy.sh --update${NC}"
  echo ""
else
  err "服务未正常启动，请检查: ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -n 30"
fi
