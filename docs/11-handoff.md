# 拾光VPS Handoff Document

生成日期：2026-05-20
对应版本：1.0.0-rc.1

## Handoff: 项目当前状态

拾光VPS（shiguang-vps）是 Go 1.26 + React 19 + SQLite 的自托管 Web 面板，把 Clash 订阅聚合、多探针流量观测、节点管理、规则脚本扩展、10 渠道通知整合到单二进制。dev-team 流水线 35 个开发任务（T-0 ~ T-34）全部完成；阶段 8 Code Review round1 修复完成；最终验收：go test 583 passed / 0 failed (-race)、vitest 64 passed、Playwright 5 e2e specs、go build + pnpm build 全绿、视觉规范违规归零。

## Handoff: 关键决策

- `docs/adr/0001-tech-stack.md` — 后端 Go 1.26 + 标准库 + modernc.org/sqlite；前端 React 19 + TanStack + Tailwind v4 + Radix
- `docs/adr/0002-user-model.md` — 多用户 admin/user 二分；不上完整 RBAC
- `docs/adr/0003-agent-strategy.md` — 自写轻量 agent + 兼容 Nezha agent v2 协议（迁移诱饵）
- `docs/adr/0004-pipeline-design.md` — 算子流水线 6 算子 + YAML 双向；v1 全后端执行
- `docs/adr/0005-notification-architecture.md` — NotificationChannel 接口 + 插件化 + SSE 推送
- `docs/adr/0006-silent-mode-retention.md` — 保留妙妙屋"未授权返 404"独家工程亮点（32hex prefix + nginx mimic）
- `docs/adr/0007-i18n-strategy.md` — react-i18next + 4 套 locale（zh-CN/en/ja/ko）
- `docs/adr/0008-deployment-model.md` — 单二进制 + Docker distroless + 一键脚本 + systemd unit

## Handoff: 领域术语

完整术语见 `docs/CONTEXT.md`：25+ Definitions / 8 Avoid / 关系图 / 5 Flagged ambiguities。核心术语：订阅（Subscription）/ 节点（Node/Proxy）/ 算子（Operator）/ 探针 agent / hub / 流水线（Pipeline）/ 规则提供者（Rule Provider）/ 静默模式（Silent Mode）/ share_token / kind=native|nezha_compat。

## Handoff: 已完成

阶段 1 → `docs/00-coding-standards.md`、`docs/01-requirements.md`、`docs/CONTEXT.md`、`docs/adr/0001-0008-*.md`
阶段 3 → `docs/02-ui-design.md`、`docs/03-architecture.md`
阶段 4 → `docs/04-api-contract.md`、`internal/types/api.go`、`internal/types/wsproto.go`、`web/src/types/api.ts`、`web/src/types/wsproto.ts`、`pkg/agentlib/protocol.go`
阶段 5 → `docs/05-tech-lead-plan.md`、`docs/_scaffold-check.md`、git tag `scaffold-base`
阶段 7 → 35 任务全部完成（T-0 ~ T-34），40 个 commit，主代码量 ~32k LOC（不含测试）
阶段 8 → `docs/06-review-backend.md` / `docs/06-review-frontend.md`、`docs/06-review-*-round1.md` 备份
阶段 9 round1 → commit `6bd983e`（后端 7 修复）+ `f97867c`（前端 13 修复）
阶段 10 → `deploy/Dockerfile` / `deploy/Dockerfile.agent` / `deploy/docker-compose*.yml` / `deploy/install*.sh` / `deploy/*.service` / `.github/workflows/*.yml`、`README.md`、`docs/user/*.md`（6 个文档 1613 行）
阶段 11 → `docs/11-handoff.md`、`CHANGELOG.md`、`docs/release-notes-1.0.0-rc.1.md`

## Handoff: 未完成 / 已知问题

### 视觉/工程债（不阻塞 v1.0 RC）
- `docs/_lint-violations.md` 中 23 个 size violations，全部 < 1.5× 阈值（Go traffic_handler.go 750 行 / TS template-editor.tsx 450 行等）—— 列入 v1.1 拆分
- `internal/types/api.go` 1100 行单文件超 2× 阈值，建议按 module 拆 `internal/types/api/*.go`
- 5 个 Go 函数超 80 行（backup.go Restore / ota applier Apply / scriptengine engine Run / node_repo UpsertBatch / agent_ws_handler ServeHTTP）

### 建议改进（review round1 标的 [bug]，未阻塞但建议 v1.0 GA 前补）
- `internal/handler/agent_handler.go:466 buildInstallCommand` 用户输入 name 未做 shell 转义
- `internal/notify/tg_bot.go:282 /start` 缺一次性 invite code 防绑定窗口期被抢
- `internal/handler/substore_compat_handler.go:64 notFound` 与其他 silent_mode 路径用了 Go 默认 404 body，建议统一调 Mimic404
- `internal/handler/install_script_handler.go:194 deriveHubURL` 信任 X-Forwarded-*，未要求 Trust-Proxy=true

### Flagged ambiguities（docs/CONTEXT.md）
- agent ↔ hub WebSocket vs gRPC 未来切换策略
- 流水线 YAML schema v2 兼容方案
- 静默模式 32hex 前缀自动轮换策略（v1 手动）

### P2 路线图（不在 v1.0 范围）
- M-P2-1 Web Terminal（agent 端 shell + xterm.js）
- M-P2-2 Docker 容器级监控
- M-P2-3 公开状态页（SLA）
- M-P2-4 WebDAV 配置同步
- M-P2-5 OAuth/OIDC 企业 SSO
- M-P2-6 主题市场 / 可换皮

## Handoff: 下一步建议

### 1. 打 1.0.0-rc.1 tag 并触发 release workflow
- 在哪：项目根目录
- 命令：`git tag v1.0.0-rc.1 && git push origin v1.0.0-rc.1`
- 预期产出：`.github/workflows/release.yml` 自动构建 5 平台二进制 + Docker 多架构镜像（amd64+arm64）+ 上传到 GitHub Release + 推送 GHCR
- 验收：`docs/release-notes-1.0.0-rc.1.md` 中所有 highlight 在 Release 页可见

### 2. 真实环境验证：跑 E2E 测试 + 部署演练
- 在哪：CI（push 后自动） + 一台 VPS（手动演练）
- 命令：
  - CI：自动触发 `.github/workflows/e2e.yml`（hub + vite dev + playwright）
  - VPS：`curl -fsSL https://get.shiguang-vps.example/install.sh | bash`（参照 deploy/install.sh）
- 预期产出：5 个 E2E specs 全过；VPS 部署后能看到登录页 + admin 密码（journalctl）+ 全功能可用
- 验收：playwright trace 无 fail；VPS 上 OTA 自更新 round-trip 成功

### 3. 补 round2 修复（如有遗留问题）
- 在哪：`docs/06-review-backend-round1.md` + `docs/06-review-frontend-round1.md` 的"建议改进"小节
- 命令：派 dev Agent，限定只动 P1 list（约 6-8 项）
- 预期产出：P1 全部清零，P2 标注列入 v1.1 backlog
- 验收：再跑一遍 review，输出"✅ 通过"

### 4. 用户文档配截图
- 在哪：`docs/user/quickstart.md` 等 6 个文档中的 `<!-- screenshot: ... -->` 占位
- 命令：跑真实环境后逐页截图，放 `docs/user/screenshots/`
- 预期产出：每个 quickstart / migration 页面至少 2-3 张关键截图
- 验收：可读性测试（一个新用户能照着文档完成首次部署）
