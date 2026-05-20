# 拾光VPS 技术架构方案

版本：v1.0
日期：2026-05-20
输入：`docs/01-requirements.md` + `docs/CONTEXT.md` + `docs/adr/0001-0008` + `docs/00-coding-standards.md` + `docs/_research-competitors.md`
范围：覆盖 hub（Go 服务端）、agent（Go 探针）、web（React 前端）三端的工程化结构、模块边界、数据模型、接口表与关键设计取舍。

本文档是 Tech Lead 阶段任务分配和开发工程师阶段动手编码的**唯一架构事实源**。任何与本文档冲突的代码必须先发 PR 改本文档。

---

## 1. 技术选型

### 1.1 后端（hub + agent，Go 1.24）

后端走"标准库优先 + 必需依赖最小化"。明确拒绝重型 Web 框架（Gin/Echo/Fiber），改用 `net/http` + 自写 mux + 中间件链；明确拒绝 ORM（GORM/ent），改走 `database/sql` + 手写 SQL（参数化），SQL 文本集中在 `internal/storage/*_repo.go`。

| 依赖 | 版本约束 | 用途 | 选型理由 |
|---|---|---|---|
| `modernc.org/sqlite` | latest | SQLite 驱动（纯 Go，无 cgo） | ADR 0001 锁定；交叉编译友好 |
| `github.com/gorilla/websocket` | v1.5+ | hub ↔ agent + 前端实时事件流 | ADR 0003 锁定；社区事实标准 |
| `gopkg.in/yaml.v3` | v3.x | YAML 解析（必须用 yaml.Node API） | 编码规范 §9 强制；保字段序/引号/注释 |
| `github.com/dop251/goja` | latest | JS 沙箱（pre_save_nodes / post_fetch） | ADR 0004 + PRD M-SCRIPT |
| `github.com/pquerna/otp` | v1.4+ | TOTP 2FA | PRD M-USER-3 |
| `golang.org/x/crypto/bcrypt` | latest | 密码 hash，cost=10 | PRD §6.3 |
| `golang.org/x/crypto/acme/autocert` | latest | 可选的内置 HTTPS（生产用反代时不启用） | 单文件部署可选 |
| `golang.org/x/sync/errgroup` | latest | 并行 fetch / 并行 TCPing | 编码规范 §5.1 |
| `golang.org/x/time/rate` | latest | 登录限速 token bucket | PRD §6.3 |
| `log/slog` | 标准库 | 结构化日志（json，stdout） | 编码规范 §6.3 |
| `github.com/google/uuid` | v1.6+ | 资源 ID（短链 / token / agent ID） | 单一来源 |
| `github.com/hashicorp/golang-lru/v2` | latest | session / rate / agent presence 缓存 | 编码规范 §5.3 |
| `github.com/benbjohnson/clock` | latest | 测试用 fake clock | 编码规范 §11.4 |
| `github.com/stretchr/testify` | latest | 断言库 | 默认测试栈 |
| `github.com/go-telegram-bot-api/telegram-bot-api/v5` | v5 | Telegram 双向 Bot | PRD M-NOTIFY-4 |
| `github.com/gomail/gomail` 或 `net/smtp` | 标准库优先 | SMTP 邮件渠道 | 标准库已够用 |
| `github.com/shirou/gopsutil/v4` | v4 | agent 端 CPU/MEM/Disk/Net 采集 | 跨平台、维护活跃 |
| `github.com/google/go-github/v62` | v62 | OTA 走 GitHub Release API | 官方 SDK |

**禁止依赖（红线）**：

- 任何 cgo SQLite（mattn/go-sqlite3、crawshaw 等）→ 破坏交叉编译，ADR 0001 已拒绝。
- 任何 Web 框架（Gin/Echo/Fiber/Chi 等重 Mux 也避免，标准库够用 + 自写少量中间件）。
- ORM（GORM、ent、sqlc 可选但 v1 不引入，全部 `database/sql`）。
- 任何 GPL/AGPL 依赖（编码规范 §12 红线）。

### 1.2 前端（React 19，TypeScript 5.x）

前端通过 `embed.FS` 打包到 Go 二进制；开发期独立 dev server（vite 7），生产期由 hub 用 `http.FileServer(http.FS(embedFS))` 直接服务。

`web/package.json` 的 `dependencies`（v1 锁定清单）：

```json
{
  "react": "^19.0.0",
  "react-dom": "^19.0.0",
  "@tanstack/react-router": "^1.90.0",
  "@tanstack/react-query": "^5.62.0",
  "@tanstack/react-virtual": "^3.10.0",
  "zustand": "^5.0.0",
  "react-i18next": "^15.0.0",
  "i18next": "^24.0.0",
  "i18next-browser-languagedetector": "^8.0.0",
  "react-hook-form": "^7.53.0",
  "@hookform/resolvers": "^3.9.0",
  "zod": "^3.23.0",
  "@dnd-kit/core": "^6.1.0",
  "@dnd-kit/sortable": "^8.0.0",
  "@dnd-kit/utilities": "^3.2.0",
  "@radix-ui/react-dialog": "^1.1.0",
  "@radix-ui/react-dropdown-menu": "^2.1.0",
  "@radix-ui/react-popover": "^1.1.0",
  "@radix-ui/react-select": "^2.1.0",
  "@radix-ui/react-switch": "^1.1.0",
  "@radix-ui/react-tabs": "^1.1.0",
  "@radix-ui/react-tooltip": "^1.1.0",
  "@radix-ui/react-toast": "^1.2.0",
  "@radix-ui/react-checkbox": "^1.1.0",
  "@radix-ui/react-label": "^2.1.0",
  "@radix-ui/react-slot": "^1.1.0",
  "@radix-ui/react-scroll-area": "^1.2.0",
  "@radix-ui/react-separator": "^1.1.0",
  "tailwindcss": "^4.0.0",
  "@tailwindcss/vite": "^4.0.0",
  "class-variance-authority": "^0.7.0",
  "clsx": "^2.1.0",
  "tailwind-merge": "^2.5.0",
  "lucide-react": "^0.460.0",
  "sonner": "^1.7.0",
  "@monaco-editor/react": "^4.6.0",
  "monaco-editor": "^0.52.0",
  "recharts": "^2.13.0",
  "@xterm/xterm": "^5.5.0",
  "@xterm/addon-fit": "^0.10.0",
  "@xterm/addon-web-links": "^0.11.0",
  "date-fns": "^4.1.0",
  "yaml": "^2.6.0"
}
```

`devDependencies`：

```json
{
  "typescript": "^5.7.0",
  "vite": "^7.0.0",
  "@vitejs/plugin-react": "^4.3.0",
  "@types/react": "^19.0.0",
  "@types/react-dom": "^19.0.0",
  "@types/node": "^22.0.0",
  "eslint": "^9.0.0",
  "@typescript-eslint/parser": "^8.0.0",
  "@typescript-eslint/eslint-plugin": "^8.0.0",
  "eslint-plugin-react-hooks": "^5.0.0",
  "vitest": "^2.1.0",
  "@testing-library/react": "^16.0.0",
  "@testing-library/jest-dom": "^6.6.0",
  "@testing-library/user-event": "^14.5.0",
  "jsdom": "^25.0.0",
  "@playwright/test": "^1.49.0",
  "prettier": "^3.4.0",
  "@tanstack/router-devtools": "^1.90.0",
  "@tanstack/react-query-devtools": "^5.62.0"
}
```

**为什么这些组合（与竞品对齐解读）**：

- React 19 + TanStack Router + Query：妙妙屋同款（已被验证），文件路由 + server-state 缓存让代码量降低。
- Tailwind v4 + Radix UI：与编码规范 §7.2 的 `components/ui/` 设计一致；Radix 提供无障碍 + 行为，Tailwind 管样式，CVA 管变体。
- @dnd-kit：ADR 0004 明确流水线 GUI 用 @dnd-kit；它在 React 19 下稳定，无需 polyfill。
- Monaco Editor：流水线 YAML + 脚本编辑器统一用 Monaco，可获得语法高亮 + LSP-like 校验 + diff 视图（M-PIPE-4 / M-PIPE-5 / M-SCRIPT 都用得到）。
- Recharts：流量趋势图（M-TRAFFIC-4），它的 ResponsiveContainer + 多 series 足够覆盖"按日/月切换 + 多探针多订阅源汇总"。
- xterm.js：v1 不做 Web Terminal（PRD §7.1 列入 P2），但 agent 已留接口；前端预装 xterm.js 以备 P2 直接接入。
- sonner：toast 默认实现，比手写 Radix Toast 简洁。

### 1.3 构建 / Lint / 格式化

- 后端：`gofumpt -l -w .`（比 gofmt 更严）、`golangci-lint run`（启用 `errcheck / govet / staticcheck / revive / gocyclo / gocritic / misspell / nilerr / sqlclosecheck`）、`go vet ./...`。
- 前端：`tsc --noEmit`、`eslint . --ext .ts,.tsx`、`prettier --check`。
- 跨端：`scripts/gen-types.sh`（Go types → TS 类型，工具：`tygo`），CI 强制 `git diff --exit-code`。
- `scripts/check-size.sh`：检查文件 / 函数行数上限（编码规范 §1）。
- `scripts/check-i18n.sh`：检查 4 套 locale key 集合相等 + 业务 ts/tsx 中无硬编码中日韩文。

### 1.4 测试

- 后端：`go test -race -cover ./...`，覆盖率 ≥ 50%；表驱动；`httptest.NewServer` 跑 handler 集成；`testify/require` 断言。
- 前端：`vitest` + `@testing-library/react`；核心 `lib/` ≥ 70%。
- E2E：`playwright`，headless，CI 跑登录 / 订阅 / 流水线 / 通知 / agent 五条主路径。

### 1.5 CI / CD

- GitHub Actions：
  - `lint.yml`：gofumpt + golangci-lint + eslint + prettier + i18n key 检查 + size 检查。
  - `test.yml`：`go test` + `vitest` + 覆盖率上传 codecov。
  - `e2e.yml`：playwright headless（仅 PR + main）。
  - `release.yml`：tag 触发，矩阵 build 5 平台二进制（linux amd64/arm64 + darwin amd64/arm64 + windows amd64） + Docker 多架构（amd64/arm64） + GitHub Release + SHA-256 校验和。
- 包管理：Go modules（go.mod）；前端 `pnpm`（lockfile in repo），CI 用 `pnpm install --frozen-lockfile`。

---

## 2. 项目结构

按编码规范 §7 的预期布局展开到文件级，所有"代表性文件"全部列出（用于阶段 5 任务分配）。

