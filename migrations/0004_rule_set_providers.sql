-- 0004_rule_set_providers.sql
--
-- M-RULE 扩展：规则集（Rule Provider）持久化层。镜像 mihomo / Clash-Meta 的
-- rule-providers YAML 块，每行对应一个远程规则集订阅。同步是惰性的：hub 只
-- 校验 URL 可达 + 记 last_synced_at，真正的 .mrs / .yaml 下载由客户端完成。
--
-- behavior / format 的取值与 mihomo 主线一致（CHECK 约束在表上）。
-- 跨用户隔离通过 user_id 外键 + ON DELETE CASCADE 保证。

CREATE TABLE IF NOT EXISTS rule_set_providers (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    behavior         TEXT NOT NULL CHECK (behavior IN ('domain', 'ipcidr', 'classical')),
    format           TEXT NOT NULL CHECK (format IN ('yaml', 'text', 'mrs')),
    url              TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL DEFAULT 86400,
    enabled          INTEGER NOT NULL DEFAULT 1,
    last_synced_at   INTEGER,
    last_sync_status TEXT,
    last_sync_error  TEXT,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rule_set_providers_user
    ON rule_set_providers(user_id);
