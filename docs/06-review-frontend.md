# Code Review: TypeScript / React 前端

日期：2026-05-20
范围：`web/src/**`（排除 `routeTree.gen.ts`、`*.test.tsx`、`*.spec.ts`）
审查代码量：约 25,000 行 TS/TSX
基准 commit：HEAD（main）

---

## 审查摘要

整体质量较高：架构清晰（api / hooks / lib / stores / components 分层规整）、严格 TypeScript（业务代码无 `any`）、4 套 locale key 集合完全对齐、四态（loading / error / empty / data）覆盖到位、auth 流程 `next` 参数有 same-origin 校验、API 错误统一走 `ApiError` + i18n。差异化核心组件（流水线编辑器、命令面板、SSE）实现得很扎实。

主要问题集中在两类：
1. **`/not-found` 与 `/` 两个"边角"路由违反硬约束**（裸 hex、硬编码英文/中文文案）。
2. **若干次要的视觉阶梯偏差**（Monaco fontSize=13、Tailwind `h-3.5/h-9` 等非阶梯尺寸），分布广但影响小。

视觉规范命中：裸 hex 3 处、任意 px CSS 1 处、`h-3.5/w-3.5` 等非阶梯 Tailwind 步进 34 处。

---

## 严重问题（必须修复）

- [ ] [bug] `web/src/routes/_public/not-found.tsx:18,21,23` — 裸 hex 字面量 `#888 / #aaa / #ddd` 直接写在 inline `style`，违反 §编码硬约束"禁止裸 hex"。`padding: "60px 20px"`、`width: "60%"`、`margin: "0 auto 16px"` 还顺带把 px / 百分比硬编码进去，且没有走 token。建议用 `--color-text-tertiary` / `--color-border` token + `text-center`/`max-w-*` class 重写；nginx 模仿要保留但用 token 着色即可（不需要为了"伪装"裸 hex，因为这是登录前但已是面板自有 HTML）。
- [ ] [bug] `web/src/routes/index.tsx:18,21` — `"Welcome to 拾光VPS"` 与 `"Self-hosted VPS panel"` 是硬编码英文 + 中文混合文案，没有走 `t()`，违反 §i18n 硬约束。注释里也写"once auth is implemented (T-6)"——既然 T-6 已经合入，这页应当直接 `redirect` 到 `/dashboard` 或 `/login`，而不是渲染欢迎卡。
- [ ] [bug] `web/src/routes/_public/totp.tsx:89-92` — 错误分支死代码：`err.code === "ERR_AUTH_TOTP_INVALID" ? t("auth:totp.error.invalid") : t("auth:totp.error.invalid")`——三元两侧返回同一 key，分支无意义。要么补一条 `ERR_AUTH_TOTP_LOCKED`/速率限制专属文案，要么去掉条件。
- [ ] [bug] `web/src/components/nodes/node-table.tsx:186` — `<LatencyBadge latencyMs={node.reachable ? node.latency_ms : node.latency_ms} />` 三元两侧返回相同值，是死条件。原意大概率是 `reachable ? node.latency_ms : -1`（与 `Node.latency_ms = -1` 表示不可达的契约一致）；当前写法在 `reachable=false` 时仍会渲染原始 `latency_ms` 值，可能误导用户。
- [ ] [style] `web/src/components/nodes/node-table.tsx:243` — `<span className="tabular-nums">total {total}</span>`——`"total "` 是硬编码英文，应走 `t("common:pagination.total", { count: total })`（其他列表组件用的是 i18n key）。
- [ ] [style] `web/src/components/layout/lang-switch.tsx:31` — `aria-label="Language"` 是硬编码英文。屏幕阅读器用户会听到固定英文，应走 `t("common:lang.aria_label")` 或类似 key。同样的问题在 `components/admin/user-table.tsx:197`、`script/script-list.tsx:105`、`subscription/sub-list.tsx:191`、`routes/_authed/pipelines/index.tsx:292` 用 `aria-label="actions"`，以及 `subscription/sub-create-wizard.tsx:212` 的 `aria-label="wizard steps"`、`dashboard/stats-cards.tsx:143` 的 `aria-label="sparkline"`。

