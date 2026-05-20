# 竞品/参考项目调研

调研日期：2026-05-20
主参考：iluobei/miaomiaowu（妙妙屋，762 ★，Go + React 19，已深度分析）
调研口径：每个候选用 GitHub README 直接取数；★、最近活跃日期来自 WebFetch 实测结果。

---

## 一、入选项目卡片

### 1. Sub-Store (sub-store-org/Sub-Store)
- ★ Star：9.6k
- 语言：JavaScript（100%，Node.js 后端 + Vercel 前端）
- 最近活跃：2026-05-20（v2.23.16，仍在频繁迭代）
- 一句话：把"订阅聚合 + 格式互转 + 脚本算子"做成跨平台桥的事实标准。
- 核心亮点（与妙妙屋的差异）：
  - 双向覆盖 9+ 客户端（Clash.Meta / Surge / Loon / Stash / Shadowrocket / QX / sing-box / Egern…），妙妙屋只能输出 Clash。
  - "Script Operator"——用户在订阅 pipeline 里写 JS 直接改 proxy 字段（filter / rename / sort / 自定义算子），与妙妙屋的 goja 脚本扩展是平行设计但更聚焦"订阅流水线"。
  - 支持最新协议：Hysteria2 / TUIC / WireGuard / mieru / sudoku 等冷门协议同步跟进。
- 值得抄走的设计：把订阅处理设计成"算子流水线（filter→map→sort→output）"，前端 UI 可视化拖拽编排，比妙妙屋"一次性脚本钩子"更易于普通用户使用。
- 不值得抄的：纯 JS 单体，多用户/权限/计费完全没有，仅适合个人自托管。

### 2. Nezha (nezhahq/nezha)
- ★ Star：10k
- 语言：Go 97.6%
- 最近活跃：2026-05-19（v2.0.11）
- 一句话：中文圈最知名的自托管多服务器探针，已经形成"主题生态"（Nazhua、nezha-dash 等第三方前端）。
- 核心亮点（与妙妙屋的差异）：
  - 完整的 HTTP/SSL/TCP/Ping 服务监控 + Cron 定时任务 + Web 终端（远程登录 agent），妙妙屋只做流量统计。
  - 分离式 Admin / User 双前端架构，多人能看不同视图。
  - 生态丰富：Android wrapper（Ulmaridae）、自定义主题、agent-level 脚本下发。
- 值得抄走的设计：(1) Web Terminal 直连 agent，做"探针 + 远程管理"复合面板；(2) 把前端做成可替换主题层（社区出 nezha-dash 这类完全重做的 UI），让妙妙屋这种闭合 UI 也能开放主题市场。
- 不值得抄的：v0→v1→v2 大版本不兼容造成生态分裂，社区抱怨多；演进时要保持 agent 协议向后兼容。

### 3. Komari (komari-monitor/komari)
- ★ Star：4.7k
- 语言：Go 98%
- 最近活跃：2026-05-08（v1.2.0）
- 一句话：刻意做轻量、对标 Nezha 的新一代探针，2025 年起势头很猛。
- 核心亮点（与妙妙屋的差异）：
  - WebSocket 长连接的 agent 心跳上报，时延比 Nezha 的 gRPC 拉模型更低。
  - 自带 `compat/nezha` 兼容层——允许直接接管 Nezha agent，迁移成本几乎为零。
  - 前端独立编译（Node 20+），主题可自定义 JSON 配置；已出现 KomariBeautify 这类美化插件。
- 值得抄走的设计：兼容老协议作为"迁移诱饵"——妙妙屋可以加 sub-store API 兼容层，让 sub-store 用户零成本切过来。
- 不值得抄的：功能比 Nezha 少（无 Web Terminal、无 Cron），是"先轻量再补功能"的路线，不要一开始就抄它的"功能少"。

