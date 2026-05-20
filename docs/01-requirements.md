# 拾光VPS 需求文档（PRD）

版本：v1.0
日期：2026-05-20
主参考：iluobei/miaomiaowu（妙妙屋）+ docs/_research-competitors.md
英文代号：shiguang-vps

---

## 1. 需求概述

**是什么**：拾光VPS 是面向个人 / 小团队的自托管 Web 面板，把 Clash 订阅聚合、多探针流量观测、节点管理、规则脚本扩展、多渠道通知整合到单一进程中。技术上以妙妙屋（762 ★）的功能集为基线，在"订阅算子流水线"和"通知系统"两处做深度差异化；运行形态为 Go 单二进制 + 内置 React 19 前端 + SQLite，可一键 Docker 或一键脚本部署。

**价值**：把原本散落在 sub-store（订阅）、Nezha（探针）、Uptime Kuma（通知）三处的能力收敛到一份 < 30MB 的二进制里，部署成本数倍降低；同时凭借 Nezha agent 协议兼容层吃下 Nezha 用户的迁移诉求，凭借 sub-store API 兼容层吃下 sub-store 用户的迁移诉求，**做这两个垂类的"低门槛聚合替代品"**。

## 2. 目标用户

- **主要用户**：个人技术爱好者 / 小机场主 / 自托管 Clash 订阅的极客（~70% 用户量）。
- **次要用户**：5-10 人技术小团队（共享一个面板做内部代理 / 监控，~20% 用户量）。
- **关联用户**：从 sub-store / Nezha 迁移过来的现有用户（~10%，是早期种子）。

**痛点**：

1. **工具碎片化**：订阅 / 探针 / 通知分别用 3 个独立项目，要管 3 套部署、3 套登录、3 套备份。
2. **国内网络环境不友好**：原生 Nezha / Beszel 等是英文界面，部分通知渠道（Bark / Server酱 / PushDeer）国内独有但无人接入。
3. **未授权扫描骚扰**：Clash 面板挂公网即被扫，常规防御暴露后台特征。
4. **多用户共享困难**：妙妙屋单用户模型让"小团队共享面板"必须共账号，无审计能力。
5. **配置无法 git 化**：订阅规则、自定义脚本只能在 GUI 改，对喜欢声明式 / GitOps 的用户不友好。

## 3. 用户故事

按角色分组，所有故事按 P0 / P1 / P2 标注优先级。

### admin 视角

1. **作为 admin，我希望** 首次部署时面板自动生成 admin 账号 + 强密码并打印到日志，**以便** 我无需手动初始化。 **(P0)**
2. **作为 admin，我希望** 创建 / 删除 user 账号，重置 user 密码，**以便** 我能管理小团队成员。 **(P0)**
3. **作为 admin，我希望** 强制为自己和所有用户启用 TOTP 2FA，**以便** 防止账号被爆破。 **(P0)**
4. **作为 admin，我希望** 看到全系统的探针 / 订阅 / 流量总览，**以便** 我能掌握整个面板的负载。 **(P0)**
5. **作为 admin，我希望** 配置系统级 SMTP / Webhook 等通知通道并下发给所有 user，**以便** 减少重复配置。 **(P1)**
6. **作为 admin，我希望** 看到所有用户的登录日志 / 操作审计，**以便** 排查异常行为。 **(P1)**
7. **作为 admin，我希望** 在面板里一键触发 OTA 自更新，**以便** 升级到最新版本而无需 ssh。 **(P0)**
8. **作为 admin，我希望** 开关静默模式（默认开），**以便** 防扫描的同时保留逃生舱。 **(P0)**
9. **作为 admin，我希望** 看到每个 agent 的版本号与 hub 是否兼容，**以便** 主动推 agent 升级。 **(P1)**
10. **作为 admin，我希望** 全量备份数据库 + 配置到本地 / S3，**以便** 灾难恢复。 **(P1)**

### user 视角