---

## 建议改进（推荐修复）

- [ ] [style] Monaco 编辑器固定 `fontSize: 13`（`components/notify/template-editor.tsx:66`、`components/script/script-editor.tsx:39`）——13px 不在字号阶梯（11/12/14/16/18/24/32/48），应改为 12 或 14，或读取 `--font-size-sm` CSS 变量再 px 化。`yaml-pane.tsx:79`、`script-test-panel.tsx:45`、`rule-preview-pane.tsx:192` 的 `fontSize: 12` 命中阶梯，OK。
- [ ] [style] `web/src/hooks/use-event-stream.ts:64-65` — `names = Object.keys(handlersRef.current)` 在 effect 挂载时一次性快照，注释说"callers can swap callbacks without forcing a reconnect"，但如果调用方动态增删事件名（不只是替换 callback），新事件名不会被监听。建议要么在文档里明确"handlers 的 keys 集合必须稳定"，要么把 `Object.keys(handlers)` 作为 effect 依赖。
- [ ] [style] `web/src/lib/silent-prefix.ts` + `web/src/hooks/use-event-stream.ts:53` — SSE 把 JWT token 以 query string `?token=<jwt>` 传递。这是 EventSource 唯一可行的方式，但 token 会被中间反代日志记录。建议在 `nginx`/反代部署文档里强调"打开 `access_log` 时需要过滤 `token=` 参数"，或把 SSE 改成短时性的一次性 ticket（用 POST 换 ticket，SSE 用 ticket 鉴权）。
- [ ] [style] `web/src/stores/auth-store.ts:42` — `token` 直接以明文持久化到 `localStorage`（key=`sgvps_auth`）。这是 SPA 常见折中（与 §安全 checklist "localStorage 是否存敏感 token：应该不是明文" 的要求冲突）。如果决定不走 httpOnly cookie，至少在代码注释里写清楚是有意识的折中，并补一个 XSS 防御 review 项目。
- [ ] [style] `web/src/lib/query-client.ts:60` — `handle401Redirect()` 用 `window.location.pathname.startsWith("/login")` 检测是否在登录页，但实际登录路径是 `/_app/<random>/login`（静默模式）或路由前缀 `/login`。在静默模式下 prefix 还在的话此检查会判错，导致 401 时无限弹回。建议读 `prefixedPath` 或 `useAuthStore` 的"未登录"状态作判据。
- [ ] [style] `web/src/components/layout/lang-switch.tsx:13-17` 与 `components/auth/profile-basic-form.tsx:103-106` 中的母语显示（中文/English/日本語/한국어）虽然是硬编码 CJK，但属于"语言切换器母语标签"的 UX 惯例（参考 Google/GitHub 等），技术上违反 §8.1 但语义上合理。建议在 i18n 校验脚本里把这两个文件加入白名单并写注释说明。
- [ ] [style] `web/src/components/notify/template-editor.tsx:147` — `h-[300px]` 是任意像素硬编码。设计稿如果是固定 300px 编辑器高度，建议改成 `min-h-[var(--size-monaco-md)]` 或 `aspect-[16/9]`/`h-72`(288)/`h-80`(320) 等阶梯值。`components/layout/cmd-k.tsx:222` 的 `max-h-[480px]` 同理。
- [ ] [style] Tailwind 非阶梯步进广泛存在 `h-3.5 / w-3.5 / h-9 / py-2.5` 等（34 处）——这些不在 cheatsheet 列出的 `0/0.5/1/1.5/2/3/4/6/8/12/16/20/24` 阶梯里。如果决定接受这些（因为 shadcn 默认就用 h-9），建议在 cheatsheet 里补一节"shadcn 兼容尺寸"白名单；否则要逐一改为 4px 倍数。
- [ ] [style] `web/src/api/subscription.ts:8-13,33-39` — `SubscriptionDetail` 与 `RotateShareTokenResponse` 在 TS api wrapper 里 ad-hoc 扩展，注释提到"codegen 未跟上"。这违反 §3.3"禁止前后端各自维护一份类型"。应该补 types codegen 步骤，让 `types/api.ts` 真正同步后端 Go 结构体，然后删掉此处的本地扩展。