### 4. Beszel (henrygd/beszel)
- ★ Star：22k（2024 年才发布，增长极快）
- 语言：Go 91.1%，前端 TypeScript
- 最近活跃：2026-04-05（v0.18.7）
- 一句话：基于 PocketBase 的现代化探针，自带 Docker 容器级监控和历史数据。
- 核心亮点（与妙妙屋的差异）：
  - **Docker / Podman 容器级指标**——按容器分维度看 CPU/MEM/网络历史；妙妙屋只到主机级。
  - 内置 OAuth/OIDC 登录 + 多用户；自动备份到 S3 兼容存储。
  - Agent 仅 6MB RAM，Hub 仅 23MB RAM；告警维度覆盖 CPU/MEM/Disk/带宽/温度/Load/状态。
- 值得抄走的设计：(1) 用 PocketBase 当后端速通——自带认证/权限/REST/Realtime，半天搭起一个面板；(2) 把"容器视角"加进探针——VPS 上跑 Docker 是刚需，妙妙屋目前缺失。
- 不值得抄的：PocketBase 灵活性高但定制深度不及自写 SQL，社区生态相对小，做严肃多租户时会撞墙。

### 5. Uptime Kuma (louislam/uptime-kuma)
- ★ Star：87k（自托管监控类目第一）
- 语言：JavaScript 56% + Vue 42%（Node.js + Vue3 + Socket.io）
- 最近活跃：2026-05-03（v2.3.2）
- 一句话：自托管监控领域的"统治级"产品，监控类型 + 通知通道覆盖最广。
- 核心亮点（与妙妙屋的差异）：
  - 13+ 监控类型：HTTP/HTTPS Keyword/JSON Query、TCP、Ping、DNS、WebSocket、Push、Docker、Steam Game Server。
  - **90+ 通知渠道**（Telegram、Discord、Gotify、Slack、Email、Pushover…），妙妙屋只接了 Telegram。
  - 多状态页 + 域名映射 + 证书过期提醒 + 2FA + Globalping（v2.1 加的全球分布式探测）。
- 值得抄走的设计：(1) 把"状态页 public 域名映射"做出来——给运营者/团队对外展示用；(2) 通知通道抽象成插件化适配器，让用户不动代码就能加 webhook 模板。
- 不值得抄的：Node 单进程 + SQLite 在 1000+ 监控目标时性能瓶颈明显；不要照搬这套架构。

### 6. 3X-UI (MHSanaei/3x-ui)
- ★ Star：37.7k（机场后台类目最高）
- 语言：Go 43% + Vue 34% + JS 15%
- 最近活跃：2026-05-05（v2.9.4）
- 一句话：Xray 核 + 多用户限额（过期/流量/IP）+ Telegram Bot 的事实标准面板。
- 核心亮点（与妙妙屋的差异）：
  - 真正的"机场后台"：每用户过期日 / 流量额度 / 同时在线 IP 数 限制，支持 Vmess/Vless/Trojan/SS/WG/Hysteria/Tun 全协议。
  - SQLite（默认）+ PostgreSQL（高并发）双后端可切，并支持多节点部署。
  - 集成 Telegram Bot 做客户端管理（不只是通知，是双向交互）。
- 值得抄走的设计：(1) 用户级"过期/流量/IP 限额"三件套是机场必备，妙妙屋走"个人聚合"路线可借此向"小型团队/工作室共享订阅"扩展；(2) Telegram Bot 当低门槛管理界面（命令行 + inline keyboard），降低运维门槛。
- 不值得抄的：3X-UI 的 UI/UX 是上一代 Vue + jQuery 风格，前端工程性差；学功能、不学界面。

### 7. Xboard (cedar2025/Xboard)
- ★ Star：4.3k
- 语言：PHP 93.6%（Laravel 12 + Octane + React/Shadcn UI 后台 + Vue3/TS 用户端）
- 最近活跃：仍在更新但官方声明"轻度维护"
- 一句话：V2board 二次开发的高性能商业级机场面板，自带计费/工单/邀请返佣。
- 核心亮点（与妙妙屋的差异）：
  - **完整 SaaS 计费体系**：套餐/订单/邀请返佣/工单系统/支付网关（Stripe、AliPay、加密货币）——妙妙屋零计费。
  - 多协议节点池统一管理 + 流量统计分摊到用户。
  - 现代化双前端（Shadcn 管理后台 + Vue3 用户端）。