11. **作为 user，我希望** 导入外部订阅 URL，系统自动拉取并解析节点列表，**以便** 集中管理多个机场的订阅。 **(P0)**
12. **作为 user，我希望** 上传一份 Clash YAML 文件作为订阅，**以便** 把本地的 yaml 也纳入面板管理。 **(P0)**
13. **作为 user，我希望** 手动创建订阅并逐节点添加，**以便** 自建小规模代理。 **(P0)**
14. **作为 user，我希望** 系统能解析 vmess / vless / ss / ssr / trojan / hysteria / hysteria2 / tuic / wireguard / anytls / socks5 / naive 等 12+ 种 URI，**以便** 不挑订阅来源。 **(P0)**
15. **作为 user，我希望** 我之前 sub-store 配置的客户端 URL 不改也能用，**以便** 零成本迁移。 **(P0)**
16. **作为 user，我希望** 在订阅之上拖拽配置算子流水线（filter / map / sort / dedupe / regex-rename / output），**以便** 加工节点列表。 **(P0)**
17. **作为 user，我希望** 把流水线导出为 YAML 文件 commit 到 git，未来再导回，**以便** GitOps 化管理。 **(P0)**
18. **作为 user，我希望** 在流水线编辑器里点"预览"看每个算子前后的节点 diff，**以便** 调试。 **(P1)**
19. **作为 user，我希望** 对单节点或批量节点做 TCPing 测延迟，**以便** 筛掉不可用节点。 **(P0)**
20. **作为 user，我希望** 用链式代理串接两个节点作为出口，**以便** 实现跳板场景。 **(P1)**
21. **作为 user，我希望** 在面板写 JS 脚本（goja）当 pre_save_nodes 或 post_fetch hook，**以便** 算子不够时灵活定制。 **(P1)**
22. **作为 user，我希望** 配置 custom_rules（DNS / rules / rule-providers，replace/prepend/append 三模式），**以便** 注入到最终 Clash 配置。 **(P0)**
23. **作为 user，我希望** 部署一个轻量 agent 到我的 VPS，**以便** 看到该机器的 CPU / 内存 / 流量。 **(P0)**
24. **作为 user，我希望** 我已经在跑的 Nezha agent 改个 server 地址就能接入拾光VPS，**以便** 零迁移成本。 **(P0)**
25. **作为 user，我希望** 看到每月已用流量趋势图，按计费周期重置，**以便** 监控限额。 **(P0)**
26. **作为 user，我希望** 在 Telegram / Discord / Bark 等 10+ 渠道接收通知（节点离线 / 流量告警 / 同步失败等），**以便** 不必盯面板。 **(P0)**
27. **作为 user，我希望** 通过 Telegram Bot 直接 `/nodes` 或 `/refresh` 命令操作，**以便** 在手机上运维。 **(P1)**
28. **作为 user，我希望** 把面板界面切到 en / ja / ko，**以便** 与团队中的非中文成员协作。 **(P1)**
29. **作为 user，我希望** 把长订阅 URL 生成短链分发给客户端，**以便** 配置简洁。 **(P1)**
30. **作为 user，我希望** 启用 2FA 时拿到 8 位 hex 备份码列表，**以便** 设备丢失时救援。 **(P0)**

## 4. 功能清单（按模块分组）

模块编号约定：M-XXX，所有功能在第 5 节有对应验收标准。

### 4.1 用户与权限模块（M-USER）

- M-USER-1：admin / user 二分角色（详 ADR 0002）。
- M-USER-2：用户名 + bcrypt 密码登录，cost=10。
- M-USER-3：TOTP 2FA 启用 / 验证 / 关闭，pquerna/otp 实现。
- M-USER-4：备份码生成（首次启用 2FA 时生成 N 个 8 位 hex，sha256 存库，一次性消耗）。
- M-USER-5：登录限速 5 次/小时（双维度：IP + 账号）。
- M-USER-6：暴力破解防护 20 次/10 分钟封 IP 1 小时。
- M-USER-7：Session 管理（默认 24h TTL，可在系统设置调）。
- M-USER-8：admin 创建 / 删除 user，重置 user 密码。
- M-USER-9：用户自助改密、改用户名、删除账号。
- M-USER-10：操作审计日志（admin 可见）。**(P1)**