---

## 视觉规范违规（速查卡 grep 结果）

- **裸 hex 命中：3 处**（全部在同一个文件）
  - `routes/_public/not-found.tsx:18` `#888`
  - `routes/_public/not-found.tsx:21` `#aaa`
  - `routes/_public/not-found.tsx:23` `#ddd`
  - 替换建议：`var(--color-text-tertiary)` / `var(--color-border)`

- **rgb/hsl 命中：0 处** ✅

- **任意像素硬编码（CSS 字面量）命中：6 处**
  - `routes/_public/not-found.tsx:17,22,23` — `padding: "60px 20px"`、`margin: "0 auto 16px"`、`width: "60%"`、`borderTop: "1px solid #ddd"`
  - `components/notify/template-editor.tsx:147` — `h-[300px]`
  - `components/layout/cmd-k.tsx:222` — `max-h-[480px]`
  - `components/layout/app-shell.tsx:27,28` — `gridTemplateRows: "56px 1fr"` / `gridTemplateColumns: "240px 1fr"`（设计稿 explicit 值，属于布局常量，可接受但建议提到 token）
  - `components/dashboard/stats-cards.tsx:141` — `gap-[2px]`（= space-0.5，命中阶梯，OK）
  - 此外 Monaco `fontSize: 13` 在 2 处（已列上方）也属于"非阶梯字号"。

- **Tailwind 非阶梯步进：34 处** `h-3.5/w-3.5/h-2.5/w-2.5/py-2.5/px-2.5`（分散于 `components/pipeline/*.tsx`、`components/agent/agent-card.tsx`、`components/ui/dropdown-menu.tsx`、`components/nodes/tcping-button.tsx` 等）。
  - 另：`h-9` 出现在 5 处（button / input / tabs 等）—— shadcn 默认尺寸，决策点。

- **硬编码 CJK 命中：3 处**
  - `routes/index.tsx:18` — `"Welcome to 拾光VPS"`（混合）
  - `components/layout/lang-switch.tsx:13` — `"中文（简体）"`（语言切换器母语标签，合理但应白名单）
  - `components/auth/profile-basic-form.tsx:103` — `<option value="zh-CN">中文</option>`（同上）

- **Anti-patterns 违反：0 处** ✅（无 gradient / backdrop-blur / glassmorphism / shadow-2xl / drop-shadow）

---

## i18n

- **4 套 locale key 集合一致：✅**（手动对比 16 个 namespace 全部对齐，参考 `docs/_lint-violations.md` T-34 结论）
- **硬编码 user-facing 文案：❌**
  - `routes/index.tsx:18,21`（混合英文+CJK）
  - `routes/_public/not-found.tsx:22,24`（"Not Found"、"nginx/1.27.0"——nginx fingerprint 文本是有意伪装，可接受；"Not Found" 仍建议 i18n）
  - `components/nodes/node-table.tsx:243` 的 `"total "` 字面量
  - 6 处硬编码英文 `aria-label`（详上方"严重问题"）

---

## 依赖安全

- **`pnpm audit --audit-level=high` 结果**：0 个高危
- **`pnpm audit --audit-level=moderate` 结果**：8 个 moderate，**全部为 `dompurify@3.2.7`** 经由 `monaco-editor@0.55.1`（`@monaco-editor/react@4.7.0` 间接依赖）引入。
  - 主要类型：DOMPurify XSS bypass、prototype pollution、mutation-XSS 等 (GHSA 系列)
  - **风险评估**：DOMPurify 在 monaco-editor 内部仅用于 hover preview HTML 的本地渲染（不接收远端用户输入），实际可利用面非常窄。但建议关注 monaco-editor 上游升级到 ≥ 0.56 时同步更新。
  - **行动**：在 `package.json` 加 `pnpm.overrides` 强制升 `dompurify` 到 ≥ 3.2.9，或等 monaco-editor 上游修复。
- **无新引入未经审计的依赖**（package.json 与 PR scope 一致）。

---

## 性能审查