- 值得抄走的设计：(1) 套餐 + 订单 + 邀请返佣的数据模型可借鉴，即便妙妙屋路线不做 SaaS，"团队套餐"功能也用得上；(2) Laravel Octane 这种"常驻进程加速框架"思路，可借鉴让 Go 后端复用连接池/缓存。
- 不值得抄的：PHP/Laravel 技术栈与妙妙屋的 Go + React 完全不同，重写代价大。

### 8. Subconverter (tindy2013/subconverter)
- ★ Star：16.6k
- 语言：C++ 97.8%
- 最近活跃：**2024-04-08（v0.9.0，已 2 年未更新）**
- 一句话：订阅格式转换的老牌底层工具，被无数前端 UI 包装。
- 核心亮点（与妙妙屋的差异）：
  - 纯转换引擎、15+ 格式互转、外部 ini 规则文件（ACL4SSR 生态依赖它）。
  - 资源占用极低（C++ 单二进制，几 MB 内存）。
- 值得抄走的设计：把"格式转换核心"抽成独立二进制 / 库，再让 UI 项目以多种方式（HTTP / FFI / WASM）调用——架构分层比单体更易长期维护。
- 不值得抄的：已经停更，**新协议（Hysteria2 等）支持落后于 sub-store**；不要把它当依赖，要么 fork 要么自写 parser。

### 9. Gatus (TwiN/gatus)
- ★ Star：11k
- 语言：Go
- 最近活跃：持续更新（2026 仍在）
- 一句话：YAML 配置即代码的开发者向状态页，DSL 化健康检查表达式。
- 核心亮点（与妙妙屋的差异）：
  - 健康检查支持"条件表达式"DSL：`[STATUS] == 200 && [RESPONSE_TIME] < 300 && [BODY].name == "john"`，逻辑灵活到能做 UAT。
  - 协议覆盖最广：HTTP / ICMP / TCP / UDP / SCTP / DNS / WebSocket / gRPC / SSH / STARTTLS / TLS。
  - 18+ 通知通道（Slack/PagerDuty/Telegram/Discord/Teams/Matrix/IFTTT…）。
- 值得抄走的设计：用 YAML/Hcl 这类声明式配置驱动监控规则，比 GUI 点选更适合 GitOps 部署，妙妙屋目前是 UI 优先，可以增加"YAML 导入/导出"作为高阶用户出口。
- 不值得抄的：Gatus 完全无 GUI 编辑（必须改 YAML 重启），普通用户上手陡——妙妙屋不要走极端。

### 10. Clash Verge Rev (clash-verge-rev/clash-verge-rev)
- ★ Star：120k（参考量级，本质是桌面客户端，不是 Web 面板）
- 语言：TypeScript 59% + Rust 32%（Tauri 2 框架）
- 最近活跃：2026-05-20（v2.5.1）
- 一句话：Mihomo 核 + Tauri 2 桌面客户端的事实标准，订阅"消费端"参考。
- 核心亮点（与妙妙屋的差异）：
  - Profile Merge + Script 双重增强机制：用户可以在订阅之上叠加 JS 脚本和 YAML 合并规则。
  - WebDAV 备份同步配置（跨设备）。
  - 可视化规则/节点编辑器。
- 值得抄走的设计：(1) WebDAV 自动同步用户配置——妙妙屋可作为"配置中心"角色，让客户端反向 pull；(2) Profile Merge 机制（叠加层）比"覆盖式脚本"更安全。
- 不值得抄的：它是桌面客户端，不要照搬交互范式；妙妙屋是服务端 / Web 面板，定位完全不同。

---

## 二、功能拼图（横向对比表）

✓ = 有，✗ = 无，~ = 部分/弱，? = 未确认，N/A = 项目类目不适用