### 4.2 订阅管理模块（M-SUB）

- M-SUB-1：导入外部订阅 URL（HTTP/HTTPS，可带 UA 自定义）。
- M-SUB-2：上传 Clash YAML 文件作为订阅。
- M-SUB-3：手动创建订阅 + 逐节点添加。
- M-SUB-4：多协议 URI 解析共 12 种：vmess / vless / ss / ssr / trojan / hysteria / hysteria2 / tuic / wireguard / anytls / socks5 / naive。
- M-SUB-5：sub-store API 兼容层（暴露 `/download/:name` 等路由，让 sub-store 客户端无改动迁移）。
- M-SUB-6：ACL4SSR 兼容（解析 subconverter 风格的 ini / list 文件）。
- M-SUB-7：订阅自动更新周期（可配，默认 6h）。
- M-SUB-8：订阅元数据：标签、过期时间、流量额度、备注。
- M-SUB-9：订阅订阅同步失败时触发通知（联动 M-NOTIFY）。

### 4.3 算子流水线模块（M-PIPE）★差异化 #1（详 ADR 0004）

- M-PIPE-1：算子库（v1 六种）：filter / map / sort / dedupe / regex-rename / output。
- M-PIPE-2：UI 拖拽编排（@dnd-kit）：左边算子库、中间画布、右边参数面板。
- M-PIPE-3：YAML 导出 + 导入，schema `apiVersion: shiguang/v1`。
- M-PIPE-4：GUI ↔ YAML 实时双向同步。
- M-PIPE-5：调试预览（点击"运行"看每个算子前后节点 diff）。
- M-PIPE-6：流水线挂载到订阅，运行时序：`原始节点 → op1 → op2 → ... → 最终节点`。
- M-PIPE-7：性能：100 节点全流水线 < 500ms。

### 4.4 节点管理模块（M-NODE）

- M-NODE-1：节点 CRUD，按订阅过滤。
- M-NODE-2：节点标签（tag）多对多。
- M-NODE-3：批量 TCPing 测延迟，200 节点并发 < 5s。
- M-NODE-4：链式代理（dialer-proxy 字段）。**(P1)**
- M-NODE-5：节点搜索 + 排序（按延迟 / 标签 / 协议）。
- M-NODE-6：节点详情面板（含 raw URI 查看 / 复制）。

### 4.5 规则系统模块（M-RULE）

- M-RULE-1：custom_rules 表，三类：dns / rules / rule-providers。
- M-RULE-2：三种 mode：replace（全替换）/ prepend（前置插入）/ append（后置追加）。
- M-RULE-3：规则模板预设（如"国内 直连 + 国外 代理"基础模板）。**(P1)**
- M-RULE-4：规则生效预览（最终 Clash 配置 yaml 即时渲染）。

### 4.6 脚本扩展模块（M-SCRIPT）

- M-SCRIPT-1：goja JS 沙箱执行。
- M-SCRIPT-2：两个 hook：pre_save_nodes（入库前加工节点列表）、post_fetch（订阅拉回原始内容预处理）。
- M-SCRIPT-3：单脚本执行 5s 超时，超时强制 kill。
- M-SCRIPT-4：沙箱无网络 / 无文件系统访问能力（仅纯计算）。
- M-SCRIPT-5：脚本错误日志可在 UI 查看。

### 4.7 探针 agent 模块（M-AGENT）（详 ADR 0003）

