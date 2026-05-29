#!/usr/bin/env bash
# deploy.sh — 一键编译 + 上传 + 部署拾光VPS 到远程 VPS
#
# 用法:
#   ./scripts/deploy.sh           # 首次部署（域名 → Nginx+SSL；留空 → IP+HTTP）
#   ./scripts/deploy.sh --update  # 更新代码（只替换二进制+前端，重启服务）
#
# 不影响已有的 3X-UI 等服务：访问端口 / 后端端口均可自定义，且部署前实时探测占用
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

# remote_listener <port> — 打印远程占用该端口的监听行（含进程），空表示端口空闲
remote_listener() {
  local port="$1"
  $SSH_CMD "ss -ltnpH 2>/dev/null | awk '{n=split(\$4,a,\":\"); if (a[n]==\"${port}\") print}'" 2>/dev/null || true
}

# ask_port <提示语> <默认端口> — 选定端口写入全局 CHOSEN_PORT；占用时让用户改端口或强制使用
ask_port() {
  local label="$1" def="$2" p hit yn
  while true; do
    read -rp "  ${label} [${def}]: " p
    p="${p:-$def}"
    if ! [[ "$p" =~ ^[0-9]+$ ]] || (( p < 1 || p > 65535 )); then
      warn "  端口需为 1-65535 的数字"
      continue
    fi
    hit=$(remote_listener "$p")
    if [[ -n "$hit" ]]; then
      warn "  端口 ${p} 已被占用："
      echo "$hit" | sed 's/^/        /'
      read -rp "  仍要使用 ${p}? [y=强制使用 / 回车=换端口]: " yn
      if [[ "$yn" =~ ^[Yy]$ ]]; then CHOSEN_PORT="$p"; return 0; fi
      continue
    fi
    CHOSEN_PORT="$p"
    return 0
  done
}

