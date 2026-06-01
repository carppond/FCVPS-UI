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

## 部署

> **没有域名也能用。** 三种方式都支持用 IP + HTTP 直接访问（`http://<你的IP>`），域名/HTTPS 是可选项。
> 注意：HTTP 为明文传输，登录密码、订阅 token、SSH 凭据都不加密——**纯 IP 模式建议仅在内网、局域网，或已套 WireGuard / Tailscale 等加密隧道时使用**；公网长期部署强烈建议配域名 + HTTPS。

### 方式 1：Docker（推荐）

```bash
docker run -d --name shiguang-vps \
  -p 8080:80 \
  -v $PWD/data:/data \
  --restart unless-stopped \
  ghcr.io/carppond/fcvps-ui:latest
```

或用 docker-compose：

```bash
curl -O https://raw.githubusercontent.com/carppond/FCVPS-UI/main/docker-compose.yml
docker compose up -d
```

查看初始 admin 密码：

```bash
docker logs shiguang-vps 2>&1 | grep ADMIN
```

访问 `http://<你的IP>:8080` 即可，**无需域名**。想上 HTTPS，在前面接 Nginx/Caddy + 域名 + Let's Encrypt 反代到 8080 即可。

> **换端口**：`-p` 左边是对外端口，随意改以避开已占用端口。例如 `-p 9000:80` → `http://<你的IP>:9000`；容器内部端口固定 `80`，无需关心。

### 方式 2：一键脚本部署到 VPS（可选 Nginx + SSL）

适合直接部署在裸 VPS 上。脚本会问域名——**填了就自动配 Nginx + Let's Encrypt HTTPS，留空就用 IP + HTTP**：

```bash
git clone https://github.com/carppond/FCVPS-UI.git
cd FCVPS-UI
./scripts/deploy.sh
```

脚本会交互式询问：
- VPS IP、SSH 端口、SSH 用户
- 域名（**可留空**；留空 → IP + HTTP，跳过 SSL）
- 邮箱（仅填了域名时需要，用于 Let's Encrypt 证书）
- **访问端口**（对外端口，默认 HTTP 80 / HTTPS 443，可自定义）
- **后端端口**（仅本机回环，默认 8080，可自定义）
- VPS 架构（amd64 / arm64）

> **端口冲突自动探测**：脚本连上 VPS 后会实时检测你填的端口是否已被占用（如 3X-UI、其他面板），占用会提示换端口，避免部署到一半失败。访问端口和后端端口都能自定义，方便与已有服务共存。
> 域名模式也支持自定义 HTTPS 端口（如 `https://<域名>:8443`）：脚本用 80 端口完成 Let's Encrypt 验证后，会把证书装到你指定的端口上。

部署完成后会**统一打印**访问地址、初始账号密码、后端端口等信息。填域名访问 `https://<your-domain>`，留空则访问 `http://<你的IP>`。若需重新查看初始密码：

```bash
ssh root@<vps> "journalctl -u shiguang-vps -n 20 | grep ADMIN"
```

更新部署：

```bash
./scripts/deploy.sh --update
```

### 方式 3：手动二进制

从 [Releases](https://github.com/carppond/FCVPS-UI/releases) 下载对应平台二进制：

```bash
chmod +x hub-linux-amd64
./hub-linux-amd64 --http-addr :8080 --data-dir ./data
```

`--http-addr` 想监听哪个端口/地址都行，例如 `--http-addr :9000`（任意端口）或 `--http-addr 127.0.0.1:8080`（仅本机，前面再接反代）。自带前端的静态文件需要单独 build（或用 Docker 方式）。

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
| [iOS 编译与自签安装](./docs/user/ios-build.md) | 基础版/完整版两个 IPA、证书要求、编译打包与重签步骤 |
| [Android 编译与打包](./docs/user/android-build.md) | APK 安装、编译环境、本地/EAS 云构建 |
| [卸载](./docs/user/uninstall.md) | 探针 / Docker / 脚本 / 二进制各方式的卸载步骤 |
| [流水线常用模式](./docs/user/pipeline-cookbook.md) | YAML 示例与配方 |
| [常见问题](./docs/user/troubleshooting.md) | 静默模式 / 密码找回 / agent 连接 |
| [安全说明](./SECURITY.md) | 漏洞报告 + 已知限制 |

---

## 致谢

本项目的设计与实现参考、借鉴了以下优秀的开源项目，在此一并致谢：

### 灵感来源

- **[iluobei/miaomiaowu](https://github.com/iluobei/miaomiaowu)** — 妙妙屋，本项目最初的设计灵感来源，订阅聚合 + 探针 + 通知一体化的产品思路
- **[sub-store-org/Sub-Store](https://github.com/sub-store-org/Sub-Store)** — Sub-Store，订阅管理与算子流水线的概念来源；本项目保留了 `/download/:name?token=<token>` 兼容路由
- **[nezhahq/nezha](https://github.com/nezhahq/nezha)** — 哪吒监控，探针 agent 设计参考；本项目支持 Nezha v2 agent 直接接入

### 规则集数据源

- **[MetaCubeX/meta-rules-dat](https://github.com/MetaCubeX/meta-rules-dat)** — Mihomo (Clash Meta) 规则集的官方数据源，本项目内置的 48 个规则集预设全部来自此项目

### 后端依赖（Go）

- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)** — 纯 Go 实现的 SQLite 驱动（无 CGo 依赖），支撑无缝交叉编译
- **[dop251/goja](https://github.com/dop251/goja)** — 纯 Go 实现的 ECMAScript 5.1 引擎，用于自定义脚本沙箱
- **[gorilla/websocket](https://github.com/gorilla/websocket)** — WebSocket 实现，用于探针 agent 与 hub 的双向通信
- **[golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)** — bcrypt 密码哈希、SSH 协议、TOTP 支持
- **[gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)** — YAML 解析与渲染，订阅产物生成
- **[mihomo](https://github.com/MetaCubeX/mihomo)** — Clash Meta 内核，订阅产物的目标客户端

### 前端依赖（Web）

- **[React 19](https://react.dev/)** + **[Vite 7](https://vitejs.dev/)** — 构建工具链
- **[TanStack Router](https://tanstack.com/router) / [Query](https://tanstack.com/query)** — 路由 + 数据层
- **[Tailwind CSS v4](https://tailwindcss.com/)** + **[Radix UI](https://www.radix-ui.com/)** — 样式与无障碍组件
- **[lucide-react](https://lucide.dev/)** — 图标库
- **[cmdk](https://cmdk.paco.me/)** — 命令面板（⌘K）
- **[@dnd-kit](https://dndkit.com/)** — 流水线拖拽编排

### 移动端依赖（Mobile）

- **[Expo SDK 56](https://expo.dev/)** + **[Expo Router](https://expo.github.io/router/)** — React Native 跨端框架与文件路由
- **[react-native-ssh-sftp](https://github.com/shaqian/react-native-ssh-sftp)** — 原生 SSH 库（封装 NMSSH + JSch），支撑移动端 SSH 终端
- **[Zustand](https://zustand-demo.pmnd.rs/)** — 轻量状态管理

### 客户端兼容性

- **[Mihomo / Clash Meta](https://github.com/MetaCubeX/mihomo)**、**[sing-box](https://github.com/SagerNet/sing-box)**、**[Surge](https://nssurge.com/)**、**[Quantumult X](https://apps.apple.com/app/quantumult-x/id1443988620)**、**[Loon](https://nsloon.com/)** — 订阅产物的目标客户端

---

## License

[MIT](./LICENSE)
