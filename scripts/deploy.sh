#!/usr/bin/env bash
# deploy.sh — 一键编译 + 上传 + 部署拾光VPS 到远程 VPS
#
# 用法:
#   ./scripts/deploy.sh
#
# 会交互式询问: VPS IP、SSH 端口、SSH 用户、域名
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

# ── 交互输入 ──
echo ""
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo -e "${CYAN}   拾光VPS 一键部署脚本${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════${NC}"
echo ""

read -rp "VPS IP 地址: " VPS_IP
read -rp "SSH 端口 [22]: " SSH_PORT
SSH_PORT="${SSH_PORT:-22}"
read -rp "SSH 用户 [root]: " SSH_USER
SSH_USER="${SSH_USER:-root}"
read -rp "域名 [your-hub.example.com]: " DOMAIN
DOMAIN="${DOMAIN:-your-hub.example.com}"
read -rp "邮箱 (SSL 证书用) [admin@example.com]: " EMAIL
EMAIL="${EMAIL:-admin@example.com}"
read -rp "VPS 架构 amd64/arm64 [amd64]: " VPS_ARCH
VPS_ARCH="${VPS_ARCH:-amd64}"

SSH_CMD="ssh -p $SSH_PORT -o StrictHostKeyChecking=accept-new ${SSH_USER}@${VPS_IP}"
SCP_CMD="scp -P $SSH_PORT"

REMOTE_DIR="/opt/shiguang-vps"

echo ""
log "目标: ${SSH_USER}@${VPS_IP}:${SSH_PORT}"
log "域名: ${DOMAIN}"
log "架构: linux/${VPS_ARCH}"
log "远程目录: ${REMOTE_DIR}"
echo ""
read -rp "确认开始? [Y/n] " CONFIRM
CONFIRM="${CONFIRM:-Y}"
[[ "$CONFIRM" =~ ^[Yy]$ ]] || { warn "已取消"; exit 0; }

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

# ── Step 4: 上传文件 ──
log "上传后端二进制..."
$SCP_CMD dist/hub-linux-"$VPS_ARCH" "${SSH_USER}@${VPS_IP}:${REMOTE_DIR}/hub"
$SCP_CMD dist/agent-linux-"$VPS_ARCH" "${SSH_USER}@${VPS_IP}:${REMOTE_DIR}/agent"

log "上传前端文件..."
# 先打包再传，比 scp -r 快很多
tar -czf /tmp/shiguang-web.tar.gz -C web/dist .
$SCP_CMD /tmp/shiguang-web.tar.gz "${SSH_USER}@${VPS_IP}:/tmp/shiguang-web.tar.gz"
$SSH_CMD "rm -rf ${REMOTE_DIR}/web/* && tar -xzf /tmp/shiguang-web.tar.gz -C ${REMOTE_DIR}/web && rm -f /tmp/shiguang-web.tar.gz"
rm -f /tmp/shiguang-web.tar.gz

$SSH_CMD "chmod +x ${REMOTE_DIR}/hub ${REMOTE_DIR}/agent"
log "文件上传完成"

# ── Step 5: 创建 systemd 服务 ──
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

# ── Step 7: 配置 Nginx ──
log "配置 Nginx..."
$SSH_CMD "
# 安装 nginx + certbot（如果没有）
if ! command -v nginx &>/dev/null; then
  apt-get update -qq && apt-get install -y -qq nginx
fi
if ! command -v certbot &>/dev/null; then
  apt-get install -y -qq certbot python3-certbot-nginx
fi

cat > /etc/nginx/sites-available/shiguang-vps << 'NGINX'
server {
    listen 80;
    server_name ${DOMAIN};

    root ${REMOTE_DIR}/web;
    index index.html;

    # 请求体大小（备份恢复上传用）
    client_max_body_size 50m;

    # API 反代
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \\\$host;
        proxy_set_header X-Real-IP \\\$remote_addr;
        proxy_set_header X-Forwarded-For \\\$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \\\$scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \\\$http_upgrade;
        proxy_set_header Connection \"upgrade\";
        proxy_read_timeout 86400;
    }

    location /download/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \\\$host;
        proxy_set_header X-Real-IP \\\$remote_addr;
    }

    location ~ ^/s/[A-Za-z0-9_-]+\\\$ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \\\$host;
        proxy_set_header X-Real-IP \\\$remote_addr;
    }

    location /api/v1/nezha {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \\\$http_upgrade;
        proxy_set_header Connection \"upgrade\";
        proxy_read_timeout 86400;
    }

    location /install-agent.sh {
        proxy_pass http://127.0.0.1:8080;
    }

    location / {
        try_files \\\$uri \\\$uri/ /index.html;
    }
}
NGINX

ln -sf /etc/nginx/sites-available/shiguang-vps /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
nginx -t && systemctl reload nginx
"
log "Nginx 配置完成"

# ── Step 8: SSL 证书 ──
log "申请 SSL 证书..."
$SSH_CMD "certbot --nginx -d ${DOMAIN} --non-interactive --agree-tos -m ${EMAIL} 2>&1 || echo '[deploy] certbot 可能已有证书或域名未解析'"

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
  echo -e "  后续更新只需重新运行: ${YELLOW}./scripts/deploy.sh${NC}"
  echo ""
else
  err "服务未正常启动，请检查: ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -n 30"
fi