- M-AGENT-1：自写 Go agent，单二进制 < 10MB，跨平台（linux amd64/arm64、macOS、Windows）。
- M-AGENT-2：WebSocket 长连接，默认心跳 30s（可配 5-300s）。
- M-AGENT-3：上报字段：CPU / MEM / Swap / Disk / NetIO / Load 1/5/15 / TCP/UDP 连接数 / Uptime。
- M-AGENT-4：token + TLS 鉴权（token 错误返 404，符合静默模式）。
- M-AGENT-5：Nezha agent v2 协议兼容端点 `/api/v1/nezha/heartbeat`（最小字段子集，详见 ADR 0003 附录 A）。
- M-AGENT-6：agent 版本号上报；hub 端比较版本提示升级。
- M-AGENT-7：agent 自更新通路（与 hub OTA 解耦）。**(P1)**
- M-AGENT-8：agent 内存稳态 < 30MB，CPU 静态占用 < 1%。

### 4.8 流量聚合模块（M-TRAFFIC）

- M-TRAFFIC-1：agent_records 高频原始记录（默认 7 天保留窗口）。
- M-TRAFFIC-2：日聚合任务（每天 00:00 跑），写入 traffic_records 长期表。
- M-TRAFFIC-3：月度计费周期可配（默认每月 1 号重置）。
- M-TRAFFIC-4：流量趋势图（按日 / 月，多探针 + 多订阅源汇总到一张图）。
- M-TRAFFIC-5：流量告警阈值（如本月已用 > 80% 触发通知）。

### 4.9 通知系统模块（M-NOTIFY）★差异化 #2（详 ADR 0005）

- M-NOTIFY-1：NotificationChannel 接口 + 10 渠道：Telegram / Discord / Slack / Email / Bark / Gotify / Webhook / Server酱 / PushDeer / IFTTT。
- M-NOTIFY-2：事件类型 opt-in：节点离线 / 流量告警 / 订阅同步失败 / 备份完成 / 登录异常 / OTA 升级 / 自定义脚本告警。
- M-NOTIFY-3：消息模板（Go template），用户自定义格式。
- M-NOTIFY-4：Telegram Bot 双向交互（inline keyboard）：`/nodes`、`/refresh <sub>`、`/agent_restart <id>`、`/traffic`、`/silent on|off`。
- M-NOTIFY-5：通知去抖（同事件 5 分钟内不重复发，可配）。

### 4.10 安全与运维模块（M-OPS）

- M-OPS-1：静默模式（默认开，详 ADR 0006）；登录页前缀 `/_app/<random-32hex>/`。
- M-OPS-2：OTA 自更新（GitHub Release 检测 + SHA-256 校验 + 优雅替换重启）。
- M-OPS-3：短链系统（fileCode + userCode 复合主键）。
- M-OPS-4：备份导出 / 恢复（数据库 + 配置，本地下载或 S3 上传）。**(P1)**
- M-OPS-5：/healthz endpoint（K8s 友好，可关）。
- M-OPS-6：结构化日志（slog json）+ 日志轮转 100MB / 7 天。

### 4.11 国际化模块（M-I18N）（详 ADR 0007）

- M-I18N-1：4 套 locale：zh-CN（默认）/ en / ja / ko。
- M-I18N-2：react-i18next 集成，命名空间按模块分。
- M-I18N-3：自动检测浏览器语言；用户偏好持久化到用户表。
- M-I18N-4：时间 / 数字格式按 locale 渲染（Intl）。
- M-I18N-5：CI lint 规则检查硬编码中文字符串。

### 4.12 P2 后期功能（明确标注，不在 v1 范围）

- M-P2-1：**P2 后期** Web Terminal（agent 已为此预留接口，调研报告"差异化机会 1"路线）。
- M-P2-2：**P2 后期** Docker 容器级监控（agent 端读 Docker socket）。
- M-P2-3：**P2 后期** 公开状态页（对外发布节点可用率 SLA 页）。
- M-P2-4：**P2 后期** WebDAV 配置同步（让 Clash 客户端反向 pull）。
- M-P2-5：**P2 后期** OAuth/OIDC 登录接入企业 SSO。
- M-P2-6：**P2 后期** 主题市场 / 可换皮（OpenAPI 化前端 + 社区主题）。

## 5. 验收标准

每条标号对应 §4 模块编号 + 子序号；每条都可被测试用例直接覆盖。至少 30 条。

