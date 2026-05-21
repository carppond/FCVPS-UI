# Nezha → 拾光VPS 迁移指引（部署侧）

> 完整迁移文档请参阅：[docs/user/migration-from-nezha.md](../docs/user/migration-from-nezha.md)（T-33）

本文仅补充**部署层面**的操作步骤：如何用拾光VPS 容器替换现有的 Nezha 容器。

---

## 前提

- Nezha 已在目标主机以 Docker / docker-compose 方式运行。
- 拥有主机 root 或 sudo 权限。
- 域名解析已指向本机（如果需要 HTTPS 反代）。

---

## 方案一：用 docker-compose 替换（推荐）

### 1. 备份 Nezha 数据

```bash
# 停止 Nezha（不删除数据）
docker compose stop nezha  # 或 docker stop <nezha-container>

# 备份数据目录（可选，不影响拾光VPS）
cp -r /opt/nezha /opt/nezha.bak
```

### 2. 保留相同端口，切换到拾光VPS

将下面的内容保存为 `docker-compose.yml`（替换原有的 Nezha compose 文件）：

```yaml
services:
  shiguang-vps:
    image: ghcr.io/shiguang-vps/shiguang-vps:latest
    container_name: shiguang-vps
    restart: unless-stopped
    ports:
      - "8080:8080"           # 与 Nezha dashboard 的端口对应，按需修改
    volumes:
      - ./data:/data           # 拾光VPS 持久化目录（与 Nezha 数据互不干扰）
    environment:
      SHIGUANG_DATA_DIR: /data
      SHIGUANG_HTTP_ADDR: ":8080"
      SHIGUANG_LOG_LEVEL: info
      SHIGUANG_LOG_FORMAT: json
```

然后启动：

```bash
docker compose up -d
docker compose logs -f shiguang-vps | grep -i ADMIN
```

记录首次启动打印的 admin 密码（只显示一次）。

### 3. 重新安装 agent（每台被监控 VPS）

在每台 VPS 上运行 install-agent.sh，或用 docker-compose.agent.yml：

```bash
# 方式 A：一键脚本
curl -fsSL https://raw.githubusercontent.com/shiguang-vps/shiguang-vps/main/deploy/install-agent.sh \
  | bash -s -- \
      --hub-url wss://hub.example.com/api/agent/ws \
      --token   <从拾光VPS管理界面生成的token> \
      --agent-id <uuidgen>

# 方式 B：docker-compose
HUB_URL=wss://hub.example.com/api/agent/ws \
TOKEN=your-token \
AGENT_ID=$(uuidgen) \
docker compose -f /path/to/deploy/docker-compose.agent.yml up -d
```

---

## 方案二：仅替换二进制（不用 Docker）

如果 Nezha 以 systemd 方式运行：

```bash
# 停止 Nezha service
sudo systemctl stop nezha-dashboard

# 安装拾光VPS hub（一键脚本）
curl -fsSL https://raw.githubusercontent.com/shiguang-vps/shiguang-vps/main/deploy/install.sh | sudo bash

# 等待 10s 后查看 admin 密码
journalctl -u shiguang-vps | grep ADMIN
```

---

## 端口与反代对照

| 项目 | 默认端口 | 反代路径 |
|------|---------|---------|
| Nezha dashboard | 8008 | `/` |
| 拾光VPS hub | 8080 | `/` |
| Nezha gRPC (agent) | 5555 | — |
| 拾光VPS agent WS | 8080 | `/api/agent/ws` |

> 拾光VPS agent 使用 WebSocket over HTTP/HTTPS，与 hub 同端口，**无需额外开放 gRPC 端口**。

---

## 数据迁移

Nezha 与拾光VPS 数据格式不兼容，无自动导入工具。  
建议：
- 服务器列表：在拾光VPS 管理界面手动重新添加（每台VPS 约1分钟）。
- 历史流量数据：无法迁移（重新开始统计）。
- 通知规则：在拾光VPS 的通知渠道页面重新配置（支持10种渠道）。

---

## 回滚

如需回滚到 Nezha：

```bash
docker compose stop shiguang-vps
docker compose up -d nezha   # 恢复原 compose 文件
```

Nezha 数据目录未被修改，直接启动即可恢复原状。
