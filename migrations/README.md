# migrations

拾光VPS 数据库迁移策略说明。

## 双轨制

按 `docs/03-architecture.md` §6.2 决策，本项目采用 **显式 SQL migration + 运行时 EnsureColumn** 双轨：

| 场景 | 工具 | 文件位置 |
| --- | --- | --- |
| **大版本断点**（v1.0、v2.0 …）：建表、表重构、数据回填 | 显式 SQL 文件 | `migrations/000X_*.sql` |
| **小补丁**：单字段增量 | `storage.EnsureColumn(table, col, ddl)` | 调用方代码（启动前调用） |

## 文件命名

- 严格 `NNNN_description.sql`，4 位补零，描述用 snake_case。
- 例：`0001_initial.sql`、`0002_add_audit_export.sql`。

## 执行顺序

`internal/storage/migrate.go` 通过 `embed.FS` 读取 `migrations/*.sql` 后按文件名升序应用：

1. 启动时打开数据库（WAL 模式）。
2. 创建 `schema_migrations` 表（首次）。
3. 遍历嵌入的 `*.sql` 文件，跳过已登记的文件名。
4. 对每个未应用文件：在事务内执行 → 写入 `schema_migrations`。
5. 增量补丁通过显式调用 `db.EnsureColumn(...)` 进行。

## 幂等性

所有 SQL 必须使用 `CREATE TABLE IF NOT EXISTS` / `CREATE INDEX IF NOT EXISTS` 写法，确保即便 `schema_migrations` 丢失也能重复执行不报错。

## 0001_initial.sql 包含的 17 张表

`schema_migrations`（系统）+ 业务 16 张：`users`、`sessions`、`subscriptions`、`nodes`、`pipelines`、`pipeline_bindings`、`custom_rules`、`scripts`、`agents`、`agent_records`、`traffic_records`、`notification_channels`、`notification_events`、`short_links`、`audit_logs`、`system_settings`。

具体字段定义见 `docs/03-architecture.md` §4.2。