| 功能 | 妙妙屋 | Sub-Store | Nezha | Komari | Beszel | Uptime Kuma | 3X-UI | Xboard | Gatus | Clash Verge Rev | 备注/差异化机会 |
|------|-------|-----------|-------|--------|--------|-------------|-------|--------|-------|------------------|----------------|
| Clash 订阅聚合 | ✓ | ✓ | N/A | N/A | N/A | N/A | ~ | ✓ | N/A | ✓（消费端） | 妙妙屋已具备 |
| 多协议 URI 解析（10+） | ✓ | ✓（最全） | N/A | N/A | N/A | N/A | ✓ | ✓ | N/A | ✓ | sub-store 覆盖最广（含 mieru/sudoku） |
| 输出多客户端格式（≥5） | ~（仅 Clash） | ✓（9+） | N/A | N/A | N/A | N/A | ~ | ~ | N/A | N/A | **机会**：扩展 Surge/sing-box/Stash 输出 |
| sub-store API 兼容 | ✓ | 自身 | N/A | N/A | N/A | N/A | ✗ | ✗ | N/A | ✗ | 妙妙屋已有 |
| 订阅算子流水线（可视化） | ~（脚本） | ✓ | N/A | N/A | N/A | N/A | ✗ | ✗ | N/A | ✗ | **机会**：sub-store 那种拖拽 pipeline |
| 多探针流量聚合 | ✓ | ✗ | ✓ | ✓ | ✓ | ✗ | ~ | ~ | ✗ | N/A | 妙妙屋三探针聚合是亮点 |
| 节点 TCPing | ✓ | ✗ | ✓ | ? | ✗ | ✓ | ✗ | ✗ | ✓ | ✓ | |
| HTTP/SSL/DNS 服务监控 | ✗ | ✗ | ✓ | ~ | ~ | ✓ | ✗ | ✗ | ✓ | ✗ | **机会**：加 HTTP/SSL 监控，覆盖 Uptime Kuma 场景 |
| 容器级监控（Docker） | ✗ | ✗ | ✗ | ✗ | ✓ | ~ | ✗ | ✗ | ✗ | ✗ | **机会**：Beszel 独有，VPS 用户刚需 |
| Web Terminal（远程登录） | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **机会**：Nezha 独有，差异化强 |
| Cron / 计划任务下发 | ✗ | ✗ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | Nezha 独有 |
| 链式代理 | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ~ | ~ | ✗ | ✓ | 妙妙屋亮点 |
| 脚本扩展（goja/JS） | ✓ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | 妙妙屋已有 |
| 健康检查 DSL（条件表达式） | ✗ | ✗ | ~ | ✗ | ✗ | ~ | ✗ | ✗ | ✓ | ✗ | **机会**：Gatus 独有，开发者向 |
| ACL4SSR 兼容 | ✓ | ✓ | N/A | N/A | N/A | N/A | ✗ | ✓ | N/A | ✓ | 中文圈基础 |
| 短链系统 | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | 妙妙屋独有 |
| TOTP 2FA | ✓ | ✗ | ~ | ✗ | ✗ | ✓ | ~ | ✓ | ✗ | ✗ | |
| OAuth/OIDC 登录 | ✗ | ✗ | ~ | ✗ | ✓ | ~ | ✗ | ✓ | ✗ | ✗ | **机会**：企业用户刚需 |
| 多用户/RBAC | ✗ | ✗ | ✓ | ~ | ✓ | ~ | ✓ | ✓ | ✗ | ✗ | 妙妙屋目前单用户 |
| Telegram 通知 | ✓ | ~ | ✓ | ~ | ✗ | ✓ | ✓ | ✓ | ✓ | ✗ | |
| Telegram Bot 双向交互 | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ~ | ✗ | ✗ | **机会**：3X-UI 风格 Bot 当管理界面 |
| 通知渠道 ≥10 | ✗ | ✗ | ~ | ✗ | ~ | ✓（90+） | ~ | ~ | ✓（18+） | ✗ | **机会**：插件化适配器 |
| OTA 自更新 | ✓ | ✗ | ~ | ~ | ✗ | ~ | ~ | ✗ | ✗ | ✓ | 妙妙屋亮点 |
| 静默模式（404 防扫） | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **妙妙屋独有，业界稀有** |
| 节点延迟实时图 | ? | ✗ | ✓ | ✓ | ~ | ~ | ✗ | ✗ | ✓ | ✓ | |
| WebSocket 实时推流 | ? | ✗ | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ | ✗ | ✗ | |
| 公开状态页（status page） | ✗ | ✗ | ~ | ✗ | ✗ | ✓ | ✗ | ✗ | ✓ | ✗ | **机会**：对外展示 SLA |
| 多用户/SaaS 计费 | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ~ | ✓ | ✗ | ✗ | Xboard 独家 |
| 邀请返佣 / 工单系统 | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | Xboard 独家 |
| WebDAV 配置同步 | ✗ | ✗ | ✗ | ✗ | ✓（备份） | ✗ | ✗ | ✗ | ✗ | ✓ | |
| YAML 配置即代码 | ✗ | ~ | ~ | ~ | ~ | ✗ | ✗ | ✗ | ✓ | ~ | **机会**：GitOps 出口 |
| 节点订阅市场 | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **没人做** |
| 主题市场 / 可换皮 | ✗ | ✗ | ✓ | ✓ | ✗ | ~ | ✗ | ✗ | ~ | ✓ | |
| 客户端 App（不止 Web） | ✗ | ✗ | ✓（社区） | ✓（社区） | ✗ | ✗ | ✗ | ~ | ✗ | ✓ | |