```text
shiguang-vps/
├── cmd/
│   ├── server/
│   │   └── main.go                       # ★共享 hub 入口：读 config → 启动 mux → 启动 agent hub → 启动调度器
│   └── agent/
│       ├── main.go                       # ★共享 agent 入口：连接 hub → 上报指标 → 处理下发命令
│       └── internal/                     # agent 私有逻辑（hub 不可见）
│           ├── collector/
│           │   ├── cpu.go
│           │   ├── memory.go
│           │   ├── disk.go
│           │   ├── net.go
│           │   ├── load.go
│           │   ├── conn.go               # TCP/UDP 连接数
│           │   └── uptime.go
│           ├── transport/
│           │   ├── client.go             # WebSocket 客户端 + 重连
│           │   ├── heartbeat.go          # 30s 心跳，可配 5-300s
│           │   └── command.go            # 收 hub 下发的命令
│           └── nezha_emitter/
│               └── emit.go               # 把指标按 Nezha v2 格式输出（兼容反向场景）
├── internal/
│   ├── auth/                             # ★共享：登录、2FA、token、session、暴破防护
│   │   ├── manager.go                    # 登录 / 改密 / 删用户 / 重置密码
│   │   ├── token_store.go                # session 表 + LRU 缓存
│   │   ├── totp.go                       # pquerna/otp 封装；recovery code
│   │   ├── recovery_codes.go             # 8 位 hex × N，sha256 存库
│   │   ├── middleware.go                 # Required / RequireAdmin / RequirePending2FA
│   │   ├── brute.go                      # IP + 账号双维度限速
│   │   ├── password.go                   # bcrypt cost=10 封装
│   │   └── errors.go                     # ErrUserNotFound / ErrInvalidCredentials / ...
│   ├── handler/                          # HTTP handler，每个业务域一个文件
│   │   ├── router.go                     # ★共享：mux 注册表 + 中间件链装配
│   │   ├── middleware/
│   │   │   ├── recover.go                # ★共享：panic 兜底
│   │   │   ├── cors.go                   # 默认禁用，可由 admin 开启
│   │   │   ├── log.go                    # 请求日志（带 trace_id）
│   │   │   ├── silent_mode.go            # ★共享：静默模式入口（前缀校验 + 404 化）
│   │   │   └── ratelimit.go              # 全局 + 路由级限速
│   │   ├── auth_handler.go               # /api/auth/*
│   │   ├── user_handler.go               # /api/me/* + /api/admin/users/*
│   │   ├── subscription_handler.go       # /api/subscriptions/*
│   │   ├── node_handler.go               # /api/nodes/*
│   │   ├── pipeline_handler.go           # ★差异化：/api/pipelines/*
│   │   ├── rule_handler.go               # /api/rules/*
│   │   ├── script_handler.go             # /api/scripts/*
│   │   ├── agent_handler.go              # /api/agents/* + /api/agent/ws
│   │   ├── nezha_handler.go              # /api/v1/nezha/*（兼容路）
│   │   ├── traffic_handler.go            # /api/traffic/*
│   │   ├── notify_handler.go             # ★差异化：/api/notify/*
│   │   ├── tcping_handler.go             # /api/nodes/tcping
│   │   ├── shortlink_handler.go          # /s/:code（公开）+ /api/shortlinks/*
│   │   ├── substore_compat_handler.go    # /download/:name（公开，token 守卫）
│   │   ├── ota_handler.go                # /api/ota/*
│   │   ├── audit_handler.go              # /api/admin/audit/*
│   │   ├── healthz_handler.go            # /healthz
│   │   ├── stream_handler.go             # /api/notify/stream（前端 SSE/WS）
│   │   └── settings_handler.go           # /api/admin/settings/*
│   ├── storage/                          # SQLite repo 层，唯一直接接触 DB
│   │   ├── db.go                         # ★共享：连接 / WAL / busy_timeout / ensureColumn
│   │   ├── migrate.go                    # ★共享：内嵌 SQL + ensureColumn 双轨
│   │   ├── tx.go                         # ★共享：事务封装 + retry on SQLITE_BUSY
│   │   ├── user_repo.go
│   │   ├── session_repo.go
│   │   ├── subscription_repo.go
│   │   ├── node_repo.go
│   │   ├── pipeline_repo.go
│   │   ├── pipeline_binding_repo.go
│   │   ├── rule_repo.go
│   │   ├── script_repo.go
│   │   ├── agent_repo.go
│   │   ├── agent_record_repo.go
│   │   ├── traffic_repo.go
│   │   ├── notify_channel_repo.go
│   │   ├── notify_event_repo.go
│   │   ├── shortlink_repo.go
│   │   ├── audit_repo.go
│   │   └── settings_repo.go
│   ├── substore/                         # 多协议解析 + Clash producer + sub-store API 兼容
│   │   ├── uri_parser.go                 # 入口：根据 scheme 路由到子 parser
│   │   ├── parser_vmess.go
│   │   ├── parser_vless.go
│   │   ├── parser_ss.go
│   │   ├── parser_ssr.go
│   │   ├── parser_trojan.go
│   │   ├── parser_hysteria.go
│   │   ├── parser_hysteria2.go
│   │   ├── parser_tuic.go
│   │   ├── parser_wireguard.go
│   │   ├── parser_anytls.go
│   │   ├── parser_socks5.go
│   │   ├── parser_naive.go
│   │   ├── acl4ssr.go                    # ini / list 兼容
│   │   ├── clash_producer.go             # Node → Clash YAML
│   │   └── substore_compat.go            # sub-store HTTP 路由的 service 层
│   ├── pipeline/                         # ★差异化 #1：算子流水线引擎
│   │   ├── engine.go                     # Runner：ops 顺序执行 + diff 抓取
│   │   ├── operator.go                   # Operator 接口 + 注册表
│   │   ├── op_filter.go
│   │   ├── op_map.go
│   │   ├── op_sort.go
│   │   ├── op_dedupe.go
│   │   ├── op_regex_rename.go
│   │   ├── op_output.go
│   │   ├── yaml_codec.go                 # YAML ↔ AST 双向（apiVersion: shiguang/v1）
│   │   ├── ast.go                        # AST 结构（go struct + json tag）
│   │   ├── debug.go                      # 调试预览：每算子前后 diff
│   │   └── validate.go                   # YAML schema 校验
│   ├── notify/                           # ★差异化 #2：通知通道插件
│   │   ├── channel.go                    # NotificationChannel 接口 + 注册表
│   │   ├── manager.go                    # 事件总线 + 去抖 + 路由
│   │   ├── event.go                      # 事件类型 enum + payload schema
│   │   ├── template.go                   # Go template 渲染封装
│   │   ├── dedupe.go                     # 5 分钟去抖（基于 sha1(event_type+key)）
│   │   ├── ch_telegram.go
│   │   ├── ch_discord.go
│   │   ├── ch_slack.go
│   │   ├── ch_email.go
│   │   ├── ch_bark.go
│   │   ├── ch_gotify.go
│   │   ├── ch_webhook.go
│   │   ├── ch_serverchan.go
│   │   ├── ch_pushdeer.go
│   │   ├── ch_ifttt.go
│   │   └── tg_bot.go                     # Telegram 双向 Bot（inline keyboard）
│   ├── scriptengine/                     # goja 沙箱
│   │   ├── engine.go                     # Runtime 池 + 5s 超时
│   │   ├── hooks.go                      # pre_save_nodes / post_fetch 入口
│   │   ├── sandbox.go                    # 禁 fs / net / require
│   │   └── runner.go                     # vm.RunString + Interrupt
│   ├── agent/                            # hub 端 agent 管理
│   │   ├── hub.go                        # WebSocket hub：connect / disconnect / broadcast
│   │   ├── client.go                     # 单 agent 连接对象（recv goroutine + send queue）
│   │   ├── protocol.go                   # 协议 schema（types/agentproto/ 镜像）
│   │   ├── presence.go                   # LRU 缓存"在线 agent + 最后心跳"
│   │   └── command.go                    # 下发命令（refresh / restart / update）
│   ├── nezha/                            # ★Nezha 兼容层（独立包）
│   │   ├── compat.go                     # 入口：Nezha 字段 ↔ 自有 AgentRecord
│   │   ├── proto_v2.go                   # v2 心跳字段定义
│   │   ├── proto_v1.go                   # v1 心跳字段（最小集，可关）
│   │   ├── adapter.go                    # nezha.Adapter 接口
│   │   └── handler.go                    # /api/v1/nezha/heartbeat 等
│   ├── traffic/                          # 流量聚合 svc
│   │   ├── aggregator.go                 # 日聚合（cron 00:00）
│   │   ├── monthly_reset.go              # 月度重置（按用户配置）
│   │   ├── threshold.go                  # 阈值告警（80% / 90% / 100%）
│   │   └── tcping.go                     # 并发 TCPing
│   ├── ratelimit/                        # 通用限速器（token bucket + LRU）
│   │   └── limiter.go
│   ├── ota/                              # 自更新
│   │   ├── checker.go                    # GitHub Release poll
│   │   ├── downloader.go                 # 下载 + SHA-256
│   │   ├── applier.go                    # 优雅停机 + 替换 + 重启
│   │   └── wal_checkpoint.go             # OTA 前 PRAGMA wal_checkpoint
│   ├── audit/
│   │   ├── logger.go                     # 写入 audit_logs
│   │   └── middleware.go                 # 在 handler 链上自动 audit
│   ├── i18n/                             # 后端 i18n（仅错误码 + 邮件模板）
│   │   ├── catalog.go
│   │   ├── locales/
│   │   │   ├── zh-CN.json
│   │   │   ├── en.json
│   │   │   ├── ja.json
│   │   │   └── ko.json
│   │   └── email_templates/
│   │       └── ...
│   ├── logger/                           # ★共享：slog json 封装 + 轮转
│   │   ├── logger.go
│   │   └── rotate.go                     # 100MB / 7 天
│   ├── config/                           # ★共享：env + 文件 + flag
│   │   ├── config.go
│   │   └── defaults.go
│   ├── shortlink/                        # 短链 svc
│   │   └── service.go
│   ├── util/                             # ★共享：纯函数工具
│   │   ├── yaml.go                       # yaml.Node helpers（ReorderProxyNode 等）
│   │   ├── id.go                         # uuid / 32hex 生成
│   │   ├── strslice.go
│   │   ├── crypto.go                     # sha256 / random bytes
│   │   ├── http.go                       # JSON 响应 / 错误响应辅助
│   │   ├── pagination.go
│   │   └── time.go                       # fake clock 友好的封装
│   └── types/                            # ★共享：API 契约 + 内部协议
│       ├── api/                          # 由 tygo 同步到 src/types/api/
│       │   ├── auth.go
│       │   ├── user.go
│       │   ├── subscription.go
│       │   ├── node.go
│       │   ├── pipeline.go
│       │   ├── rule.go
│       │   ├── script.go
│       │   ├── agent.go
│       │   ├── traffic.go
│       │   ├── notify.go
│       │   ├── shortlink.go
│       │   ├── ota.go
│       │   ├── audit.go
│       │   ├── settings.go
│       │   └── errcode.go                # 错误码枚举
│       └── agentproto/                   # agent ↔ hub 协议（向后兼容硬约束 §10）
│           ├── envelope.go               # 公共信封：type / id / payload
│           ├── heartbeat.go
│           ├── record.go
│           ├── command.go
│           └── version.go
├── pkg/
│   └── agentlib/                         # ★hub 与 agent 共享的协议库（被 cmd/agent 直接 import）
│       ├── proto.go                      # types/agentproto 的导出镜像
│       └── ws.go                         # WebSocket 编解码 helper
├── migrations/                           # 显式 SQL（仅大版本断点用，常态走 ensureColumn）
│   ├── 0001_initial.sql
│   ├── 0002_pipeline.sql
│   └── README.md
├── deploy/
│   ├── Dockerfile                        # 多阶段：build → distroless
│   ├── Dockerfile.agent
│   ├── docker-compose.yml
│   ├── docker-compose.agent.yml
│   ├── install.sh                        # 一键脚本（hub）
│   ├── install-agent.sh                  # 一键脚本（agent）
│   ├── shiguang-vps.service              # systemd unit
│   └── nezha-migration.md                # Nezha → 拾光VPS 迁移指引
├── web/                                  # 前端 SPA 源码
│   ├── src/
│   │   ├── routes/                       # TanStack Router 文件路由
│   │   │   ├── __root.tsx
│   │   │   ├── _public/
│   │   │   │   ├── login.tsx
│   │   │   │   ├── totp.tsx
│   │   │   │   └── not-found.tsx
│   │   │   └── _authed/
│   │   │       ├── route.tsx             # 鉴权守卫
│   │   │       ├── index.tsx             # Dashboard
│   │   │       ├── subscriptions.tsx
│   │   │       ├── subscriptions.$id.tsx
│   │   │       ├── nodes.tsx
│   │   │       ├── pipelines.tsx
│   │   │       ├── pipelines.$id.tsx     # ★差异化：编辑器
│   │   │       ├── rules.tsx
│   │   │       ├── scripts.tsx
│   │   │       ├── agents.tsx
│   │   │       ├── agents.$id.tsx
│   │   │       ├── traffic.tsx
│   │   │       ├── notifications.tsx     # ★差异化：通道列表
│   │   │       ├── notifications.$id.tsx
│   │   │       ├── shortlinks.tsx
│   │   │       ├── settings.tsx
│   │   │       └── admin/
│   │   │           ├── users.tsx
│   │   │           ├── audit.tsx
│   │   │           └── ota.tsx
│   │   ├── components/
│   │   │   ├── ui/                       # ★共享原子组件（Radix + Tailwind 封装）
│   │   │   │   ├── button.tsx
│   │   │   │   ├── input.tsx
│   │   │   │   ├── dialog.tsx
│   │   │   │   ├── dropdown-menu.tsx
│   │   │   │   ├── select.tsx
│   │   │   │   ├── switch.tsx
│   │   │   │   ├── tabs.tsx
│   │   │   │   ├── tooltip.tsx
│   │   │   │   ├── toast.tsx
│   │   │   │   ├── checkbox.tsx
│   │   │   │   ├── label.tsx
│   │   │   │   ├── separator.tsx
│   │   │   │   ├── scroll-area.tsx
│   │   │   │   ├── popover.tsx
│   │   │   │   ├── badge.tsx
│   │   │   │   ├── card.tsx
│   │   │   │   ├── table.tsx
│   │   │   │   └── form.tsx
│   │   │   ├── layout/
│   │   │   │   ├── app-shell.tsx
│   │   │   │   ├── sidebar.tsx
│   │   │   │   ├── topbar.tsx
│   │   │   │   └── lang-switch.tsx
│   │   │   ├── auth/
│   │   │   │   ├── login-form.tsx
│   │   │   │   ├── totp-form.tsx
│   │   │   │   └── recovery-codes-dialog.tsx
│   │   │   ├── subscription/
│   │   │   │   ├── sub-list.tsx
│   │   │   │   ├── sub-form.tsx
│   │   │   │   ├── sub-upload.tsx
│   │   │   │   └── sub-tag-input.tsx
│   │   │   ├── node/
│   │   │   │   ├── node-table.tsx
│   │   │   │   ├── node-detail-dialog.tsx
│   │   │   │   ├── tcping-button.tsx
│   │   │   │   └── chain-proxy-picker.tsx
│   │   │   ├── pipeline/                 # ★差异化：编辑器
│   │   │   │   ├── editor.tsx            # 总入口：左库 / 中画布 / 右参数
│   │   │   │   ├── operator-library.tsx
│   │   │   │   ├── canvas.tsx            # @dnd-kit Sortable
│   │   │   │   ├── operator-node.tsx
│   │   │   │   ├── param-panel.tsx
│   │   │   │   ├── yaml-pane.tsx         # Monaco
│   │   │   │   ├── preview-pane.tsx      # 每算子 diff
│   │   │   │   └── sync-hook.ts          # GUI ↔ YAML 双向同步
│   │   │   ├── rule/
│   │   │   │   ├── rule-list.tsx
│   │   │   │   ├── rule-form.tsx
│   │   │   │   └── preview-pane.tsx      # 最终 Clash yaml 渲染
│   │   │   ├── script/
│   │   │   │   ├── script-list.tsx
│   │   │   │   └── script-editor.tsx     # Monaco
│   │   │   ├── agent/
│   │   │   │   ├── agent-list.tsx
│   │   │   │   ├── agent-card.tsx
│   │   │   │   └── metric-chart.tsx      # Recharts
│   │   │   ├── traffic/
│   │   │   │   ├── traffic-chart.tsx     # Recharts 切换日 / 月
│   │   │   │   └── threshold-config.tsx
│   │   │   ├── notify/                   # ★差异化
│   │   │   │   ├── channel-list.tsx
│   │   │   │   ├── channel-form.tsx
│   │   │   │   ├── channel-test-button.tsx
│   │   │   │   ├── event-subscriptions.tsx
│   │   │   │   └── template-editor.tsx
│   │   │   ├── shortlink/
│   │   │   │   └── shortlink-list.tsx
│   │   │   ├── ota/
│   │   │   │   └── ota-dialog.tsx
│   │   │   └── admin/
│   │   │       ├── user-table.tsx
│   │   │       ├── audit-table.tsx
│   │   │       └── settings-form.tsx
│   │   ├── hooks/                        # ★共享：通用 hook
│   │   │   ├── use-debounce.ts
│   │   │   ├── use-media-query.ts
│   │   │   ├── use-local-storage.ts
│   │   │   ├── use-event-stream.ts       # SSE/WS 实时订阅
│   │   │   └── use-pagination.ts
│   │   ├── lib/                          # ★共享：纯工具
│   │   │   ├── cn.ts                     # clsx + tailwind-merge
│   │   │   ├── api-client.ts             # fetch 封装 + 错误码 → i18n
│   │   │   ├── query-keys.ts             # TanStack Query keys
│   │   │   ├── format.ts                 # 流量 / 时间 / 数字（按 locale）
│   │   │   ├── storage.ts                # localStorage 封装
│   │   │   └── silent-prefix.ts          # 静默模式前缀注入到所有 URL
│   │   ├── stores/                       # Zustand
│   │   │   ├── auth-store.ts             # 用户信息 + token
│   │   │   ├── ui-store.ts               # 主题 / 折叠状态
│   │   │   └── pipeline-store.ts         # 编辑器临时状态（拖拽中、未保存）
│   │   ├── api/                          # 按业务域分文件，全部基于 lib/api-client
│   │   │   ├── auth.ts
│   │   │   ├── user.ts
│   │   │   ├── subscription.ts
│   │   │   ├── node.ts
│   │   │   ├── pipeline.ts
│   │   │   ├── rule.ts
│   │   │   ├── script.ts
│   │   │   ├── agent.ts
│   │   │   ├── traffic.ts
│   │   │   ├── notify.ts
│   │   │   ├── shortlink.ts
│   │   │   ├── ota.ts
│   │   │   └── settings.ts
│   │   ├── locales/                      # 编码规范 §8.2 命名空间
│   │   │   ├── zh-CN/
│   │   │   │   ├── common.json
│   │   │   │   ├── auth.json
│   │   │   │   ├── subscription.json
│   │   │   │   ├── pipeline.json
│   │   │   │   ├── node.json
│   │   │   │   ├── rule.json
│   │   │   │   ├── script.json
│   │   │   │   ├── agent.json
│   │   │   │   ├── traffic.json
│   │   │   │   ├── notify.json
│   │   │   │   └── errors.json
│   │   │   ├── en/...
│   │   │   ├── ja/...
│   │   │   └── ko/...
│   │   ├── types/
│   │   │   ├── api/                      # 由 scripts/gen-types.sh 生成；DO NOT EDIT
│   │   │   │   └── ... (镜像 internal/types/api/)
│   │   │   └── view/                     # 前端独有视图类型
│   │   ├── styles/
│   │   │   ├── globals.css               # Tailwind v4 入口
│   │   │   └── monaco.css
│   │   ├── main.tsx
│   │   └── i18n.ts                       # i18next 初始化 + detector
│   ├── public/                           # 静态资源（favicon 等，最终被 embed）
│   ├── index.html
│   ├── package.json
│   ├── pnpm-lock.yaml
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── eslint.config.js
│   └── playwright.config.ts
├── scripts/
│   ├── gen-types.sh                      # Go struct → TS（tygo）
│   ├── check-size.sh                     # 文件 / 函数行数
│   ├── check-i18n.sh                     # 4 套 locale key 对齐 + 无硬编码 CJK
│   ├── build-release.sh                  # 多平台构建
│   └── dev.sh                            # 启动 hub + web dev server
├── .github/
│   └── workflows/
│       ├── lint.yml
│       ├── test.yml
│       ├── e2e.yml
│       └── release.yml
├── docs/
│   ├── 00-coding-standards.md
│   ├── 01-requirements.md
│   ├── 02-ui-design.md                   # 由 UI 阶段产出
│   ├── 03-architecture.md                # 本文档
│   ├── 04-api-contract.md                # 由 API 契约阶段产出
│   ├── CONTEXT.md
│   ├── _research-competitors.md
│   └── adr/
│       └── 0001-0008-*.md
├── go.mod
├── go.sum
├── .golangci.yml
├── .gitignore
├── LICENSE
└── README.md
```