- **列表 key**：合规。Skeleton 占位用 `key={i}` 是固定长度数组、不重排，安全；唯一一处生产代码 `script-test-panel.tsx:132` 的日志行也是 append-only。✅
- **虚拟滚动**：依赖 `@tanstack/react-virtual` 已声明但未在源码中实际调用。当前所有 PAGE_SIZE ≤ 50 的分页表格 + 桌面 dashboard 无 100+ 行同屏，暂可接受；待节点详情/audit 等场景规模上来后再接入。⚠️ 标 TODO 即可，不阻塞。
- **代码分割**：TanStack Router 的 `createFileRoute` 自动 per-route code split + i18n namespace 按需 `addResourceBundle`，✅
- **React.memo / useMemo**：观察 `template-editor.tsx`、`pipeline/canvas.tsx`、`node-table.tsx` 等大组件，props 都是引用稳定的回调，没有"在 render 内 `new` 对象作 prop"的坑。✅
- **AppShell 的 grid 布局** 用 inline `style` + `gridTemplateAreas`，每次 render 都新建对象——AppShell 渲染频率低，可忽略，但理论上可提到模块顶 const。

---

## API 契约一致性（抽样 5 个 DTO）

| DTO | TS `web/src/types/api.ts` | Go `internal/types/api.go` | 一致性 |
|-----|--------------------------|--------------------------|--------|
| `User` | line 149-159 | line 242-252 | ✅ 字段、类型完全一致 |
| `UserPublicProfile` | line 161-169 | line 255-263 | ✅ |
| `Subscription` | line 260-279 | line 370-389 | ✅ 19 字段对齐 |
| `SubscriptionDetail` | line 288-292 | line 396-402 | ❌ **TS 缺 `share_token`**（Go 有，TS 在 `api/subscription.ts:33-39` 通过本地 extends 补，但 `types/api.ts` 自身未同步） |
| `Node` / `NodeWithLatency` | line 335-356 | line 458-481 | ✅ `parsed_config` Go 是 `any` 对应 TS `Record<string, unknown>`，OK |

**问题**：`SubscriptionDetail.share_token` 与 `RotateShareTokenResponse` 在 codegen 产物 `types/api.ts` 中缺失，目前通过 `api/subscription.ts` 临时 extends 弥补——属于 §3.3"禁止前后端各自维护类型"的违规。**建议**：本次 PR 不阻塞，但应在下一个 sprint 把 codegen 跑起来或手动 sync 一次。

---

## 优点

- **错误处理统一**：`apiFetch` 把所有非 2xx 包装成 `ApiError`，401 自动清 store + 全局重定向 + `next` 参数携带，链路完整、无散落 `try/catch`。
- **auth 流程严谨**：`/login` 与 `/totp` 的 `sanitizeNext` 都校验 same-origin，防止 open-redirect；`twoFactorPending` 用 `partialize` 排除持久化，符合"过 tab 即失效"的预期。
- **设计 token 化彻底**：除上面列出的边角，绝大多数组件用 `bg-[var(--color-*)]` / `text-[var(--color-*)]` / `rounded-[var(--radius-*)]` 形式调 token，主题切换 / 暗色优先策略可以真正生效。
- **i18n parity 维护到位**：16 个 namespace × 4 套 locale 经 `jq` 对比 key 集合完全一致；`docs/_lint-violations.md` 显示 T-34 已闭环。
- **严格 TS**：业务代码（排除 `routeTree.gen.ts` 自动生成）grep 不到任何 `: any` / `as any` / `<any>`，对 `unknown` 的 narrow 写得规整（`api-client.ts`、`use-event-stream.ts` 均是好例子）。
- **架构分层清晰**：`api/`（hook + URL）、`hooks/`（通用 hook）、`lib/`（纯工具）、`stores/`（zustand）、`components/`（UI / 业务）、`types/`（契约）边界清晰，无环依赖。

---

## 结论

⚠️ **需修改后通过**

阻断项：5 个 bug（裸 hex / 死分支三元 / 硬编码文案）必须在合入前修复。
非阻断项：8 项视觉与契约 style 改进可以分 ticket 跟进。
依赖 8 个 moderate 漏洞全在 monaco-editor 间接依赖，建议加 `pnpm.overrides` 强制升 dompurify，但不阻塞本次合入。