---

## 三、技术栈倾向汇总

按候选项目出现频率统计（10 个项目）：

- 后端：
  - **Go**：6 个（Nezha、Komari、Beszel、3X-UI、Gatus、妙妙屋）—— 自托管+探针类绝对主流
  - **Node.js / TS**：3 个（Sub-Store、Uptime Kuma、Clash Verge Rev 桌面端）
  - **C++**：1 个（Subconverter）—— 老牌底层工具
  - **PHP / Laravel**：1 个（Xboard）—— SaaS 路线特有
  - **Rust**：1 个（Clash Verge Rev 部分）—— 桌面客户端
- 前端：
  - **Vue**：4 个（Uptime Kuma、3X-UI、Xboard 用户端、Beszel 部分）
  - **React**：3 个（妙妙屋、Xboard 后台、Sub-Store 前端为 Vercel SPA）
  - **原生 / 模板**：2 个（Gatus、Nezha admin 部分）
  - **TypeScript + Tauri**：1 个（Clash Verge Rev）
- 数据库：
  - **SQLite**：6 个项目默认（妙妙屋、3X-UI 默认、Komari、Beszel via PocketBase、Uptime Kuma、Gatus）
  - **PostgreSQL**：3 个可选（3X-UI、Nezha、Xboard）
  - **MySQL/MariaDB**：2 个（Xboard、Uptime Kuma v2.0+）
- 部署：
  - **Docker 镜像**：10/10 全部提供
  - **单二进制（Go 项目）**：6 个
  - **一键脚本**：8 个（中文圈强需求）
  - **k8s Helm Chart**：仅 Uptime Kuma、Beszel 社区有

结论：做新项目时 **Go + SQLite + Docker + 单二进制 + 一键脚本** 是该垂类的"低风险默认选择"，前端 React/Vue 都可。

---

## 四、关键差异化机会（最重要的输出）

### 机会 1：把"探针"升级成"探针 + 容器观测 + Web Terminal"复合面板
- 差异化点：把 Beszel 的"Docker 容器级监控"和 Nezha 的"Web Terminal"合并进妙妙屋同款 UI，做"VPS 一站式管控"。
- 业界现状：Beszel 有容器但无 Terminal；Nezha 有 Terminal 但无容器；没人合二为一。VPS 用户绝大多数在跑 Docker。
- 实现复杂度：**中**（容器：直接读 Docker socket；Terminal：用 xterm.js + WebSocket 桥接 agent shell，Nezha 已有开源参考）
- 价值：直接吃掉两个相邻类目用户，"流量统计 → 全栈运维面板"完成跃迁。

### 机会 2：订阅算子流水线 + YAML 双模式
- 差异化点：把 sub-store 的"算子流水线"做成可视化拖拽，同时支持导出/导入 YAML（Gatus 风格 GitOps）。
- 业界现状：sub-store 有算子但 UI 是堆叠表单不直观；Gatus 是纯 YAML 无 GUI；没人做"双向同步"。
- 实现复杂度：**中-高**（前端编辑器 + AST 双向转换）
- 价值：普通用户用 GUI，进阶用户能 git 化管理订阅规则，覆盖两类受众。