**共享文件 vs 模块文件标注（任务分配用）**：

- **共享基础设施（H-INFRA）**：所有 `★共享` 标注的文件 + `internal/types/`、`internal/util/`、`internal/auth/`、`internal/logger/`、`internal/config/`、`internal/storage/{db,migrate,tx}.go`、`internal/handler/{router,middleware/*}.go`、`cmd/server/main.go`、`cmd/agent/main.go`、`pkg/agentlib/`、`web/src/lib/`、`web/src/components/ui/`、`web/src/components/layout/`、`web/src/main.tsx`、`web/src/i18n.ts`。这些必须在 Sprint 1 早期由 1-2 个人统一拉起，其他模块 fork 它们的接口往下开发。
- **模块文件**：每个 `internal/<module>/`、对应的 `internal/handler/<module>_handler.go`、`internal/storage/<module>_repo.go`、`web/src/components/<module>/`、`web/src/routes/.../<module>.tsx`、`web/src/api/<module>.ts`、`web/src/locales/*/<module>.json`。每个业务模块由 1 个开发独立拉通。

---

## 3. 模块划分

下表给出 12 个模块（11 个业务 + 1 个共享基础设施）；每个模块标注职责、文件路径、依赖、公开接口、数据流向。

### 3.1 H-INFRA：共享基础设施