# app_locations — 输出 Nginx 应用反代 location 块；__BP__ 占位后端端口，nginx 变量保持字面量
app_locations() {
  cat <<'LOC'
    root /opt/shiguang-vps/web;
    index index.html;
    client_max_body_size 50m;

    location /api/ {
        proxy_pass http://127.0.0.1:__BP__;
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
        proxy_pass http://127.0.0.1:__BP__;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location ~ ^/s/[A-Za-z0-9_-]+$ {
        proxy_pass http://127.0.0.1:__BP__;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /api/v1/nezha {
        proxy_pass http://127.0.0.1:__BP__;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }

    location /install-agent.sh {
        proxy_pass http://127.0.0.1:__BP__;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
LOC
}

# install_nginx_conf <本地配置文件> — 上传并启用配置，校验后 reload
install_nginx_conf() {
  scp ${SCP_OPTS} "$1" "${SSH_USER}@${VPS_IP}:/etc/nginx/sites-available/shiguang-vps"
  $SSH_CMD "
    mkdir -p /etc/nginx/sites-enabled
    ln -sf /etc/nginx/sites-available/shiguang-vps /etc/nginx/sites-enabled/
    rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
    nginx -t && systemctl reload nginx
  "
}

# print_summary <访问地址> — 部署结束统一打印：地址 + 账号 + 密码 + 常用命令
print_summary() {
  local url="$1"
  echo ""
  echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
  echo -e "${GREEN}  部署成功！${NC}"
  echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
  echo ""
  echo -e "  面板地址:  ${CYAN}${url}${NC}"
  if [[ -n "$ADMIN_USER" && -n "$ADMIN_PASS" ]]; then
    echo ""
    echo -e "  ${YELLOW}初始管理员账号（仅本次显示，请立即保存）：${NC}"
    echo -e "    用户名:  ${CYAN}${ADMIN_USER}${NC}"
    echo -e "    密  码:  ${CYAN}${ADMIN_PASS}${NC}"
  else
    echo -e "  ${YELLOW}账号:${NC}      沿用已有管理员（非首次部署，密码未变）"
  fi
  echo ""
  echo -e "  后端端口:  127.0.0.1:${BACKEND_PORT}"
  echo -e "  数据目录:  ${REMOTE_DIR}/data/"
  echo -e "  查看日志:  ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -f"
  echo -e "  后续更新:  ${YELLOW}./scripts/deploy.sh --update${NC}"
  echo -e "${GREEN}═══════════════════════════════════════════════${NC}"
  echo ""
}

# ── 模式判断 ──
UPDATE_ONLY=false
if [[ "${1:-}" == "--update" || "${1:-}" == "-u" ]]; then
  UPDATE_ONLY=true
fi

# ── 交互输入：基础连接信息 ──
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

REMOTE_DIR="/opt/shiguang-vps"

# SSH ControlMaster — 只输一次密码，后续复用连接
SSH_SOCKET="/tmp/shiguang-deploy-$$"
SSH_COMMON="-o StrictHostKeyChecking=accept-new -o ControlMaster=auto -o ControlPath=${SSH_SOCKET} -o ControlPersist=300"
SSH_CMD="ssh -p $SSH_PORT ${SSH_COMMON} ${SSH_USER}@${VPS_IP}"
SCP_OPTS="-P $SSH_PORT ${SSH_COMMON}"

cleanup() { ssh -p "$SSH_PORT" -O exit -o ControlPath="${SSH_SOCKET}" "${SSH_USER}@${VPS_IP}" 2>/dev/null || true; }
trap cleanup EXIT

# 先建立 SSH 连接（输一次密码）——后续端口探测、读取现有配置都依赖它
echo ""
log "建立 SSH 连接（${SSH_USER}@${VPS_IP}:${SSH_PORT}）..."
$SSH_CMD echo connected >/dev/null

# ── 域名 + 端口配置 ──
DOMAIN=""
EMAIL=""
EXTERNAL_PORT=""
HTTPS_SUFFIX=""

if $UPDATE_ONLY; then
  # 更新模式：从现有 systemd 单元读回后端端口，避免被重置回默认值
  BACKEND_PORT=$($SSH_CMD "grep -oE 'http-addr 127.0.0.1:[0-9]+' /etc/systemd/system/shiguang-vps.service 2>/dev/null | grep -oE '[0-9]+$' | head -1" || true)
  BACKEND_PORT="${BACKEND_PORT:-8080}"
  log "沿用现有后端端口: 127.0.0.1:${BACKEND_PORT}"
  # 沿用首次部署写入的受保护端口（面板防自锁），更新时不要丢
  PROTECTED_PORTS=$($SSH_CMD "grep -oE 'SHIGUANG_FIREWALL_PROTECTED_PORTS=[0-9,]*' /etc/systemd/system/shiguang-vps.service 2>/dev/null | head -1 | cut -d= -f2" || true)
else
  echo ""
  echo -e "${CYAN}域名可留空：${NC}"
  echo -e "  • 填域名  → 自动配置 Nginx + Let's Encrypt HTTPS（推荐公网部署）"
  echo -e "  • 留空    → 用 IP + HTTP 访问，跳过 SSL"
  echo ""
  read -rp "域名（如 vpn.example.com，可留空走 IP 模式）: " DOMAIN
  if [[ -n "$DOMAIN" ]]; then
    read -rp "邮箱（SSL 证书申请用）: " EMAIL
    if [[ -z "$EMAIL" ]]; then
      err "填了域名就需要邮箱来申请证书；想跳过 HTTPS 请把域名也留空"
      exit 1
    fi
  else
    warn "未填域名 → IP + HTTP 模式：流量为明文（含登录密码、订阅 token、SSH 凭据）"
    warn "建议仅在内网 / 局域网，或已套 WireGuard·Tailscale 等加密隧道时使用"
  fi

  echo ""
  echo -e "${CYAN}端口配置（会实时探测远程是否被占用，可避开 3X-UI 等已有服务）：${NC}"
  if [[ -n "$DOMAIN" ]]; then
    ask_port "访问端口（HTTPS，默认 443）" 443
  else
    ask_port "访问端口（HTTP，浏览器访问用）" 80
  fi
  EXTERNAL_PORT="$CHOSEN_PORT"
  ask_port "后端端口（仅本机 127.0.0.1 回环，Nginx 反代到它）" 8080
  BACKEND_PORT="$CHOSEN_PORT"

  # 域名模式：非标准 HTTPS 端口时，跳转目标与访问地址要带端口
  if [[ -n "$DOMAIN" && "$EXTERNAL_PORT" != "443" ]]; then
    HTTPS_SUFFIX=":${EXTERNAL_PORT}"
  fi

  # 域名模式申请证书走 ACME HTTP-01，需要 80 端口可达；若被占给出路而非硬锁
  if [[ -n "$DOMAIN" && "$EXTERNAL_PORT" != "80" ]]; then
    port80=$(remote_listener 80)
    if [[ -n "$port80" ]]; then
      warn "签发证书需要 80 端口（ACME HTTP-01 验证），但 80 当前被占用："
      echo "$port80" | sed 's/^/      /'
      echo -e "  ${YELLOW}出路：${NC}"
      echo -e "    1) 临时停掉占用 80 的服务，部署完再起"
      echo -e "    2) 手动用 DNS-01 方式签证书（不需要 80），再把证书路径填进 Nginx"
      echo -e "    3) 放弃 HTTPS，重跑脚本时域名留空走 IP+HTTP"
      read -rp "  仍要继续尝试（证书可能失败，失败时面板保留 80 端口 HTTP 可用）? [y/N] " goon
      [[ "$goon" =~ ^[Yy]$ ]] || { warn "已取消"; exit 0; }
    fi
  fi

  # 面板受保护端口：SSH + 对外访问端口（域名模式再加 80），写进 systemd env，
  # 让 hub 拒绝从面板删除这些端口的放行规则（防自锁）。
  PROTECTED_PORTS="${SSH_PORT},${EXTERNAL_PORT}"
  [[ -n "$DOMAIN" ]] && PROTECTED_PORTS="${PROTECTED_PORTS},80"
fi

# ── 确认 ──
echo ""
log "目标: ${SSH_USER}@${VPS_IP}:${SSH_PORT}  架构 linux/${VPS_ARCH}"
if ! $UPDATE_ONLY; then
  if [[ -n "$DOMAIN" ]]; then
    log "访问: https://${DOMAIN}${HTTPS_SUFFIX}   后端: 127.0.0.1:${BACKEND_PORT}"
  else
    SHOW_PORT=""; [[ "$EXTERNAL_PORT" != "80" ]] && SHOW_PORT=":${EXTERNAL_PORT}"
    log "访问: http://${VPS_IP}${SHOW_PORT}   后端: 127.0.0.1:${BACKEND_PORT}"
  fi
fi
log "模式: $($UPDATE_ONLY && echo '更新（跳过 Nginx/SSL）' || echo '首次部署')"
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

# ── Step 5: 创建/更新 systemd 服务（后端端口可配） ──
log "配置 systemd 服务（后端 127.0.0.1:${BACKEND_PORT}）..."
$SSH_CMD "cat > /etc/systemd/system/shiguang-vps.service << 'UNIT'
[Unit]
Description=Shiguang VPS Hub
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${REMOTE_DIR}
ExecStart=${REMOTE_DIR}/hub --http-addr 127.0.0.1:${BACKEND_PORT} --data-dir ${REMOTE_DIR}/data
Restart=always
RestartSec=5
Environment=SHIGUANG_LOG_LEVEL=info
Environment=SHIGUANG_LOG_FORMAT=json
Environment=SHIGUANG_FIREWALL_PROTECTED_PORTS=${PROTECTED_PORTS}

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable shiguang-vps
systemctl restart shiguang-vps
sleep 2
"
log "服务已启动"

# ── Step 6: 获取初始密码（仅捕获，统一在结尾打印） ──
log "获取初始管理员密码..."
ADMIN_LINE=$($SSH_CMD "journalctl -u shiguang-vps --no-pager -n 50 2>/dev/null | grep 'ADMIN BOOTSTRAPPED' | tail -1" || true)
ADMIN_USER=""
ADMIN_PASS=""
if [[ -n "$ADMIN_LINE" ]]; then
  # 日志为 JSON 格式，解析 username / password 字段
  ADMIN_USER=$(echo "$ADMIN_LINE" | grep -oE '"username":"[^"]*"' | head -1 | sed 's/"username":"//; s/"$//')
  ADMIN_PASS=$(echo "$ADMIN_LINE" | grep -oE '"password":"[^"]*"' | head -1 | sed 's/"password":"//; s/"$//')
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
    echo -e "  后端端口:  127.0.0.1:${BACKEND_PORT}"
    echo -e "  查看日志:  ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -f"
    echo ""
  else
    err "服务未正常启动，请检查: ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -n 30"
  fi
  exit 0
fi

# ── Step 7: 配置 Nginx（仅首次） ──
log "配置 Nginx..."
$SSH_CMD "command -v nginx >/dev/null 2>&1 || { apt-get update -qq && apt-get install -y -qq nginx; }"

APP_LOC=$(app_locations | sed "s/__BP__/${BACKEND_PORT}/g")
NGINX_CONF=$(mktemp)

SUMMARY_URL=""

if [[ -z "$DOMAIN" ]]; then
  # ── IP + HTTP：单 server，监听自定义访问端口 ──
  {
    echo "server {"
    echo "    listen ${EXTERNAL_PORT} default_server;"
    echo "    server_name _;"
    printf '%s\n' "$APP_LOC"
    echo "}"
  } > "$NGINX_CONF"
  install_nginx_conf "$NGINX_CONF"
  rm -f "$NGINX_CONF"
  log "Nginx 配置完成（IP + HTTP）"
  warn "IP 模式：跳过 SSL 证书申请"
  if [[ "$EXTERNAL_PORT" == "80" ]]; then
    SUMMARY_URL="http://${VPS_IP}  (HTTP 明文，注意使用环境)"
  else
    SUMMARY_URL="http://${VPS_IP}:${EXTERNAL_PORT}  (HTTP 明文，注意使用环境)"
  fi
else
  # ── 域名 + HTTPS：先 80 做 ACME，再在自定义端口上 ssl ──
  $SSH_CMD "command -v certbot >/dev/null 2>&1 || { apt-get update -qq && apt-get install -y -qq certbot; }"
  $SSH_CMD "mkdir -p /var/www/certbot"

  # 阶段 1：仅 80（ACME webroot 验证 + 跳转 https）
  {
    echo "server {"
    echo "    listen 80;"
    echo "    server_name ${DOMAIN};"
    echo "    location /.well-known/acme-challenge/ { root /var/www/certbot; }"
    echo "    location / { return 301 https://\$host${HTTPS_SUFFIX}\$request_uri; }"
    echo "}"
  } > "$NGINX_CONF"
  install_nginx_conf "$NGINX_CONF"
  log "Nginx 阶段1（80 端口 ACME）就绪"

  # 申请证书（webroot 方式，续期同样走 80，不依赖面板端口）
  log "申请 SSL 证书..."
  $SSH_CMD "certbot certonly --webroot -w /var/www/certbot -d ${DOMAIN} --non-interactive --agree-tos -m ${EMAIL} 2>&1 || echo '[deploy] certbot: 申请失败（域名未解析到本机 / 80 被占 / 频率限制）'"

  if $SSH_CMD "test -f /etc/letsencrypt/live/${DOMAIN}/fullchain.pem"; then
    # 阶段 2：80 跳转 + 自定义端口 ssl
    {
      echo "server {"
      echo "    listen 80;"
      echo "    server_name ${DOMAIN};"
      echo "    location /.well-known/acme-challenge/ { root /var/www/certbot; }"
      echo "    location / { return 301 https://\$host${HTTPS_SUFFIX}\$request_uri; }"
      echo "}"
      echo "server {"
      echo "    listen ${EXTERNAL_PORT} ssl;"
      echo "    server_name ${DOMAIN};"
      echo "    ssl_certificate /etc/letsencrypt/live/${DOMAIN}/fullchain.pem;"
      echo "    ssl_certificate_key /etc/letsencrypt/live/${DOMAIN}/privkey.pem;"
      printf '%s\n' "$APP_LOC"
      echo "}"
    } > "$NGINX_CONF"
    install_nginx_conf "$NGINX_CONF"
    rm -f "$NGINX_CONF"
    log "Nginx 配置完成（域名 + HTTPS:${EXTERNAL_PORT}）"
    SUMMARY_URL="https://${DOMAIN}${HTTPS_SUFFIX}"
  else
    rm -f "$NGINX_CONF"
    warn "证书未签发成功 —— 面板暂以 HTTP 在 80 端口提供，可访问 http://${DOMAIN}"
    warn "排查（域名 A 记录是否指向本机、80 是否可达）后重跑脚本，或手动签发证书"
    SUMMARY_URL="http://${DOMAIN}  (证书未签发，暂为 HTTP)"
  fi
fi

# ── Step 7.5: ufw 放行（仅当 ufw 已启用，纯加法，绝不主动 enable） ──
# 已启用 ufw 时，把 SSH 端口 + 访问端口（域名模式再加 80）放行，避免它们被默认
# 拒绝策略挡住。未启用时不碰防火墙——强行 enable 会把未识别的其他服务一起闷死，
# 且容易锁掉 SSH。
log "检查 ufw 防火墙..."
FW_PORTS="${SSH_PORT} ${EXTERNAL_PORT}"
[[ -n "$DOMAIN" ]] && FW_PORTS="${FW_PORTS} 80"
if $SSH_CMD "command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'"; then
  for p in $FW_PORTS; do
    $SSH_CMD "ufw allow ${p}/tcp >/dev/null 2>&1 || true"
  done
  log "ufw 已启用：已放行端口 ${FW_PORTS}（tcp）"
else
  warn "ufw 未启用（或未安装）→ 未改动防火墙，端口默认可达"
  warn "如需启用：务必先 ufw allow ${SSH_PORT}/tcp 和访问端口，再 ufw enable，否则会断开 SSH"
fi

# ── Step 8: 验证 + 统一汇总 ──
log "验证部署..."
STATUS=$($SSH_CMD "systemctl is-active shiguang-vps" || true)
if [[ "$STATUS" == "active" ]]; then
  print_summary "$SUMMARY_URL"
else
  err "服务未正常启动，请检查: ssh ${SSH_USER}@${VPS_IP} journalctl -u shiguang-vps -n 30"
fi
