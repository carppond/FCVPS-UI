# 拾光VPS 领域术语表

> 用途：所有团队成员（PM / 架构师 / 前后端 / Reviewer / AI Agent）在沟通时统一口径。任何模糊点先回到此处确认。
> 维护原则：新增术语先进 Definitions；发现易混淆 → 进 Avoid；新增实体关系 → 进 Relationships；未决议问题 → 进 Flagged。

---

## Definitions（术语 → 一句话定义）

- **拾光VPS（shiguang-vps）**：本项目代号；个人自托管的 Clash 订阅聚合 + 多探针流量观测 + 通知中枢面板。
- **订阅（Subscription）**：一组节点的来源单位，可以是外部 URL（远程拉取）、上传的 YAML 文件、或手动创建；带元数据（标签、更新策略、过期时间、流量额度等）。
- **节点（Node / Proxy）**：单个代理出口的具象表达，包含协议（vmess/vless/ss/...）、server、port、加密参数等；属于某个订阅。
- **算子（Operator）**：订阅流水线中的一个处理步骤，类型固定（filter / map / sort / dedupe / regex-rename / output 等），有结构化输入输出。
- **流水线（Pipeline）**：一条订阅之上挂载的一串算子的有序组合，运行后从原始节点列表得出目标节点列表；同时支持 UI 拖拽编辑与 YAML 文件描述。
- **探针（Probe）**：部署在远端 VPS 上、负责上报机器指标的轻量进程；具象为 agent 二进制。
- **agent**：探针进程本身，自写 Go 二进制 < 10MB；同时兼容 Nezha v1/v2 agent 协议。
- **hub**：拾光VPS 主服务进程，负责接收 agent 心跳、聚合数据、提供 Web UI 和 API。
- **agent_record**：agent 上报的一条原始指标记录（CPU/MEM/带宽/连接数/磁盘/负载等），高频；保留窗口短（默认 7 天）。
- **traffic_record**：经过日聚合的流量记录，对应"日累计上下行字节数"，长期保留；按月度计费周期累加可得"本月已用流量"。
- **规则提供者（Rule Provider）**：Clash 配置中可外部加载的规则集（domain/ipcidr/classical），由本项目的 `custom_rules` 表统一管理输出。
- **自定义规则（Custom Rule）**：用户在面板上配置的、要注入到 Clash 配置最终输出中的 DNS / rules / rule-providers 片段，每条带 mode（replace/prepend/append）。
- **goja 脚本扩展**：基于 dop251/goja 的 JS 沙箱钩子；目前有两个 hook：`pre_save_nodes`（入库前对节点列表做加工）、`post_fetch`（订阅拉回原始内容后预处理）。
- **链式代理（Proxy Chain）**：一个出口节点的实际流量被串接经过另一个/多个节点；在节点表里用 `dialer-proxy` 字段表达。
- **sub-store API 兼容层**：拾光VPS 暴露与 sub-store 风格一致的 HTTP 路由（如 `/download/:name`），让原本配在 sub-store 的客户端无改动迁移过来。
- **Nezha agent 协议兼容**：拾光VPS hub 暴露与 Nezha v1/v2 agent 心跳协议兼容的接收端点，原 Nezha agent 仅需改 server 地址即可接入。
- **静默模式（Silent Mode）**：保护性运行模式——未带合法授权的请求（如未登录、token 错误、未知路径）一律返回 HTTP 404，不暴露任何"此处是拾光VPS"的特征。
- **OTA 自更新**：hub 进程检测 GitHub Release，下载新版本二进制，校验 SHA-256，自替换并重启的能力。
- **TOTP 2FA**：基于 pquerna/otp 的时间型一次性密码二步验证，使用 RFC 6238。
- **备份码（Backup Code）**：用户首次启用 2FA 时生成的 N 个 8 位 hex 一次性救援码，sha256 存库；用一个销毁一个。
- **短链系统**：拾光VPS 内置的 URL 缩短服务，主键由 `fileCode + userCode` 复合而成；用于把长订阅 URL 缩到便于客户端配置。
- **NotificationChannel**：通知渠道在代码层的抽象接口（Go interface），具体实现包括 Telegram / Discord / Slack / Email / Bark / Gotify / Webhook / Server酱 / PushDeer / IFTTT 等 10+ 个。
- **Telegram Bot 双向交互**：通过 inline keyboard 让用户在 Telegram 里执行查节点、刷订阅、重启 agent 等管理操作，不必打开 Web。
- **静默模式 vs 维护模式**：静默模式是默认运行态对未授权请求的隐身；维护模式（如有）才是把所有用户都挡在外面。本项目只做静默模式。
- **ACL4SSR**：中文 Clash 圈广泛使用的、由 subconverter 解析的老式规则配置仓库；拾光VPS 需要兼容其常见的 ini / list 文件格式。
- **i18n locale**：界面语言文件粒度，本项目固定 4 套：zh-CN、en、ja、ko。