- **职责**：登录鉴权、token / session、密码 hash、暴破防护、限速、日志、配置、错误码、纯工具、API 类型契约、HTTP mux + 中间件链、SQLite 连接 + 迁移。是所有业务模块的"地基"，不能反向依赖业务模块。
- **文件**：
  - `cmd/server/main.go`、`cmd/agent/main.go`
  - `internal/auth/{manager,token_store,totp,recovery_codes,middleware,brute,password,errors}.go`
  - `internal/handler/router.go` + `internal/handler/middleware/{recover,cors,log,silent_mode,ratelimit}.go`
  - `internal/storage/{db,migrate,tx}.go`
  - `internal/ratelimit/limiter.go`
  - `internal/logger/{logger,rotate}.go`
  - `internal/config/{config,defaults}.go`
  - `internal/util/*.go`
  - `internal/types/api/*.go` + `internal/types/agentproto/*.go`
  - `pkg/agentlib/{proto,ws}.go`
  - `web/src/{main,i18n}.tsx/ts` + `web/src/lib/*` + `web/src/components/ui/*` + `web/src/components/layout/*` + `web/src/stores/{auth,ui}-store.ts` + `web/src/hooks/*`
- **依赖**：标准库 + 第三方依赖；不依赖任何业务模块。
- **公开接口**（关键签名）：
  ```go
  // internal/auth
  func (m *Manager) Login(ctx, username, password) (*Session, *PendingTOTP, error)
  func (m *Manager) VerifyTOTP(ctx, sessionID, code) (*Session, error)
  type Middleware interface { Required(next http.Handler) http.Handler; RequireAdmin(next http.Handler) http.Handler }

  // internal/storage
  func Open(ctx, path) (*DB, error)             // WAL + busy_timeout
  func (db *DB) EnsureColumn(table, col, ddl) error
  func (db *DB) WithTx(ctx, fn func(*sql.Tx) error) error

  // internal/handler
  func NewRouter(deps Deps) http.Handler        // 装配所有路由
  ```
- **数据流向**：HTTP 请求 → middleware chain（recover→log→cors→silent_mode→ratelimit→authn→authz）→ business handler。

### 3.2 M-USER：用户与权限模块

- **PRD 对应**：M-USER-1..10。
- **职责**：admin / user 二分；密码登录；TOTP 2FA；备份码；用户 CRUD；自助改密；session 管理；登录限速 / 暴破防护。
- **文件**：
  - `internal/handler/{auth_handler,user_handler}.go`
  - `internal/storage/{user_repo,session_repo}.go`
  - `web/src/routes/_public/{login,totp,not-found}.tsx`、`_authed/admin/users.tsx`
  - `web/src/components/auth/*` + `web/src/components/admin/user-table.tsx`
  - `web/src/api/{auth,user}.ts` + `web/src/locales/*/auth.json`
- **依赖**：H-INFRA（auth、storage、types）。
- **公开接口**：
  ```go
  // internal/storage
  type UserRepo interface {
      Create(ctx, u *User) error
      ByID(ctx, id string) (*User, error)
      ByUsername(ctx, name string) (*User, error)
      Update(ctx, u *User) error
      Delete(ctx, id string) error
      List(ctx, page Pagination) ([]*User, int, error)
  }
  ```
- **数据流向**：登录 POST → AuthHandler → AuthManager.Login → UserRepo.ByUsername + bcrypt 校验 → SessionRepo.Create → 返回 cookie。

### 3.3 M-SUB：订阅管理模块

- **PRD 对应**：M-SUB-1..9。
- **职责**：URL/上传/手动三种来源；12 种协议 URI 解析；ACL4SSR；sub-store API 兼容；自动同步周期；同步失败联动通知。
- **文件**：
  - `internal/handler/{subscription_handler,substore_compat_handler}.go`
  - `internal/storage/{subscription_repo,node_repo}.go`（与 M-NODE 共享 node_repo）
  - `internal/substore/*.go`
  - `web/src/routes/_authed/subscriptions{,.$id}.tsx`
  - `web/src/components/subscription/*`
  - `web/src/api/subscription.ts` + `web/src/locales/*/subscription.json`
- **依赖**：H-INFRA、M-NODE（写节点）、M-NOTIFY（同步失败事件）、M-SCRIPT（post_fetch hook）。
- **公开接口**：
  ```go
  type SubscriptionService interface {
      Create(ctx, in CreateInput) (*Subscription, error)
      Sync(ctx, subID string) (*SyncResult, error)   // 拉取 + parse + 入库
      Render(ctx, subID string, format string) ([]byte, error)  // 输出 Clash YAML
  }

  // internal/substore
  type Parser interface { Parse(uri string) (*Node, error) }
  var Registry = map[string]Parser{ "vmess": &vmessParser{}, ... }
  ```
- **数据流向**：URL → SubscriptionService.Sync → fetch raw → post_fetch hook（M-SCRIPT）→ uri_parser → []Node → pre_save_nodes hook → NodeRepo.UpsertBatch。

### 3.4 M-PIPE：算子流水线模块（★差异化 #1）

- **PRD 对应**：M-PIPE-1..7。
- **职责**：6 个算子；GUI 拖拽（@dnd-kit）+ YAML 双向同步；调试预览；运行时执行（< 500ms / 100 节点）；流水线挂载到订阅。
- **文件**：
  - `internal/handler/pipeline_handler.go`
  - `internal/storage/{pipeline_repo,pipeline_binding_repo}.go`
  - `internal/pipeline/*.go`
  - `web/src/routes/_authed/pipelines{,.$id}.tsx`
  - `web/src/components/pipeline/*`（editor / canvas / yaml-pane / preview-pane）
  - `web/src/api/pipeline.ts` + `web/src/locales/*/pipeline.json`
- **依赖**：H-INFRA、M-SUB（订阅绑定）、M-NODE（输入 / 输出节点）。
- **公开接口**：
  ```go
  // internal/pipeline
  type Operator interface {
      Name() string
      Validate(params map[string]any) error
      Run(ctx context.Context, in []Node, params map[string]any) ([]Node, error)
  }

  type Engine interface {
      Run(ctx, ast *PipelineAST, in []Node) (*RunResult, error)  // RunResult 包含每算子 diff
      EncodeYAML(ast *PipelineAST) ([]byte, error)
      DecodeYAML(raw []byte) (*PipelineAST, error)
  }
  ```
- **数据流向**：编辑器 → 保存 → AST + YAML 双存 → 订阅 sync 时 Engine.Run → 写回 Node 表（或运行时合成、不持久化，取决于设计决策 §6.5）。

### 3.5 M-NODE：节点管理模块

- **PRD 对应**：M-NODE-1..6。
- **职责**：节点 CRUD；tag；TCPing（并发 50，200 节点 < 5s）；链式代理；搜索 / 排序；raw URI 查看。
- **文件**：
  - `internal/handler/{node_handler,tcping_handler}.go`
  - `internal/storage/node_repo.go`
  - `internal/traffic/tcping.go`（TCPing 算法）
  - `web/src/routes/_authed/nodes.tsx`
  - `web/src/components/node/*`
  - `web/src/api/node.ts` + `web/src/locales/*/node.json`
- **依赖**：H-INFRA、M-SUB（节点归属订阅）。
- **公开接口**：
  ```go
  type NodeRepo interface {
      UpsertBatch(ctx, subID string, ns []*Node) error  // 按 (sub_id, hash(server,port,type)) 去重
      ListBySub(ctx, subID string, filter NodeFilter) ([]*Node, error)
      Tag(ctx, nodeID string, tags []string) error
      SetChainParent(ctx, nodeID, parentID string) error
  }

  type TCPing interface {
      Run(ctx, ns []*Node, concurrency int) ([]*PingResult, error)
  }
  ```
- **数据流向**：M-SUB.Sync → NodeRepo.UpsertBatch；前端 TCPing → tcping_handler → TCPing.Run（goroutine pool）→ 写 latency 到内存缓存（不入库）。

### 3.6 M-RULE：规则系统模块

- **PRD 对应**：M-RULE-1..4。
- **职责**：custom_rules 三类（dns / rules / rule-providers）× 三模式（replace / prepend / append）；规则模板；最终 Clash 配置即时预览。
- **文件**：
  - `internal/handler/rule_handler.go`
  - `internal/storage/rule_repo.go`
  - `internal/substore/clash_producer.go`（注入点）
  - `web/src/routes/_authed/rules.tsx`
  - `web/src/components/rule/*`
  - `web/src/api/rule.ts` + `web/src/locales/*/rule.json`
- **依赖**：H-INFRA、M-SUB（规则注入到 Clash producer）。
- **公开接口**：
  ```go
  type RuleRepo interface { CRUD ... }
  type Injector interface {
      Inject(base *yaml.Node, rules []*CustomRule) error  // 按 sort 顺序 + mode 注入
  }
  ```
- **数据流向**：用户在 UI 配置 → RuleRepo 持久化 → SubscriptionService.Render 时 ClashProducer 调用 Injector → 输出最终 YAML。

