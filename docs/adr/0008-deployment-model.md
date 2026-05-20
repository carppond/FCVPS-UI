# ADR 0008: 部署模型 —— 单二进制 + Docker + 一键脚本

## Context

- 调研报告（docs/_research-competitors.md 第三节"部署"统计）：10/10 项目都提供 Docker 镜像；6 个 Go 项目都做单二进制；8 个有一键脚本。这三种是该垂类用户的"标准入口"。
- 目标用户群体：
  - 个人爱好者：偏好"一行命令装好"（一键脚本）。
  - 小机场主：习惯 Docker 部署。
  - 极客 / 离线环境用户：需要单二进制。
- 妙妙屋当前提供 Docker + 一键脚本，单二进制偶有缺位；本项目需要全覆盖。
- 单二进制的实现关键：前端 embed 到 Go 二进制（`embed.FS` + Go 1.16+ 特性），SQLite 用纯 Go 实现（modernc.org/sqlite，见 ADR 0001）避免 cgo。

## Decision

**三入口部署**：

1. **单二进制**：
   - 平台：Linux amd64 / Linux arm64 / macOS amd64+arm64 / Windows amd64。
   - 体积：hub < 30MB（含 embed 前端 + SQLite + 所有依赖）；agent < 10MB。
   - 启动：`./shiguang-vps`，默认监听 `:8080`；首次启动自动建库 + 输出 admin 初始密码 + 静默模式入口前缀。
   - 数据：默认 `./data/` 目录，可通过 `--data` 或 `SHIGUANG_DATA_DIR` 覆盖。
2. **Docker 镜像**：
   - 平台：linux/amd64 + linux/arm64 双架构。
   - 镜像：`ghcr.io/<org>/shiguang-vps:<tag>`（基于 scratch 或 distroless）。
   - 暴露：8080 端口；卷挂载 `/data`。
   - docker-compose.yml 模板提供。
3. **一键脚本**：
   - `curl -fsSL https://<域名>/install.sh | bash` 风格。
   - 检测系统（systemd / OpenRC / 直接进程），下载对应平台二进制到 `/opt/shiguang-vps`，生成 systemd unit，启动服务。
   - 支持卸载 (`install.sh --uninstall`)、升级 (`install.sh --upgrade`)。

**OTA 自更新**（与三入口正交）：hub 检测 GitHub Release，下载新二进制 + SHA-256 校验 + 优雅停机 + 替换 + 重启。OTA 流程触发前自动 wal_checkpoint，避免 SQLite WAL 文件残留（CONTEXT.md Flagged #5 已锁）。

## Consequences

**正面**：
- 三入口覆盖各类用户，门槛最低（一键脚本）和门槛最高（离线单二进制）都有覆盖。
- Docker + 单二进制双轨意味着 K8s 用户也无障碍接入（已有用户用 helm chart 包 Docker 镜像）。
- OTA 自更新对单二进制用户尤其友好（无 Docker 时 `docker pull` 流程缺失，OTA 是唯一替代）。

**负面 / 待办**：
- 多平台交叉编译需要 CI 矩阵（5+ 目标），首次设置工作量较大。
- 一键脚本的安全性需慎重：所有外链下载脚本都被批评过；需提供完整源码 + GitHub Release 校验和。
- OTA 自更新 + SQLite WAL 的边界场景需要在 Sprint 6 集中验证（覆盖：升级中断、磁盘满、权限不足等）。

**替代方案为何放弃**：
- **只做 Docker**（Beszel 路线）：放弃 30%+ 偏好原生部署的用户。
- **不做一键脚本**（Gatus 路线）：中文圈用户体验差，社区扩张缓慢。
- **依赖 cgo SQLite**：交叉编译复杂，arm64 / musl libc 平台容易踩坑。

## Related

- ADR 0001: 技术栈（modernc.org/sqlite 纯 Go 实现是部署模型的先决条件）
- ADR 0003: agent 策略（agent 也走单二进制 + Docker 双轨）
- CONTEXT.md Flagged #5（OTA 与 WAL 文件锁的处理假设）
- 调研：docs/_research-competitors.md 第三节"部署"统计