### M-USER
- **M-USER.1**：首次启动后，日志包含 `Admin password: ******` + `Login URL: http://<host>:8080/_app/<32hex>/`，通过该 URL 能登录；任何其他路径返回 404。
- **M-USER.2**：连续 5 次错误密码登录后，第 6 次请求返回 429（同 IP 同账号 1 小时内）。
- **M-USER.3**：启用 2FA 后，关闭浏览器再访问，需先输入 TOTP 才能进入。
- **M-USER.4**：8 个备份码用一个少一个；用完后该码不可再用且不影响其他码。
- **M-USER.5**：admin 删除某 user 后，该 user 的所有订阅 / 规则 / 脚本被级联删除（或归档可配）。

### M-SUB
- **M-SUB.1**：导入 sub-store 风格 URL（如 `http://host:port/download/<name>`）后，节点数量与原订阅一致，每个节点的 server/port/uuid 字段完全匹配。
- **M-SUB.2**：上传一份含 vless+reality 节点的 yaml，输出 Clash 格式时该节点自动过滤（Clash 不支持 reality）；UI 同时给出 warning。
- **M-SUB.3**：12 种协议 URI 各取 1 个真实样例，导入后能正确解析出 server/port/auth；不支持的字段以 `_raw` 保留原文。
- **M-SUB.4**：sub-store 兼容路由 `/download/:name?token=xxx` 在 token 错误时返 404（静默模式联动）。
- **M-SUB.5**：订阅同步失败（如远端 5xx）后，5 分钟内触发"订阅同步失败"通知到所有 opt-in 渠道。

### M-PIPE
- **M-PIPE.1**：在 UI 拖入 filter→sort→output 三个算子，运行后产出的节点列表与等价 YAML 配置导入运行的结果**完全一致**（同序、同字段）。
- **M-PIPE.2**：导出 YAML 文件含 `apiVersion: shiguang/v1` 字段；重新导入后流水线在 UI 完整还原。
- **M-PIPE.3**：100 节点 + 6 算子流水线在 H/W = 2c4g VPS 上 < 500ms 完成。
- **M-PIPE.4**：调试预览面板展示每个算子前 / 后节点数变化、新增 / 删除 / 修改的节点 diff。

### M-NODE
- **M-NODE.1**：批量 TCPing 200 个节点（并发=50），整体 < 5s 完成；每节点返回 ms 数 + 是否可达。
- **M-NODE.2**：节点详情可复制 raw URI；复制后粘贴回"手动添加节点"输入框，解析结果与原节点一致。
- **M-NODE.3**：节点搜索框输入 `tag:hk`，列表只剩带 hk 标签的节点。

### M-RULE
- **M-RULE.1**：添加一条 rules 类型 prepend 规则 `DOMAIN-SUFFIX,example.com,DIRECT`，最终 Clash 配置的 `rules:` 段第一条就是该规则。
- **M-RULE.2**：把同一规则改为 replace 模式后，原 rules 段全部消失，只剩这一条。

### M-SCRIPT
- **M-SCRIPT.1**：写一段 5.1s 死循环的 pre_save_nodes 脚本，hub 在 5s 超时后杀掉、记录错误日志，并把节点保持原样入库。
- **M-SCRIPT.2**：脚本里 `require('fs')` 或 `fetch(...)` 调用抛出"not allowed in sandbox"错误。

### M-AGENT
- **M-AGENT.1**：agent 二进制单文件大小 < 10MB（linux amd64 release build）。
- **M-AGENT.2**：agent 静态运行 24h 内存稳态 < 30MB，CPU < 1%。
- **M-AGENT.3**：原 Nezha agent v2（不改 binary）改 server URL 后能连上拾光VPS hub，并在面板看到 CPU/MEM 上报。
- **M-AGENT.4**：agent token 错误时连接返回 404（静默模式联动），不暴露"token invalid"明文。