### 3.7 M-SCRIPT：脚本扩展模块

- **PRD 对应**：M-SCRIPT-1..5。
- **职责**：goja JS 沙箱；pre_save_nodes / post_fetch 两个 hook；5s 超时强 kill；无 fs / net 能力；错误日志可查。
- **文件**：
  - `internal/handler/script_handler.go`
  - `internal/storage/script_repo.go`
  - `internal/scriptengine/*.go`
  - `web/src/routes/_authed/scripts.tsx`
  - `web/src/components/script/*`
  - `web/src/api/script.ts` + `web/src/locales/*/script.json`
- **依赖**：H-INFRA、M-SUB（hook 在 sync 流程中被调用）。
- **公开接口**：
  ```go
  type Engine interface {
      RunPreSaveNodes(ctx, code string, in []Node) ([]Node, error)
      RunPostFetch(ctx, code string, raw []byte) ([]byte, error)
  }
  // 内部用 JSON.parse 把 []Node 序列化进 vm，避免 goja 直接 Set 切片的 bug（见 §6.3）
  ```
- **数据流向**：M-SUB.Sync → if 有 post_fetch → Engine.RunPostFetch → parse → if 有 pre_save_nodes → Engine.RunPreSaveNodes → NodeRepo。

### 3.8 M-AGENT：探针 agent 模块

- **PRD 对应**：M-AGENT-1..8。
- **职责**：自写 Go agent；WebSocket 心跳；token + TLS；版本兼容；Nezha v2 协议兼容层；agent 自更新通路。
- **文件**：
  - `cmd/agent/main.go` + `cmd/agent/internal/{collector,transport,nezha_emitter}/`
  - `internal/agent/{hub,client,protocol,presence,command}.go`
  - `internal/handler/agent_handler.go`（含 WebSocket upgrade）
  - `internal/nezha/*.go` + `internal/handler/nezha_handler.go`
  - `internal/storage/{agent_repo,agent_record_repo}.go`
  - `pkg/agentlib/{proto,ws}.go`
  - `web/src/routes/_authed/agents{,.$id}.tsx`
  - `web/src/components/agent/*`
  - `web/src/api/agent.ts` + `web/src/locales/*/agent.json`
- **依赖**：H-INFRA、M-TRAFFIC（写入 agent_records）、M-NOTIFY（agent 离线事件）。
- **公开接口**：
  ```go
  // internal/agent
  type Hub interface {
      Connect(ws *websocket.Conn, agentID string) error
      Broadcast(cmd Command) int
      Send(agentID string, cmd Command) error
      List() []*Presence
  }

  // internal/nezha
  type Adapter interface { ToAgentRecord(n NezhaState) AgentRecord }
  ```
- **数据流向**：agent 启动 → Dial WebSocket（带 token）→ 30s 心跳 → AgentRecord → Hub → AgentRecordRepo.Insert；hub 下发 → Hub.Send → agent collector / shell。

### 3.9 M-TRAFFIC：流量聚合模块

- **PRD 对应**：M-TRAFFIC-1..5。
- **职责**：高频 agent_records 写入；日聚合定时任务（00:00）；月度计费周期重置（默认 1 号）；趋势图数据接口；阈值告警。
- **文件**：
  - `internal/handler/traffic_handler.go`
  - `internal/storage/{agent_record_repo,traffic_repo}.go`
  - `internal/traffic/{aggregator,monthly_reset,threshold}.go`
  - `web/src/routes/_authed/traffic.tsx`
  - `web/src/components/traffic/*`
  - `web/src/api/traffic.ts` + `web/src/locales/*/traffic.json`
- **依赖**：H-INFRA、M-AGENT（输入数据）、M-NOTIFY（阈值告警事件）。
- **公开接口**：
  ```go
  type Aggregator interface {
      RunDaily(ctx, date time.Time) error              // 跑日聚合
      RunMonthlyReset(ctx, userID string, date time.Time) error
      MonthlyUsage(ctx, userID string, ym string) (*Usage, error)
  }
  ```
- **数据流向**：每天 00:00 cron → Aggregator.RunDaily → 读 agent_records[date] → 聚合 → 写 traffic_records；UI 趋势图 → traffic_handler → 按 date 范围读 traffic_records。

### 3.10 M-NOTIFY：通知系统模块（★差异化 #2）

- **PRD 对应**：M-NOTIFY-1..5。
- **职责**：10 通道（Telegram / Discord / Slack / Email / Bark / Gotify / Webhook / Server酱 / PushDeer / IFTTT）；事件 opt-in；模板系统；Telegram Bot 双向；5 分钟去抖。
- **文件**：
  - `internal/handler/{notify_handler,stream_handler}.go`
  - `internal/storage/{notify_channel_repo,notify_event_repo}.go`
  - `internal/notify/*.go`
  - `web/src/routes/_authed/notifications{,.$id}.tsx`
  - `web/src/components/notify/*`
  - `web/src/api/notify.ts` + `web/src/locales/*/notify.json`
- **依赖**：H-INFRA、被所有业务模块通过 `notify.Manager.Emit(event)` 调用。
- **公开接口**：
  ```go
  type NotificationChannel interface {
      Name() string
      ConfigSchema() jsonschema.Schema
      Send(ctx context.Context, evt Event, cfg map[string]any) error
  }

  type Manager interface {
      Emit(ctx, evt Event) error                       // 入事件总线
      Test(ctx, channelID string) error                // UI 测试按钮
      Subscribe(stream chan<- Event)                   // 前端 SSE 用
  }
  ```
- **数据流向**：业务模块 → Manager.Emit(Event) → dedupe → 路由到 opt-in channels → template render → Channel.Send；TG Bot poll/webhook → 命令解析 → 调用业务模块 → 响应。

### 3.11 M-OPS：安全与运维模块

- **PRD 对应**：M-OPS-1..6。
- **职责**：静默模式（前缀 + 404 化）；OTA 自更新（GitHub Release + SHA-256 + 优雅重启）；短链系统；备份导出 / 恢复（P1）；/healthz；结构化日志 + 轮转；审计日志。
- **文件**：
  - `internal/handler/{silent_mode（在 middleware/）,ota_handler,shortlink_handler,healthz_handler,settings_handler,audit_handler}.go`
  - `internal/handler/middleware/silent_mode.go`
  - `internal/storage/{shortlink_repo,audit_repo,settings_repo}.go`
  - `internal/shortlink/service.go`
  - `internal/ota/*.go`
  - `internal/audit/{logger,middleware}.go`
  - `web/src/routes/_authed/{shortlinks,settings}.tsx` + `_authed/admin/{audit,ota}.tsx`
  - `web/src/components/{shortlink,ota,admin}/*`
- **依赖**：H-INFRA；被几乎所有 handler 通过 audit middleware 联动。
- **公开接口**：
  ```go
  type SilentMode interface {
      Prefix() string                                  // /_app/<32hex>/
      IsAllowed(r *http.Request) bool
  }
  type OTAService interface {
      Check(ctx) (*ReleaseInfo, error)
      Apply(ctx, info *ReleaseInfo) error              // graceful + checksum + restart
  }
  type ShortLinkService interface {
      Create(ctx, target string, ttl time.Duration) (*ShortLink, error)
      Resolve(ctx, file, user string) (string, error)
  }
  ```
- **数据流向**：所有 HTTP 请求 → silent_mode middleware → if 前缀不对 / 未授权 → 404；audit middleware → 写 audit_logs；admin OTA 触发 → OTAService.Apply → wal_checkpoint → 替换 → 重启。

### 3.12 M-I18N：国际化模块

- **PRD 对应**：M-I18N-1..5。
- **职责**：4 套 locale；命名空间按业务；浏览器语言自动检测 + 用户偏好持久化；时间 / 数字 Intl；CI lint。
- **文件**：
  - 前端：`web/src/i18n.ts`、`web/src/locales/<lang>/*.json`、`web/src/components/layout/lang-switch.tsx`
  - 后端：`internal/i18n/{catalog,locales/*.json,email_templates/*}`（仅错误码 + 邮件模板）
  - CI：`scripts/check-i18n.sh`
- **依赖**：H-INFRA；横切所有前端业务模块。
- **公开接口**：
  - 前端：`useTranslation('namespace')` + `t('key')`
  - 后端：`i18n.Catalog.Translate(code, locale)`（仅邮件用）
- **数据流向**：用户偏好（user.locale 字段）→ 前端登录后 i18next.changeLanguage → 自动加载 JSON → t() 渲染。

---

## 4. 数据模型（SQLite Schema）

### 4.1 全局约定

- **驱动**：`modernc.org/sqlite`，DSN 后缀 `?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)&_pragma=synchronous(NORMAL)`。
- **WAL 模式 + busy_timeout=5000ms**（编码规范 §5.3）。
- **写并发**：`sql.DB.SetMaxOpenConns(1)`（写连接），另起一个读连接池 `SetMaxOpenConns(8)`（同一 DB 文件，独立 `*sql.DB`）。
- **主键**：除非显式声明（如 `short_links` / `traffic_records`），均为 `TEXT PRIMARY KEY`（uuid v7 字符串），便于分发 / 调试。
- **时间**：`created_at` / `updated_at` 用 `INTEGER NOT NULL`（unix ms），SQLite 不强 NOT NULL 但代码层强约束。
- **软删除**：v1 一律物理删除（admin 删 user 时级联删除其所有资源，PRD M-USER.5 锁定）。审计日志 `audit_logs` 单独保留。
- **迁移策略**：见 §6.2，**ensureColumn 增量 + 大版本断点显式 SQL（migrations/0001_...sql）**。

### 4.2 表定义

