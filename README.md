# 拾光VPS (shiguang-vps)

**个人自托管 Clash 订阅聚合 + 探针流量观测 + 通知系统一体化平台**

[![CI](https://github.com/shiguang-vps/shiguang-vps/actions/workflows/test.yml/badge.svg)](https://github.com/shiguang-vps/shiguang-vps/actions)
[![Version](https://img.shields.io/github/v/release/shiguang-vps/shiguang-vps)](https://github.com/shiguang-vps/shiguang-vps/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

<!-- screenshot: 拾光VPS 总览 Dashboard，显示订阅节点数、在线 agent 数、本月流量 -->

把 sub-store（订阅）、Nezha（探针）、Uptime Kuma（通知）三套工具收敛到一份 < 30MB 的 Go 单二进制，开箱即用。

---

## 核心特性

1. **订阅聚合**：支持 URL / 上传 YAML / 手动添加三种方式，解析 12 种协议 URI（vmess / vless / ss / ssr / trojan / hysteria / hysteria2 / tuic / wireguard / anytls / socks5 / naive）
2. **算子流水线**：可视化拖拽编排 filter / sort / dedupe / regex-rename / map / output 六类算子，YAML 可导出 git 化管理
3. **sub-store 兼容**：`/download/:name?token=<token>` 兼容路由，原客户端（mihomo / Clash Verge Rev）无需改配置即可迁移
4. **探针监控**：自写轻量 Go agent（< 10MB / 内存稳态 < 30MB），上报 CPU / MEM / Disk / NetIO / Load / 连接数
5. **Nezha 兼容**：原 Nezha agent 只改 server 地址即可接入，零迁移成本
6. **10 渠道通知**：Telegram / Discord / Slack / Email / Bark / Gotify / Webhook / Server酱 / PushDeer / IFTTT
7. **Telegram Bot 双向交互**：`/nodes`、`/refresh <sub>`、`/traffic` 等命令在手机运维
8. **静默模式**：默认开启，所有路径均返回 404，登录页面隐藏在 32 位随机前缀下，防爆扫
9. **OTA 自更新**：面板内一键升级，SHA-256 校验 + 优雅重启，无需 SSH
10. **单二进制部署**：Go + SQLite + React 19 前端全部打包，< 30MB，无外部依赖

---

## 快速开始

### Docker（推荐）

```bash
docker run -d --name shiguang \
  -p 8080:8080 \
  -v /data/shiguang:/data \
  ghcr.io/shiguang-vps/shiguang-vps:latest
```

启动后查看日志获取初始密码和登录 URL：

```bash
docker logs shiguang 2>&1 | grep -E "ADMIN|Login URL"
```

### 一键安装脚本

```bash
curl -fsSL https://get.shiguang-vps.example/install.sh | bash
```

### 手动二进制

从 [Releases](https://github.com/shiguang-vps/shiguang-vps/releases) 下载对应平台二进制，直接运行：

```bash
chmod +x shiguang-vps-linux-amd64
./shiguang-vps-linux-amd64
```

---

## 文档

| 文档 | 说明 |
|------|------|
| [快速开始](./docs/user/quickstart.md) | 三种部署方式 + 首次配置 |
| [从 sub-store 迁移](./docs/user/migration-from-substore.md) | sub-store 用户零成本切换指引 |
| [从 Nezha 迁移](./docs/user/migration-from-nezha.md) | Nezha agent 直接接入拾光VPS |
| [通知渠道配置](./docs/user/notification-setup.md) | 10 个通知渠道逐一配置说明 |
| [流水线常用模式](./docs/user/pipeline-cookbook.md) | 算子流水线 YAML 示例与配方 |
| [常见问题排查](./docs/user/troubleshooting.md) | 静默模式 / 密码找回 / agent 连接等问题 |

---

<!-- screenshot: 算子流水线编辑器截图，左侧算子库 + 中间画布 + 右侧参数面板 -->
<!-- screenshot: 探针 agent 监控页，多服务器 CPU/内存/流量趋势图 -->
<!-- screenshot: 通知渠道配置页，Telegram/Discord/Bark 等 10 个渠道 -->

---

## 系统要求

- **内存**：hub 稳态 < 50MB；agent 稳态 < 30MB；推荐最低 256MB RAM
- **操作系统**：Linux（amd64 / arm64）、macOS（amd64 / arm64）、Windows（amd64）
- **浏览器**：Chrome 110+ / Edge 110+ / Firefox 115+ / Safari 16+

---

## License

[MIT](./LICENSE)