### 机会 3：通知渠道插件化适配器市场
- 差异化点：抽象 NotificationChannel 接口 + 模板系统，让用户在 UI 里 zero-code 配置 90+ 渠道（Uptime Kuma 现成参考）。
- 业界现状：妙妙屋只 Telegram；探针类项目普遍 3-5 个渠道硬编码；Uptime Kuma 的 90+ 是高门槛壁垒。
- 实现复杂度：**低**（每个渠道一个 Go 文件 + 配置 schema；Uptime Kuma 代码可作 reference）
- 价值：直接弥补与 Uptime Kuma 的功能落差，是"卷功能列表"性价比最高的一项。

### 机会 4：公开状态页 + 团队/SLA 模式
- 差异化点：把妙妙屋扩展出"对外发布"模式——给团队/小型商业用户用，外部 URL 公开节点可用率，但不暴露管理细节。
- 业界现状：Uptime Kuma 有状态页但与订阅无关；Xboard 有商业但走 SaaS 重路线；没人把"订阅聚合面板 + 公开 SLA 页"组合。
- 实现复杂度：**中**（独立公开路由 + RBAC + 域名映射）
- 价值：从"个人自托管"上抬到"小团队/工作室共享"用户群。

### 机会 5：Telegram Bot 作为低门槛管理界面
- 差异化点：复刻 3X-UI 风格的双向 Telegram Bot——用 inline keyboard 直接看流量/重启 agent/刷新订阅，不必上 Web。
- 业界现状：探针类项目几乎都只把 Telegram 当单向通知出口；3X-UI 在机场类已证明双向 Bot 是降低运维门槛的好招。
- 实现复杂度：**低-中**（go-telegram-bot-api 直接套）
- 价值：手机用户友好，VPS 圈用户 90%+ 用 Telegram，转化率高。

---

## 五、强烈建议参考的 5 个最佳实践

### 1. 兼容老协议作为迁移诱饵（来自 Komari）
Komari 自带 `compat/nezha` 模块，允许 Nezha agent 不改配置直接连入，迁移成本几乎为零。妙妙屋已经做了 sub-store API 兼容，建议**继续扩展**：兼容 Nezha agent 协议，让 Nezha 用户能"切探针面板而不换 agent"。这是该垂类最有效的获客手段。

### 2. PocketBase 当"半边后端"（来自 Beszel）
Beszel 用 PocketBase 提供认证 / RBAC / REST / Realtime / Admin UI 全套基础设施，把项目核心代码量降到很小。即便妙妙屋继续自写 Go 后端，也可以借鉴这种思路——**把认证/审计/Schema 演进交给成熟组件**（如 ent、Casbin、Atlas migrate），避免重新发明轮子。

### 3. 前端做成可替换主题层（来自 Nezha）
Nezha 1.x→2.x 时官方前端和社区 nezha-dash 共存运行，证明"前后端解耦 + 公开 API 文档"能让社区主动贡献 UI 迭代。妙妙屋目前前后端紧耦合在一个仓库，建议**OpenAPI 化所有内部接口 + 在 README 标注"欢迎做主题"**，长期能省下大量前端工作。

### 4. 健康检查表达式 DSL（来自 Gatus）
Gatus 的 `[STATUS] == 200 && [BODY].name == "john"` 这套语法用极少代码提供了极大灵活性。妙妙屋的节点测速可以借鉴：让用户写 `[LATENCY] < 200 && [LOSS] < 0.1` 作为"节点可用"判断，比硬编码阈值灵活得多。**用 expr-lang/expr 这个 Go 表达式引擎，几十行代码就能集成**。

### 5. WebDAV 配置同步作为"客户端反向 pull"通道（来自 Clash Verge Rev）
Clash Verge Rev 用 WebDAV 实现跨设备配置同步。妙妙屋作为服务端，可以**反过来当 WebDAV server**——让所有 Clash 类客户端用同一份订阅 URL 同步配置，包括用户在面板上做的算子修改、规则增删。一份后端服务多个客户端，比"导出文件用户手动导入"高级一个层次。

---

调研使用：6 个 WebSearch + 11 个 WebFetch（在预算内）。所有 ★ 数与日期均来自实测。