```sql
-- 用户
CREATE TABLE users (
    id              TEXT PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,           -- bcrypt cost=10
    role            TEXT NOT NULL CHECK(role IN ('admin','user')),
    is_active       INTEGER NOT NULL DEFAULT 1,
    email           TEXT,                    -- 可为 NULL，仅作邮件通知用
    locale          TEXT NOT NULL DEFAULT 'zh-CN',
    totp_secret     TEXT,                    -- base32，2FA 启用前为 NULL
    totp_enabled    INTEGER NOT NULL DEFAULT 0,
    recovery_codes_hash TEXT,                -- JSON 数组 sha256，一次性消耗
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_users_username ON users(username);

-- session（登录态）
CREATE TABLE sessions (
    id              TEXT PRIMARY KEY,        -- session id（也是 cookie 值）
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL,           -- sha256(session_secret)
    pending_2fa     INTEGER NOT NULL DEFAULT 0,
    expires_at      INTEGER NOT NULL,
    last_used_at    INTEGER NOT NULL,
    ip              TEXT,
    user_agent      TEXT,
    created_at      INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- 订阅
CREATE TABLE subscriptions (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL CHECK(type IN ('url','upload','manual')),
    source_url      TEXT,                    -- type=url 时必填
    raw_content     BLOB,                    -- type=upload 时存原文；url 同步后也存（便于回放）
    ua              TEXT,                    -- 自定义 UA
    sync_interval   INTEGER NOT NULL DEFAULT 21600,  -- 秒，默认 6h
    last_synced_at  INTEGER,
    last_sync_status TEXT,                   -- ok / error / pending
    last_sync_error TEXT,
    expire_at       INTEGER,                 -- 订阅本身的过期
    traffic_total   INTEGER,                 -- 字节，0/NULL 表示未知
    traffic_used    INTEGER,
    tags            TEXT NOT NULL DEFAULT '[]',  -- JSON array
    remark          TEXT,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_last_synced ON subscriptions(last_synced_at);

-- 节点
CREATE TABLE nodes (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    raw_uri         TEXT NOT NULL,
    parsed_config_json TEXT NOT NULL,        -- 完整解析结果（JSON），含 _raw 兜底
    protocol        TEXT NOT NULL,
    server          TEXT NOT NULL,
    port            INTEGER NOT NULL,
    tag             TEXT NOT NULL,           -- proxy name
    tags            TEXT NOT NULL DEFAULT '[]', -- 用户/算子打的标签 JSON
    is_chain_proxy  INTEGER NOT NULL DEFAULT 0,
    chain_parent_id TEXT REFERENCES nodes(id) ON DELETE SET NULL,
    position        INTEGER NOT NULL DEFAULT 0,  -- 在订阅里的顺序
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_nodes_sub ON nodes(subscription_id);
CREATE INDEX idx_nodes_protocol ON nodes(protocol);
CREATE UNIQUE INDEX idx_nodes_sub_dedupe ON nodes(subscription_id, server, port, protocol);

-- 流水线
CREATE TABLE pipelines (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    yaml_content    TEXT NOT NULL,           -- 完整 YAML 文本（保格式）
    ast_json        TEXT NOT NULL,           -- 编译后的 AST（运行时直接用）
    version         INTEGER NOT NULL DEFAULT 1,
    schema_version  TEXT NOT NULL DEFAULT 'shiguang/v1',
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_pipelines_user ON pipelines(user_id);

-- 流水线绑定订阅（多对多 + 顺序）
CREATE TABLE pipeline_bindings (
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    pipeline_id     TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL DEFAULT 0,
    enabled         INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (subscription_id, pipeline_id)
);
CREATE INDEX idx_pipeline_bindings_sub ON pipeline_bindings(subscription_id, position);

-- 自定义规则
CREATE TABLE custom_rules (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL CHECK(type IN ('dns','rules','rule-providers')),
    mode            TEXT NOT NULL CHECK(mode IN ('replace','prepend','append')),
    content         TEXT NOT NULL,           -- YAML 片段
    enabled         INTEGER NOT NULL DEFAULT 1,
    sort            INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_custom_rules_user ON custom_rules(user_id, type, sort);

-- 脚本
CREATE TABLE scripts (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    hook            TEXT NOT NULL CHECK(hook IN ('pre_save_nodes','post_fetch')),
    code            TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    last_run_at     INTEGER,
    last_error      TEXT,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_scripts_user_hook ON scripts(user_id, hook, enabled);

-- agent
CREATE TABLE agents (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    token_hash      TEXT NOT NULL,           -- sha256(token)，token 明文只在创建时返回一次
    kind            TEXT NOT NULL CHECK(kind IN ('native','nezha_compat')),
    version         TEXT,
    os              TEXT,
    arch            TEXT,
    public_ip       TEXT,
    last_seen_at    INTEGER,
    status          TEXT NOT NULL DEFAULT 'offline' CHECK(status IN ('online','offline','degraded')),
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_agents_user ON agents(user_id);
CREATE INDEX idx_agents_token_hash ON agents(token_hash);
CREATE INDEX idx_agents_last_seen ON agents(last_seen_at);

-- agent 高频原始记录（7 天保留）
CREATE TABLE agent_records (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,  -- 高频写，用自增 int 比 uuid 省 80% 空间
    agent_id        TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    recorded_at     INTEGER NOT NULL,        -- unix ms
    cpu_percent     REAL,
    mem_used        INTEGER,
    mem_total       INTEGER,
    swap_used       INTEGER,
    swap_total      INTEGER,
    disk_used       INTEGER,
    disk_total      INTEGER,
    net_in          INTEGER,                 -- 累计字节
    net_out         INTEGER,
    net_in_speed    INTEGER,                 -- B/s 瞬时
    net_out_speed   INTEGER,
    conn_tcp        INTEGER,
    conn_udp        INTEGER,
    load1           REAL,
    load5           REAL,
    load15          REAL,
    uptime          INTEGER,                 -- 秒
    process_count   INTEGER
);
CREATE INDEX idx_agent_records_agent_time ON agent_records(agent_id, recorded_at);
-- 注：7 天保留由 internal/traffic/aggregator.go 定时 DELETE，不走外键 ON DELETE。

-- 流量日聚合（长期保留）
CREATE TABLE traffic_records (
    date            TEXT NOT NULL,           -- YYYY-MM-DD
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id        TEXT REFERENCES agents(id) ON DELETE SET NULL,  -- 按 agent 维度分行
    total_limit     INTEGER,                 -- 字节，NULL 表示无限额
    total_used      INTEGER NOT NULL DEFAULT 0,
    total_in        INTEGER NOT NULL DEFAULT 0,
    total_out       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (date, user_id, agent_id)
);
CREATE INDEX idx_traffic_user_date ON traffic_records(user_id, date);

-- 通知通道
CREATE TABLE notification_channels (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,           -- telegram / discord / slack / email / ...
    name            TEXT NOT NULL,
    config_json     TEXT NOT NULL,           -- 渠道配置（含 webhook url / token / chat_id 等）
    template        TEXT,                    -- 用户自定义 Go template；空走默认
    event_types     TEXT NOT NULL DEFAULT '[]',  -- JSON array of subscribed event types
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_notification_channels_user ON notification_channels(user_id, kind);

-- 通知事件投递日志
CREATE TABLE notification_events (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id      TEXT REFERENCES notification_channels(id) ON DELETE SET NULL,
    event_type      TEXT NOT NULL,
    dedupe_key      TEXT,                    -- sha1(event_type+resource)，用于去抖
    payload         TEXT NOT NULL,           -- JSON
    status          TEXT NOT NULL CHECK(status IN ('pending','sent','failed','skipped_dedupe')),
    sent_at         INTEGER,
    error           TEXT,
    created_at      INTEGER NOT NULL
);
CREATE INDEX idx_notification_events_user_time ON notification_events(user_id, created_at);
CREATE INDEX idx_notification_events_dedupe ON notification_events(dedupe_key, created_at);

-- 短链
CREATE TABLE short_links (
    file_code       TEXT NOT NULL,
    user_code       TEXT NOT NULL,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_url      TEXT NOT NULL,
    expires_at      INTEGER,                 -- NULL = 永久
    created_at      INTEGER NOT NULL,
    PRIMARY KEY (file_code, user_code)
);
CREATE INDEX idx_short_links_user ON short_links(user_id);
CREATE INDEX idx_short_links_expires ON short_links(expires_at);

-- 审计日志
CREATE TABLE audit_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         TEXT REFERENCES users(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,           -- login / create_sub / delete_sub / ota_apply / ...
    resource_type   TEXT,
    resource_id     TEXT,
    ip              TEXT,
    user_agent      TEXT,
    payload         TEXT,                    -- 操作详细 JSON
    success         INTEGER NOT NULL DEFAULT 1,
    created_at      INTEGER NOT NULL
);
CREATE INDEX idx_audit_user_time ON audit_logs(user_id, created_at);
CREATE INDEX idx_audit_action_time ON audit_logs(action, created_at);

-- 系统设置（k/v）
CREATE TABLE system_settings (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL,
    updated_at      INTEGER NOT NULL
);
-- 已知 key：silent_mode_enabled / silent_mode_prefix / monthly_reset_day / session_ttl_seconds / smtp_config / ota_check_interval / ...
```

### 4.3 索引 / 唯一性总览

- `users.username` UNIQUE。
- `nodes(subscription_id, server, port, protocol)` UNIQUE（同订阅内防重复入库）。
- `pipeline_bindings(subscription_id, pipeline_id)` 复合 PK。
- `short_links(file_code, user_code)` 复合 PK。
- `traffic_records(date, user_id, agent_id)` 复合 PK。
- `agent_records(agent_id, recorded_at)` 高频查询索引。
- `notification_events(dedupe_key, created_at)` 去抖窗口查询。
- `sessions(expires_at)` 用于过期清理。

### 4.4 数据保留策略

| 表 | 策略 |
|---|---|
| `agent_records` | 默认保留 7 天，可配 1-30 天；每天 03:00 cron 跑 `DELETE WHERE recorded_at < ?`。 |
| `traffic_records` | 永久保留。 |
| `notification_events` | 默认保留 90 天。 |
| `audit_logs` | 默认保留 180 天，admin 可手动归档。 |
| `sessions` | 过期后保留 7 天供审计查询，再 cron 清理。 |

### 4.5 大版本断点

v1.0 发布前合并 `migrations/0001_initial.sql`（包含上述全部 CREATE TABLE）。
后续小补丁通过 `db.EnsureColumn(table, col, ddl)` 增量；累积到大版本（v2.0）再合并到 `0002_*.sql`。

---

## 5. 接口设计

### 5.1 HTTP API 路由表

约定：
- 所有 `/api/*` 必须经过 silent_mode middleware（无授权直接 404）。
- 鉴权标签：`[public]` 不需登录但仍受静默模式保护；`[pending2fa]` 已登录但未过 2FA；`[user]` 完整登录；`[admin]` admin role。
- 所有写入操作（POST/PUT/DELETE）经过 audit middleware。

#### 5.1.1 静默 / 公开
```
GET    /_app/<32hex>/*                              - 前端 SPA（embed.FS）          [public]
GET    /healthz                                     - 健康检查（可关）              [public]
GET    /s/:code                                     - 短链跳转                      [public]
GET    /download/:name                              - sub-store 兼容输出           [public+token]
GET    /api/v1/nezha/heartbeat                      - Nezha v2 心跳                [public+token]
POST   /api/v1/nezha/report                         - Nezha 上报                   [public+token]
```

