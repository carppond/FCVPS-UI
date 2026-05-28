# 拾光VPS (shiguang-vps)

**自托管 Clash 订阅聚合 + VPS 资产管理 + 探针监控 + 通知系统一体化平台**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

把 sub-store（订阅）、Nezha（探针）、Uptime Kuma（通知）、VPS 资产管理收敛到一份 Go 单二进制 + React 网页 + 移动端 App。

---

## 核心特性

- **订阅聚合**：URL / 上传 YAML / 手动添加，解析 12 种协议（vmess / vless / ss / ssr / trojan / hysteria / hysteria2 / tuic / wireguard / anytls / socks5 / naive）
- **算子流水线**：可视化拖拽编排 filter / sort / dedupe / regex-rename / map / output 六类算子
- **多平台订阅输出**：Clash / sing-box / Surge / V2Ray 等 11 种客户端格式
- **sub-store 兼容**：`/download/:name?token=<token>` 路由，原客户端无需改配置
- **VPS 资产管理**：记录 VPS 费用、到期日、SSH 凭据，到期自动通知
- **探针监控**：自写轻量 Go agent，上报 CPU / MEM / Disk / NetIO / Load
- **Nezha 兼容**：原 Nezha agent 只改 server 地址即可接入
- **10 渠道通知**：Telegram / Discord / Slack / Email / Bark / Gotify / Webhook / Server酱 / PushDeer / IFTTT
- **Telegram Bot 双向交互**：`/nodes`、`/refresh <sub>`、`/traffic` 等命令运维
- **静默模式**：所有路径返回 404，登录页面隐藏在随机前缀下
- **移动端 App**：iOS + Android（Expo / React Native），含 SSH 终端、订阅复制、VPS 管理

---

## 项目结构

```
.
├── cmd/server/        # Go 后端入口
├── cmd/agent/         # 探针 agent 入口
├── internal/          # Go 后端代码
├── web/               # React 前端 (Vite + TanStack Router)
├── mobile/            # Expo 移动端 (iOS + Android)
├── migrations/        # SQLite 迁移
├── scripts/           # 部署 + 开发脚本
└── docs/              # 文档
```

---

## 部署到 VPS

### 1. 准备

- 一台 Linux VPS（Debian 12 / Ubuntu 22.04+）
- 一个解析到 VPS 的域名
- 一个用于申请 SSL 证书的邮箱
- Mac/Linux 本机已安装 Go 1.21+、Node.js 20+、pnpm

### 2. 一键部署

```bash
git clone <你的私有仓库>
cd shiguang-vps
./scripts/deploy.sh
```

脚本会交互式询问：
- VPS IP、SSH 端口、SSH 用户
- 域名（必填）
- 邮箱（必填，用于 Let's Encrypt 证书）
- VPS 架构（amd64 / arm64）

部署完成后访问 `https://<your-domain>` 即可。首次启动会在 systemd 日志里打印一次性 admin 密码：

```bash
ssh root@<vps> "journalctl -u shiguang-vps -n 20 | grep ADMIN"
```

### 3. 更新部署

```bash
./scripts/deploy.sh --update
```

只重新编译上传二进制和前端，不动 Nginx 和 SSL。

---

## 本地开发

### 后端 + Web

```bash
# 后端（端口 8080）
go run ./cmd/server/

# Web（端口 5173，代理到 8080）
cd web && pnpm install && pnpm dev
```

### 后端 + 移动端

```bash
./scripts/dev-mobile.sh
```

一键启动后端 + Expo dev server + iOS 模拟器。

### 移动端真机测试

```bash
cd mobile
npx expo run:ios --device --configuration Release
```

需要 Mac + Xcode + iPhone 用 USB 连接。免费 Apple ID 可签名 7 天。

> SSH 终端功能依赖原生模块，仅在 dev build 下工作（Expo Go 不支持）。

---

## 系统要求

- **VPS**：512MB RAM 起步；hub 稳态 < 50MB，agent 稳态 < 30MB
- **OS**：Linux（amd64 / arm64）、macOS、Windows
- **浏览器**：Chrome 110+ / Edge 110+ / Firefox 115+ / Safari 16+
- **iOS**：14.0+（移动端 App）
- **Android**：6.0+（移动端 App）

---

## 文档

| 文档 | 说明 |
|------|------|
| [快速开始](./docs/user/quickstart.md) | 部署与首次配置 |
| [API 契约](./docs/04-api-contract.md) | 完整 REST API 说明 |
| [架构](./docs/03-architecture.md) | 后端架构与模块划分 |
| [从 sub-store 迁移](./docs/user/migration-from-substore.md) | sub-store 用户切换指引 |
| [从 Nezha 迁移](./docs/user/migration-from-nezha.md) | Nezha agent 接入 |
| [通知渠道配置](./docs/user/notification-setup.md) | 10 个渠道配置说明 |
| [流水线常用模式](./docs/user/pipeline-cookbook.md) | YAML 示例与配方 |
| [常见问题](./docs/user/troubleshooting.md) | 静默模式 / 密码找回 / agent 连接 |
| [安全说明](./SECURITY.md) | 漏洞报告 + 已知限制 |

---

## License

[MIT](./LICENSE)