## Avoid（容易混淆的反例）

- **"订阅"≠"节点"**：订阅是一个节点集合 + 元数据（更新策略、流量、过期等）；节点是其中一个出口。一个订阅里通常有几十到几百个节点。
- **"agent"≠"代理（proxy）"**：agent 是探针进程，跑在 VPS 上向 hub 汇报机器指标；代理节点是 Clash 客户端连出去用的出口。两者完全不是同一类东西。
- **"Filter 算子"≠"GeoIP 过滤"**：Filter 算子是订阅流水线的一个步骤（按表达式留下符合条件的节点）；GeoIP 是其中一个常见使用维度。Filter 是抽象，GeoIP 是具体。
- **"自定义规则"≠"Clash 配置文件"**：自定义规则只是要注入到最终 Clash 配置里的片段（DNS 段 / rules 段 / rule-providers 段），不是完整配置；完整配置由 hub 在生成时把订阅节点 + 用户规则合并而成。
- **"OTA 自更新"≠"agent 自更新"**：OTA 指 hub 主进程自更新；agent 的自更新走另一条单独通路（hub 推 + agent 拉，见 ADR 0003）。
- **"静默模式"≠"维护模式"**：静默模式是常态防扫描（仅未授权请求被 404）；维护模式（暂未做）才是停所有服务。
- **"短链"≠"订阅 URL"**：短链是把长 URL 包装成短形式的服务，订阅 URL 是面板生成的客户端可消费 URL，短链可以指向订阅 URL，但短链本身不是订阅。
- **"sub-store API 兼容层"≠"sub-store 完整功能复刻"**：本项目只兼容 sub-store 用户端会调用的 HTTP 路由（足以让客户端零改动迁移），不复刻 sub-store 内部的脚本算子实现细节。

## Relationships（实体之间的关系）

```
User (1) ──< owns >── (N) Subscription
   │                         │
   │                         ├──< contains >── (N) Node
   │                         │
   │                         └──< has-pipeline >── (N) PipelineOperator
   │                                                      │
   │                                                      └─ ordered, runtime: filter → map → sort → output
   │
   ├──< has-2fa >── (1) TotpSecret + (N) BackupCode
   ├──< has-rules >── (N) CustomRule  (dns / rules / rule-providers × replace/prepend/append)
   ├──< has-scripts >── (N) Script (hook: pre_save_nodes | post_fetch)
   ├──< has-shortlinks >── (N) ShortLink  (PK: fileCode + userCode)
   └──< has-notifications >── (N) NotificationChannel  (telegram/discord/...)

Agent (1) ──< reports >── (N) AgentRecord (高频, 短保留)
                              └──< 日聚合 >── TrafficRecord (长期保留)

Node (N) ──< chains-via >── (N) Node     （链式代理；dialer-proxy）
Node (N) ──< tagged-by >── (N) Tag
Subscription (1) ──< triggers >── (N) NotificationEvent ──< dispatched-to >── (N) NotificationChannel
```

## Flagged ambiguities（已注意到但暂未决议的模糊点）

1. **agent ↔ Nezha 协议字段映射的细粒度**：Nezha v1 与 v2 的心跳 protobuf 字段并不完全对齐，本项目兼容到哪个版本、哪些字段必填 / 选填，需要在 ADR 0003 的"附录 A"里给出完整映射表。当前默认假设：**兼容 v2 最小字段集（CPU/MEM/Disk/NetIO/Load）**，扩展字段忽略。
2. **流水线 YAML schema 版本号**：v1 默认为"平铺数组 + type 字段"；如未来需要嵌套表达式（如 if/then/else），需要 v2。当前先冻结 v1 schema，并在 PRD 与 ADR 0004 注明"v1/v2 兼容由 hub 端按 `apiVersion: shiguang/v1` 字段路由"。
3. **算子之间的数据契约**：是约束所有算子输入输出都是 `[]Node`，还是允许中间产物为别的类型（如 `map[string][]Node` 分组）？暂定 **v1 全部为 `[]Node`**，分组类需求用元数据字段（如 `node.tags`）表达。
4. **多用户共享订阅**：admin 创建的订阅是否对 user 可见 / 可用？目前默认 **admin 全可见所有用户的订阅；user 仅可见自己的；订阅是否支持"share to user"动作待 v1.1 评估**。
5. **OTA 自更新与 SQLite 文件锁**：单进程替换二进制时 WAL 文件如何处理？默认假设 **OTA 触发优雅停机后再替换，重启时自动 wal_checkpoint**；如有问题在 ADR 0008 补救。