#### 5.1.2 认证（M-USER）
```
POST   /api/auth/login                              - 用户名密码登录                [public]
POST   /api/auth/verify-totp                        - 验证 TOTP                    [pending2fa]
POST   /api/auth/verify-recovery                    - 用备份码登录                  [pending2fa]
POST   /api/auth/logout                             - 登出                         [user]
POST   /api/auth/refresh                            - 滑动续期 session             [user]
GET    /api/me                                      - 当前用户信息                  [user]
PATCH  /api/me                                      - 改用户名 / locale / email    [user]
POST   /api/me/password                             - 改密                         [user]
DELETE /api/me                                      - 删除自己账号                 [user]
GET    /api/me/totp/setup                           - 生成 TOTP secret + qrcode    [user]
POST   /api/me/totp/enable                          - 启用 2FA（验证 code）         [user]
POST   /api/me/totp/disable                         - 关闭 2FA（验证 code+密码）    [user]
POST   /api/me/totp/recovery-codes                  - 重新生成备份码                [user]
GET    /api/me/sessions                             - 列出我的活跃 session         [user]
DELETE /api/me/sessions/:id                         - 踢掉某 session               [user]
```

#### 5.1.3 用户管理（M-USER admin）
```
GET    /api/admin/users                             - 列出用户                     [admin]
POST   /api/admin/users                             - 创建用户                     [admin]
GET    /api/admin/users/:id                         - 用户详情                     [admin]
PATCH  /api/admin/users/:id                         - 修改用户                     [admin]
DELETE /api/admin/users/:id                         - 删除用户（级联）             [admin]
POST   /api/admin/users/:id/reset-password          - 重置用户密码                 [admin]
POST   /api/admin/users/:id/disable-2fa             - 强制关闭某用户 2FA           [admin]
```

#### 5.1.4 订阅（M-SUB）
```
GET    /api/subscriptions                           - 列出我的订阅                 [user]
POST   /api/subscriptions                           - 创建订阅（url/upload/manual）[user]
GET    /api/subscriptions/:id                       - 订阅详情（含节点统计）       [user]
PATCH  /api/subscriptions/:id                       - 修改订阅元数据                [user]
DELETE /api/subscriptions/:id                       - 删除订阅                     [user]
POST   /api/subscriptions/:id/sync                  - 触发立即同步                 [user]
GET    /api/subscriptions/:id/raw                   - 查看原始内容                 [user]
GET    /api/subscriptions/:id/output                - 输出 Clash YAML              [user]
POST   /api/subscriptions/upload                    - 上传 yaml 文件                [user]
GET    /api/subscriptions/:id/pipelines             - 列出绑定的流水线              [user]
PUT    /api/subscriptions/:id/pipelines             - 重置绑定 + 顺序                [user]
```

#### 5.1.5 节点（M-NODE）
```
GET    /api/subscriptions/:id/nodes                 - 列出订阅下节点               [user]
POST   /api/subscriptions/:id/nodes                 - 手动添加节点（URI）           [user]
GET    /api/nodes/:id                               - 节点详情                     [user]
PATCH  /api/nodes/:id                               - 改 tag / chain_parent_id    [user]
DELETE /api/nodes/:id                               - 删除节点                     [user]
POST   /api/nodes/tcping                            - 批量 TCPing                  [user]
POST   /api/nodes/:id/chain                         - 设置链式代理出口              [user]
```

#### 5.1.6 流水线（M-PIPE）
```
GET    /api/pipelines                               - 列出我的流水线                [user]
POST   /api/pipelines                               - 创建流水线                   [user]
GET    /api/pipelines/:id                           - 详情（含 yaml + ast）        [user]
PUT    /api/pipelines/:id                           - 整体保存（双更 yaml 和 ast） [user]
DELETE /api/pipelines/:id                           - 删除                         [user]
POST   /api/pipelines/:id/run                       - 在指定订阅上跑预览（dry-run）[user]
POST   /api/pipelines/yaml-to-ast                   - YAML → AST 转换（无副作用）[user]
POST   /api/pipelines/ast-to-yaml                   - AST → YAML 转换              [user]
GET    /api/pipelines/operators                     - 列出可用算子 + schema        [user]
```

#### 5.1.7 规则（M-RULE）
```
GET    /api/rules                                   - 列出                         [user]
POST   /api/rules                                   - 创建                         [user]
PATCH  /api/rules/:id                               - 修改                         [user]
DELETE /api/rules/:id                               - 删除                         [user]
PUT    /api/rules/order                             - 批量改 sort                  [user]
GET    /api/rules/preview/:subID                    - 预览注入后的 Clash YAML      [user]
GET    /api/rules/templates                         - 预设模板列表                  [user]
```

#### 5.1.8 脚本（M-SCRIPT）
```
GET    /api/scripts                                 - 列出                         [user]
POST   /api/scripts                                 - 创建                         [user]
PATCH  /api/scripts/:id                             - 修改                         [user]
DELETE /api/scripts/:id                             - 删除                         [user]
POST   /api/scripts/:id/test                        - 用样例数据跑一次（dry-run） [user]
GET    /api/scripts/:id/logs                        - 查最近错误日志               [user]
```

#### 5.1.9 agent（M-AGENT）
```
GET    /api/agents                                  - 列出我的 agent               [user]
POST   /api/agents                                  - 创建 agent（生成 token）      [user]
GET    /api/agents/:id                              - agent 详情                   [user]
PATCH  /api/agents/:id                              - 改名 / 配置                  [user]
DELETE /api/agents/:id                              - 删除 + 吊销 token            [user]
POST   /api/agents/:id/rotate-token                 - 轮换 token                   [user]
POST   /api/agents/:id/restart                      - 下发 restart 命令            [user]
GET    /api/agents/:id/records?from=&to=            - 高频原始记录                 [user]
GET    /api/admin/agents                            - admin 看全系统 agent         [admin]
```

#### 5.1.10 WebSocket（agent + 前端）
```
GET    /api/agent/ws?token=xxx                      - agent ↔ hub 长连接          [agent-token]
GET    /api/notify/stream                           - 前端订阅实时事件（SSE）       [user]
```

#### 5.1.11 流量（M-TRAFFIC）
```
GET    /api/traffic/summary                         - 当月概览（限额/已用/剩余）    [user]
GET    /api/traffic/chart?range=day|month&from=&to= - 趋势图数据                   [user]
GET    /api/traffic/by-agent                        - 按 agent 拆分                [user]
POST   /api/traffic/threshold                       - 设置阈值                     [user]
```

#### 5.1.12 通知（M-NOTIFY）
```
GET    /api/notify/channels                         - 列出通道                     [user]
POST   /api/notify/channels                         - 创建通道                     [user]
PATCH  /api/notify/channels/:id                     - 修改                         [user]
DELETE /api/notify/channels/:id                     - 删除                         [user]
POST   /api/notify/channels/:id/test                - 发测试消息                   [user]
GET    /api/notify/channel-kinds                    - 列出可用通道类型 + schema    [user]
GET    /api/notify/event-types                      - 列出事件类型                  [user]
GET    /api/notify/events?status=&from=&to=         - 投递历史                     [user]
POST   /api/notify/telegram/webhook/:token          - TG Bot webhook 接收点         [public+token]
```

#### 5.1.13 短链（M-OPS）
```
GET    /api/shortlinks                              - 我的短链列表                  [user]
POST   /api/shortlinks                              - 创建短链                     [user]
DELETE /api/shortlinks/:fileCode/:userCode          - 删除                         [user]
```

#### 5.1.14 OTA / 设置 / 审计（M-OPS）
```
GET    /api/admin/ota/check                         - 检查更新                     [admin]
POST   /api/admin/ota/apply                         - 触发升级                     [admin]
GET    /api/admin/ota/history                       - 升级历史                     [admin]
GET    /api/admin/settings                          - 系统设置                     [admin]
PATCH  /api/admin/settings                          - 修改设置                     [admin]
POST   /api/admin/settings/silent-mode              - 开关静默模式 / 轮换前缀       [admin]
GET    /api/admin/audit                             - 审计日志                     [admin]
POST   /api/admin/backup                            - 触发备份                     [admin]
POST   /api/admin/restore                           - 从备份恢复                   [admin]
```

合计：**约 102 个 HTTP endpoint**（含静默模式公开路径 6 个 + Nezha 兼容 2 个）。

### 5.2 WebSocket 端点

| 路径 | 方向 | 用途 | 鉴权 |
|---|---|---|---|
| `/api/agent/ws?token=xxx` | agent ↔ hub | 心跳、上报、命令下发 | agent token（错则 404） |
| `/api/notify/stream` | hub → 前端 | 实时事件流（SSE，不用 WS 也可，但前端预留 WS upgrade） | session cookie |

**agent 协议消息信封**（`pkg/agentlib/proto.go`）：

```go
type Envelope struct {
    Type    string          `json:"type"`    // hello / heartbeat / metric / cmd / ack / bye
    ID      string          `json:"id"`      // 客户端生成
    Payload json.RawMessage `json:"payload"`
}

type HelloPayload struct {
    AgentID string `json:"agent_id"`
    Version string `json:"version"`
    OS      string `json:"os"`
    Arch    string `json:"arch"`
}

type MetricPayload AgentRecord  // 复用 types/agentproto/record.go
type CmdPayload struct{ Cmd string; Args map[string]any }
```

### 5.3 静默模式特殊路径

- 入口前缀 `/_app/<32hex>/`，由首次启动时生成（写入 `data/silent_prefix.txt` + 打印到日志）。
- 允许列表（绕过前缀强制，但仍受其他鉴权）：
  - `/healthz`（admin 可关）
  - `/s/:code`（短链）
  - `/download/:name`（sub-store 兼容，必带 token）
  - `/api/v1/nezha/*`（必带 token）
  - `/api/notify/telegram/webhook/:token`（必带 token）
  - `/api/agent/ws`（必带 token）
  - `/api/auth/login`（必须在 `/_app/<32hex>/api/auth/login` 形式下访问，前端自动注入前缀）
- 其他所有路径：未匹配 → 返回 nginx 默认 404 body + `Server: nginx`。
- 32hex 前缀可由 admin 在 `/api/admin/settings/silent-mode` 触发轮换（详 §6.7）。

---

## 6. 关键设计决策

每条按"上下文 / 选择 / 理由 / 替代方案为何放弃 / 后续可优化方向"展开。

### 6.1 SQLite journal 模式：WAL vs DELETE

