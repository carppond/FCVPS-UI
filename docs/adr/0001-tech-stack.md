# ADR 0001: 技术栈选型 —— Go 1.24 + React 19 + SQLite

## Context

- 拾光VPS 的目标是"个人 / 小团队自托管的 Clash 订阅 + 探针面板"。
- 同类垂直领域（10 个候选项目调研，见 `docs/_research-competitors.md` 第三节）的事实标准：
  - 后端：Go 占 6/10（Nezha、Komari、Beszel、3X-UI、Gatus、妙妙屋），是自托管 + 探针类的绝对主流。
  - 数据库：SQLite 占 6/10 默认。
  - 部署：10/10 全部提供 Docker；6 个 Go 项目都做单二进制；8 个有一键脚本（中文圈强需求）。
- 主参考项目妙妙屋使用 Go + React 19 + SQLite，社区接受度已被验证。
- 当前限制：本项目作者倾向单人 / 小团队维护，需要"低运维成本 + 单文件部署 + 跨平台二进制"。

## Decision

采用 **Go 1.24（后端） + React 19 + TanStack Router/Query + Tailwind v4 + Radix UI（前端） + SQLite（modernc.org/sqlite，纯 Go 实现）**。明确拒绝 Node.js / PHP / Rust 作为后端语言；明确拒绝 PostgreSQL / MySQL 作为默认数据库。

## Consequences

**正面**：
- 单二进制部署（前端 embed 进 Go binary），用户开箱即用，符合"低运维"目标。
- modernc.org/sqlite 纯 Go 实现，无需 cgo，可在 musl libc / Windows / 各种 ARM 平台无障碍交叉编译。
- Go 的内存安全 + 静态类型 + GC 调优成熟，单机内存稳态 < 50MB 可达。
- React 19 + TanStack 套件是 2025 年现代前端工程的稳定组合，社区文档充足。
- 与同类项目（妙妙屋、Nezha、Komari、Beszel）技术栈对齐，社区贡献者无学习成本。

**负面 / 待办**：
- modernc.org/sqlite 性能略低于 mattn/go-sqlite3（cgo 版），单机 1k QPS 以下场景不构成瓶颈；如需高并发后续可选切（不影响 schema）。
- Tailwind v4 与 Radix UI 在 2026 年初仍属于较新组合，少量第三方组件库需自研。
- Go 的反射 + 范型在算子流水线（差异化 #1）的类型抽象上略繁琐，需要在 ADR 0004 单独设计。

**替代方案为何放弃**：
- **Node.js + TypeScript（如 sub-store、Uptime Kuma）**：单进程并发模型在 1000+ 监控目标时性能瓶颈明显（Uptime Kuma 已被社区抱怨），且二进制分发不友好。
- **Rust（如 Clash Verge Rev 部分）**：开发效率低于 Go，本项目对极致性能无诉求；编译体积与时间成本高。
- **PHP / Laravel（如 Xboard）**：纯 SaaS 路线友好但与本项目"个人自托管"定位错位；运行需 PHP-FPM + Nginx，部署门槛高。
- **PostgreSQL 默认**：小团队场景 SQLite 足够；引入 PG 增加部署复杂度，与"单二进制"目标矛盾。WAL 模式下 SQLite 单写并发已可满足 P0 全部需求。

## Related

- ADR 0008: 部署模型
- 调研：docs/_research-competitors.md 第三节"技术栈倾向汇总"
