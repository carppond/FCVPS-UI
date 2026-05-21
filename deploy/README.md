# 拾光VPS — 部署指南

本目录包含所有部署相关文件。支持四种部署方式。

---

## 1. Docker（推荐：最简单）

```bash
docker run -d \
  --name shiguang-vps \
  --restart unless-stopped \
  -p 8080:8080 \
  -v "$(pwd)/data:/data" \
  ghcr.io/shiguang-vps/shiguang-vps:latest

# 查看 admin 密码（首次启动）
docker logs shiguang-vps | grep ADMIN
```

## 2. docker-compose（推荐：持久化配置）

```bash
# Hub
cp deploy/docker-compose.yml docker-compose.yml
docker compose up -d
docker compose logs -f | grep ADMIN

# Agent（在被监控的 VPS 上运行）
HUB_URL=wss://hub.example.com/api/agent/ws \
TOKEN=your-token \
AGENT_ID=$(uuidgen) \
docker compose -f deploy/docker-compose.agent.yml up -d
```

## 3. 一键脚本 + systemd（Linux amd64/arm64）

```bash
# Hub 服务器
curl -fsSL https://raw.githubusercontent.com/shiguang-vps/shiguang-vps/main/deploy/install.sh | sudo bash

# Agent（被监控的 VPS）
curl -fsSL https://raw.githubusercontent.com/shiguang-vps/shiguang-vps/main/deploy/install-agent.sh \
  | sudo bash -s -- \
      --hub-url wss://hub.example.com/api/agent/ws \
      --token   <token> \
      --agent-id <uuidgen>
```

脚本支持 `--uninstall` 和 `--upgrade`。

## 4. 手动（单二进制）

从 [Releases](https://github.com/shiguang-vps/shiguang-vps/releases) 下载对应平台的 binary：

```bash
# 验证 checksum
sha256sum -c checksums.txt --ignore-missing

# 运行
./shiguang-vps --data-dir ./data
```

支持平台：`linux/amd64`、`linux/arm64`、`darwin/amd64`、`darwin/arm64`、`windows/amd64`

---

## 文件清单

| 文件 | 说明 |
|------|------|
| `Dockerfile` | Hub 多阶段构建（node→go→distroless，< 30MB）|
| `Dockerfile.agent` | Agent 多阶段构建（go→distroless，< 15MB）|
| `docker-compose.yml` | Hub compose 模板 |
| `docker-compose.agent.yml` | Agent compose 模板（network_mode: host）|
| `shiguang-vps.service` | Hub systemd unit |
| `shiguang-vps-agent.service` | Agent systemd unit |
| `install.sh` | Hub 一键安装脚本（Linux only）|
| `install-agent.sh` | Agent 一键安装脚本（Linux only）|
| `nezha-migration.md` | 从 Nezha 迁移的部署侧指引 |

---

## 数据目录

所有持久化数据（SQLite + 日志）位于 `/data`（Docker）或 `/var/lib/shiguang-vps`（systemd）。

## 反代配置

拾光VPS 监听 `:8080`，推荐在前面放 Nginx / Caddy：

```nginx
location / {
    proxy_pass         http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header   Upgrade $http_upgrade;
    proxy_set_header   Connection "upgrade";
    proxy_set_header   Host $host;
}
```

> Agent WebSocket（`/api/agent/ws`）需要 `Upgrade` 头透传，上面的配置已覆盖。