- **上下文**：modernc.org/sqlite 默认 DELETE 模式，读写互锁，并发性能差。
- **选择**：**WAL**，DSN 强制 `journal_mode=WAL` + `busy_timeout=5000` + `synchronous=NORMAL`。
- **理由**：(1) WAL 读不阻塞写、写不阻塞读，是 SQLite 多并发的标配；(2) 妙妙屋同款，社区已验证；(3) 编码规范 §5.3 强制要求；(4) 写并发 1 + 读并发 8 的双连接池模型对 hub 这种 P99 < 50 QPS 的场景绰绰有余。
- **替代为何放弃**：DELETE 模式在 SQLite 多 goroutine 写时频繁触发 SQLITE_BUSY；WAL2（SQLite 3.46+）尚处实验阶段，modernc 跟进慢。
- **后续优化**：v1.5 可探索 `PRAGMA wal_autocheckpoint`（默认 1000 页）调优 + 定期 `PRAGMA wal_checkpoint(TRUNCATE)`（每天 03:00 跑，配合 agent_records 清理）。

### 6.2 Schema 迁移：ensureColumn 增量 vs 显式 SQL migration

- **上下文**：妙妙屋走 ensureColumn 路线（运行时按需 ADD COLUMN），简单但难追溯；显式 SQL migration（如 golang-migrate）规范但开销大。
- **选择**：**双轨制 —— v1.0 大版本断点用显式 `migrations/000X_*.sql`，小补丁用 `db.EnsureColumn(table, col, ddl)`**。
- **理由**：(1) 大版本断点（v1.0 → v2.0）容量超过 ensureColumn 表达力（涉及表重构、数据回填）；(2) v1.0 → v1.x 小迭代用 ensureColumn 可在启动时自动跟进，无需用户跑迁移命令；(3) 编码规范 §7.1 留了 `migrations/` 目录；(4) ensureColumn 的实现只需 30 行（`PRAGMA table_info` + 比对 + `ALTER TABLE ADD COLUMN`）。
- **替代为何放弃**：纯 ensureColumn 路线无法表达 DROP / RENAME / INDEX 重建；纯 migration 路线对个人自托管用户体验不佳（升级要看文档）。
- **后续优化**：v2.0 可考虑切到 `ariga/atlas` 或 `pressly/goose`，引入 declarative schema diff。

### 6.3 goja JS 数据传递：JSON.parse vs vm.Set

- **上下文**：goja 把 `[]Node`（Go 切片）通过 `vm.Set("nodes", nodes)` 直接暴露会导致字段名大小写转换、嵌套 map 类型不一致、修改后无法回传 Go 切片等问题；妙妙屋已踩坑。
- **选择**：**用 JSON 序列化作中介 —— `vm.Set("nodesJSON", string(jsonBytes))` + 用户代码先 `JSON.parse(nodesJSON)`；返回时也走 `JSON.stringify` 字符串**。
- **理由**：(1) 完全规避 goja 的反射坑（结构体 tag、嵌套 map、interface{} 兼容）；(2) 单次 JSON 编解码在 200 节点规模下 < 10ms，远低于 PRD M-PIPE.3 的 500ms 预算；(3) 与浏览器侧 JS 行为一致，用户脚本可本地调试；(4) 妙妙屋实战路线已验证。
- **替代为何放弃**：`vm.Set` 暴露原生切片会让用户脚本写出"看似生效但 hub 端没拿到"的 bug，调试地狱。
- **后续优化**：v1.x 可在 sandbox 内提供 `__shiguang__.json.parse/stringify` 内置函数，避免每次都重复写 boilerplate。

### 6.4 agent 协议：WebSocket pull 心跳 vs gRPC push

- **上下文**：Nezha 选 gRPC（双向流），Komari 选 WebSocket，妙妙屋通过外部 agent 拉数据。
- **选择**：**WebSocket 长连接，30s 心跳（可配 5-300s），双向消息**。
- **理由**：(1) ADR 0003 已锁定；(2) Komari 实战已证明 WebSocket 时延比 gRPC pull 低、实现复杂度低；(3) 跨 NAT / 防火墙友好（标准 HTTPS 端口）；(4) 与浏览器 `/api/notify/stream` 走同一个 mux，运维简单；(5) gorilla/websocket 是事实标准库，无需额外学习；(6) `pkg/agentlib` 让 agent 与 hub 共享 envelope 编解码，protocol drift 风险低。
- **替代为何放弃**：gRPC 需要 protobuf 工具链 + 单二进制体积 +5MB（grpc-go 太重）；HTTP long-polling 在大量 agent 时无法复用连接，开销更大。
- **后续优化**：v1.x 若发现 WebSocket 在某些环境被截断，加 HTTPS long-polling fallback（ADR 0003 已留口子，PRD §7.1 记入 P2）。

### 6.5 流水线 AST 存储：JSON 字段 vs YAML 字段 vs 双存

- **上下文**：流水线需要支持 GUI（结构化 AST 操作）和 YAML（文本编辑、git 化）两种入口（M-PIPE-3 / M-PIPE-4）。
- **选择**：**双存 —— `pipelines.yaml_content`（用户原文，保格式）+ `pipelines.ast_json`（编译产物，运行时直接用）**，保存时一并写入；冲突场景下 yaml 为权威，ast 是缓存。
- **理由**：(1) YAML 是用户的"事实源"（git 化），不能丢；(2) AST 直接进引擎跑，省每次解析的成本（100 节点流水线本身就要在 500ms 跑完，省一次 yaml decode 是值得的）；(3) 双存代价仅多一个 TEXT 字段，可忽略；(4) 保存时序：UI 编辑 → 算 AST → AST 反编译生成 YAML → 都进 DB；YAML 模式编辑 → 解析 AST → 都进 DB。
- **替代为何放弃**：只存 AST → 丢失用户在 YAML 模式下的注释 / 引号风格 / 顺序，GitOps 用户不满；只存 YAML → 每次运行都重新 parse，500ms 预算紧张。
- **后续优化**：v1.x 可加 `ast_hash` 字段，启动时校验 `hash(yaml) == ast.source_hash`，不一致则以 YAML 为准重编译。

### 6.6 通知去抖：channel 层 vs event bus 层

- **上下文**：同一事件（如某节点离线）在 5 分钟窗口内不应重复发；但用户可能有 3 个通道都订阅了"节点离线"，去抖窗口在哪一层？
- **选择**：**在 event bus 层去抖（`notify.Manager.Emit` 入口）—— 同一 `dedupe_key` 在 5 分钟窗口内只触发一次"路由到所有 channel"**。
- **理由**：(1) 用户视角："3 分钟前已被告知 NodeA 离线，5 分钟内别再吵我"是按事件视角，不是按渠道视角；(2) 在 event bus 层只查一次 `notification_events` 表（按 `dedupe_key` + `created_at > now-5min`），比每个 channel 各自查更高效；(3) 失败重试不能算"再次触发"，重试通道维度处理（status=failed 时自动重试 3 次，不进入去抖）。
- **替代为何放弃**：在 channel 层去抖 → 用户给 Telegram + Email 都订阅"节点离线"时，3 分钟前发了 Telegram，现在新增 Email 通道后该事件再来，Email 也不发 → 反直觉。
- **后续优化**：v1.x 加"分级去抖"：节点离线 5 分钟 / 流量告警 1 小时 / 备份完成不去抖（每次都发）—— 在事件类型 schema 里声明窗口。

### 6.7 静默模式 32hex 前缀：固定 vs 可轮换

- **上下文**：32hex 前缀一旦泄露（如不小心截图分享），扫描器能直接命中。
- **选择**：**可轮换 + 默认固定生命周期**——首次启动随机生成；admin 可在设置页一键轮换；轮换后前端会话被强制重新登录，新前缀必须重新告知 admin。
- **理由**：(1) 妙妙屋是固定的，但已有用户反馈"截图泄露"风险；(2) 轮换实现简单（更新 `system_settings.silent_mode_prefix` + 广播 invalidate session 给所有在线 session）；(3) 提供逃生舱：admin 可设置"30 天自动轮换"配合通知，但 v1 不强制；(4) 同时提供"显示 / 不显示"开关，不在 admin 界面显示当前前缀（admin 知道就行，不让侧坐看见）。
- **替代为何放弃**：固定路线 → 一旦泄露需要重新部署；纯自动轮换 → 用户登录体验差（每月要找新地址）。
- **后续优化**：v1.x 加"信任设备"机制（首次登录后浏览器记 cookie，绕过前缀强校验），让轮换无感知。

### 6.8 OTA 信任根：GitHub Release 直信 vs cosign 签名

- **上下文**：OTA 自更新涉及"下载新二进制并执行"，攻击者一旦窃取 GitHub 账号或仓库 maintainer 权限即可投毒。
- **选择**：**v1.0 走 GitHub Release 直信 + SHA-256 校验（在 Release 描述里发布），v1.5 起加 cosign keyless 签名**。
- **理由**：(1) v1.0 用户基数小、攻击面有限，cosign 部署增加 release 流程复杂度（需配 OIDC issuer）；(2) GitHub Release 本身有 webhook 防篡改 + 改动审计；(3) SHA-256 在 Release 描述发布后用户可侧链校验（如改成`go install`手动校验）；(4) v1.5 引入 cosign keyless 是渐进路径，不破坏既有用户。
- **替代为何放弃**：cosign 直接上 v1.0 → 个人维护者 OIDC 工作流配置成本高；不校验 → 安全红线，绝不接受。
- **后续优化**：v1.5 加 cosign keyless 签名（GitHub Actions OIDC 自签）+ Sigstore 透明日志；v2.0 探索 in-toto attestation。

---

## 附录：模块依赖图（文本版）

```
                       ┌────────────┐
                       │  H-INFRA   │
                       │  (auth /   │
                       │   storage /│
                       │   logger / │
                       │   types)   │
                       └─────┬──────┘
                             │  被所有模块依赖
       ┌────────┬────────┬──┴──┬────────┬────────┬────────┐
       ▼        ▼        ▼     ▼        ▼        ▼        ▼
   M-USER  M-SUB    M-NODE  M-RULE  M-SCRIPT  M-OPS    M-I18N
              │        ▲       │        │       (跨切)
              │        │       │        │
              ▼        │       │        │
            M-PIPE ────┘       │        │
              │                ▼        ▼
              │             M-NOTIFY ◀──┘
              ▼                ▲
            M-AGENT ───────────┤
              │                │
              ▼                │
           M-TRAFFIC ──────────┘
```

边语义：A → B 表示 A import / 调用 B；M-NOTIFY 被多个模块调用（事件发布），是中央总线。M-I18N 横切前端所有模块，无独立后端调用关系（仅 i18n.Catalog 用于邮件模板渲染）。
