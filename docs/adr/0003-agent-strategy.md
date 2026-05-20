# ADR 0003: 探针策略 —— 自写 agent + 兼容 Nezha 协议

## Context

- 妙妙屋的多探针流量聚合是其工程亮点，目前依赖外部探针（哪吒 / 哪吒 dash 第三方 agent）拉数据，缺乏自有一致性。
- Komari（docs/_research-competitors.md 第一节卡片 3）的成功经验：自带 `compat/nezha` 兼容层，让 Nezha agent 不改配置直接连入，迁移成本几乎为零，**这是该垂类最有效的获客手段**（调研报告"最佳实践 #1"明确列出）。
- 同时，自写 agent 才能保证：(a) 单二进制 < 10MB（Nezha agent 是 20MB+）；(b) 未来加 Docker 容器观测 / Web Terminal 等扩展时不被外部协议绑死；(c) hub ↔ agent 通讯协议（WebSocket）由我们自己掌控。
- 业界数据：Beszel agent 仅 6MB RAM；Komari 用 WebSocket 长连接，时延比 Nezha gRPC 拉模型更低。

## Decision

**双轨策略**：

1. **主路：自写 Go agent**，单二进制 < 10MB，与 hub 走 WebSocket 长连接（30s 心跳，可配），上报 CPU / MEM / Disk / NetIO / Load / 连接数；token + TLS 鉴权。
2. **兼容路：Nezha agent 协议兼容层**。hub 暴露 `/api/v1/nezha/...` 端点，兼容 Nezha v2 心跳协议最小字段集（CPU/MEM/Disk/NetIO/Load）。**作为迁移诱饵**，原 Nezha 用户仅需改 server 地址即可接入拾光VPS。

agent 自更新走单独通路：hub 周期性广播"最新 agent 版本"，agent 检测后自拉取替换，与 hub OTA 解耦。

## Consequences

**正面**：
- 兼容 Nezha 协议 = 直接吃下数千 Nezha 用户的迁移诉求；获客成本接近零。
- 自写 agent 保留未来扩展性（容器观测 / Web Terminal 等差异化机会在 ADR 后续扩展时不被卡住）。
- 单二进制 < 10MB，可跑在 256MB RAM VPS 上，符合目标用户群（小 VPS 玩家）。
- WebSocket 长连接：心跳低延迟（< 1s），且天然适合"hub → agent 下发命令"（如刷新订阅、重启 agent）。

**负面 / 待办**：
- Nezha 协议兼容需要持续跟进 Nezha 上游 v1→v2 的演进；当前默认只兼容 v2 最小字段集（CONTEXT.md Flagged #1 已标注），未来扩展需补 ADR。
- 自写 agent 需要单独的 CI/CD 流水线（多平台交叉编译：linux amd64/arm64 / macOS / Windows）。
- WebSocket 在某些防火墙环境下可能被截断，需要预留 HTTPS long-polling fallback（v1 暂不做，记入 P2）。

**替代方案为何放弃**：
- **直接拿 Nezha agent / Komari agent 当依赖**：丧失协议演进自主权，未来加扩展字段时受制于上游。
- **gRPC 协议（Nezha v2 路线）**：实现复杂度高、二进制体积大、且 WebSocket 已足够（Komari 已证明）。
- **完全自写不兼容 Nezha**：放弃迁移诱饵这一最大获客手段，得不偿失。

## Related

- ADR 0001: 技术栈（Go agent 依赖纯 Go SQLite 与 modernc 生态）
- ADR 0006: 静默模式（agent 接入端点需要鉴权 token；token 错误返 404）
- 调研：docs/_research-competitors.md 第四节"差异化机会 1"（探针扩展路线）+ 第五节"最佳实践 #1"

## 附录 A（Flagged）

Nezha v1/v2 字段映射详表暂未冻结，等 Sprint 3 实施前由架构师补充。当前默认范围：
- v2 心跳字段子集：State { cpu, memory, swap, disk, net_in_speed, net_out_speed, net_in_transfer, net_out_transfer, uptime, load_1/5/15, tcp_conn_count, udp_conn_count, process_count }。