### M-TRAFFIC
- **M-TRAFFIC.1**：模拟 agent 上报 30 天数据后，traffic 趋势图按日 / 月切换显示正确累加值。
- **M-TRAFFIC.2**：本月已用流量超 80% 阈值后 5 分钟内触发"流量告警"通知。
- **M-TRAFFIC.3**：月度重置日（默认每月 1 号 00:00）触发后，"本月已用"归零，"上月已用"展示前月累计。

### M-NOTIFY
- **M-NOTIFY.1**：10 个渠道每个至少有一个真实测试用例（mock 服务器接收消息），收到的消息体符合各自 schema。
- **M-NOTIFY.2**：Telegram Bot 中点击 inline keyboard 的 `/nodes`，2s 内 Bot 回复当前节点列表（含延迟）。
- **M-NOTIFY.3**：连续触发 3 次相同 "节点离线" 事件，5 分钟去抖窗口内只发 1 条通知。

### M-OPS
- **M-OPS.1**：静默模式开启时，访问 `/`、`/admin`、`/api/login` 等路径全返回 404 + Server header 伪装为 nginx 默认；唯独 `_app/<前缀>/` 能进登录页。
- **M-OPS.2**：触发 OTA 自更新，hub 优雅停机 → 拉取新二进制 → SHA-256 校验通过 → 重启；数据库 wal_checkpoint 已执行，重启后无 WAL 残留文件。
- **M-OPS.3**：生成短链 `<host>/s/<fileCode><userCode>`，访问后 302 跳转到原长 URL。

### M-I18N
- **M-I18N.1**：切到 ja，所有 UI 文案换为日文；时间显示按 `ja-JP` 格式（如 2026/05/20 14:30）。
- **M-I18N.2**：CI 跑 lint 时，源码中出现硬编码中文字符串（非测试 / 非注释）会失败。
- **M-I18N.3**：首次访问无 Cookie 时，按 `navigator.language` 自动选择 locale；浏览器是 ko-KR 则默认韩文。

## 6. 非功能性需求

### 6.1 性能
- 单二进制冷启动 < 100ms。
- 内存稳态 < 50MB（不含 agent）。
- 单订阅同步（200 节点，网络正常）< 3s。
- 流水线 100 节点全跑完 < 500ms。
- TCPing 200 节点并发（并发=50）< 5s。

### 6.2 可观测性
- 结构化日志：slog json 输出 stdout。
- 日志轮转：100MB / 7 天滚动。
- agent 心跳频率：30s（可配 5-300s）。
- 内置 `/healthz` endpoint（可关）。

### 6.3 安全
- TOTP 2FA + 8 位 hex 备份码（sha256 存储，一次性消耗）。
- 默认 Session TTL 24h，可配。
- 静默模式：未授权全 404（详 ADR 0006）。
- 暴力破解：20 次/10 分钟封 IP 1 小时。
- 登录限速：5 次/小时 双维度（IP + 账号）。
- 密码：bcrypt cost=10。
- 所有用户输入正则编译失败不 panic（用 `regexp.Compile` + 错误回吐 UI）。
- agent ↔ hub：token 鉴权 + TLS。

### 6.4 国际化
- 所有界面文案走 react-i18next；CI lint 校验无硬编码中文。
- 默认 zh-CN；自动检测浏览器语言。
- 时间戳按用户时区显示。
- 数字 / 货币按 locale 格式化（Intl）。

### 6.5 兼容性
- Linux amd64 / linux arm64 / macOS amd64+arm64 / Windows amd64 五种二进制。
- Docker 镜像：linux/amd64 + linux/arm64 双架构。
- 浏览器：Chrome 110+ / Edge 110+ / Firefox 115+ / Safari 16+。

## 7. 边界与约束

### 7.1 不做什么（明确划清边界）
- **不做 SaaS 计费 / 工单 / 邀请返佣**（→ Xboard 路线，超出本项目定位）。
- **不做 Web Terminal**（→ P2 后期，agent 已为此预留接口）。
- **不做 Docker 容器级监控**（→ P2 后期）。
- **不做客户端 App**（专注服务端 / Web 面板）。
- **不做完整 RBAC**（admin / user 二分够用，详 ADR 0002）。
- **不做公开状态页**（→ P2 后期）。
- **不做 OAuth/OIDC**（→ P2 后期）。
- **不做主题市场**（→ P2 后期）。
- **不做 WebDAV 配置同步**（→ P2 后期）。

### 7.2 技术约束
- 后端必须能跑在 ≥ Linux 3.10 内核（避免新 syscall）。
- agent 必须能跑在 256MB RAM 的 VPS。
- SQLite 强制 WAL 模式。
- **不依赖 cgo**（modernc.org/sqlite 是纯 Go 实现）。

### 7.3 依赖与第三方
- `github.com/dop251/goja` 用于 JS 脚本沙箱。
- `github.com/pquerna/otp` 用于 TOTP。
- `github.com/gorilla/websocket` 用于 agent 心跳。
- `gopkg.in/yaml.v3` 用于 YAML 解析（必须用 Node API 保留字段顺序）。
- `modernc.org/sqlite` 用于数据库（纯 Go）。
- `golang.org/x/crypto/bcrypt` 用于密码哈希。
- `log/slog`（标准库）用于结构化日志。
- 前端：React 19 + TanStack Router + TanStack Query + Tailwind v4 + Radix UI + react-i18next + react-hook-form + @dnd-kit。

## 8. 开放问题

仍然不确定但不阻塞 PRD 起草的点（同步进 CONTEXT.md Flagged 区）：

1. **agent ↔ hub 通信协议长期路线**：默认走 WebSocket（同 Komari）；如未来需要 gRPC 性能更高可后期切（已记入 ADR 0003）。**默认假设：v1 全部 WebSocket。**
2. **流水线 YAML schema 演进**：默认 v1 = 平铺数组 + type 字段；如需嵌套表达式可加 v2（CONTEXT.md Flagged #2）。**默认假设：先冻结 v1。**
3. **算子之间的数据契约**：v1 全部 `[]Node`；分组类需求用 `node.tags` 元数据表达。**默认假设：v1 不引入中间产物类型。**
4. **多用户共享订阅**：admin 可见全部 user 订阅，user 仅可见自己的；"share to user"动作 v1.1 评估。**默认假设：v1 不做共享。**
5. **Nezha 协议字段映射详表**：暂只兼容 v2 最小字段集（详 ADR 0003 附录 A）；扩展字段忽略并打 warning 日志。**默认假设：v1 最小集。**

## 9. 路线图（高层）

总工期：**9 周（中位估算，区间 7-11 周）**。

| 阶段 | 周次 | 交付 |
|------|------|------|
| Sprint 1 | W1-W2 | 用户体系（M-USER）+ 订阅基础 CRUD（M-SUB-1/2/3）+ 多协议 URI 解析（M-SUB-4） |
| Sprint 2 | W3-W4 | 规则系统（M-RULE）+ 脚本扩展（M-SCRIPT）+ 链式代理（M-NODE-4）+ TCPing（M-NODE-3） |
| Sprint 3 | W5-W6 | agent（M-AGENT）+ Nezha 兼容（M-AGENT-5）+ 流量聚合（M-TRAFFIC） |
| Sprint 4 | W7 | **差异化 #1：算子流水线（M-PIPE）** |
| Sprint 5 | W8 | **差异化 #2：通知系统（M-NOTIFY）含 Telegram Bot 双向** |
| Sprint 6 | W9 | 工程亮点（静默模式 M-OPS-1 / OTA M-OPS-2 / 短链 M-OPS-3）+ i18n（M-I18N）|
| Sprint 7 | W10-W11 | 集成测试 + 文档 + 1.0 发布；缓冲 buffer |

里程碑（M）：

- **M1（W4 末）**：基础订阅管理 + 规则脚本可用，可承接 sub-store 用户。
- **M2（W6 末）**：agent + Nezha 兼容上线，可承接 Nezha 用户。
- **M3（W8 末）**：差异化 #1 + #2 全部到位，v1.0 RC 可演示。
- **M4（W11）**：v1.0 正式发布。
