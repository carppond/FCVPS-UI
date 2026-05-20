-- 0001_initial.sql
-- 拾光VPS v1.0 数据库初始化脚本。包含全部 17 张表。
-- 来源：docs/03-architecture.md §4.2。
-- 执行策略：启动时按文件名升序执行；schema_migrations 记录已应用文件名以保证幂等。

-- ---------------------------------------------------------------------------
-- schema_migrations: 迁移版本登记
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename   TEXT PRIMARY KEY,
    applied_at INTEGER NOT NULL
);

-- ---------------------------------------------------------------------------
-- 1. users: 用户
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id                  TEXT PRIMARY KEY,
    username            TEXT NOT NULL UNIQUE,
    password_hash       TEXT NOT NULL,
    role                TEXT NOT NULL CHECK(role IN ('admin','user')),
    is_active           INTEGER NOT NULL DEFAULT 1,
    email               TEXT,
    locale              TEXT NOT NULL DEFAULT 'zh-CN',
    totp_secret         TEXT,
    totp_enabled        INTEGER NOT NULL DEFAULT 0,
    recovery_codes_hash TEXT,
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- ---------------------------------------------------------------------------
-- 2. sessions: 登录态
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sessions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL,
    pending_2fa  INTEGER NOT NULL DEFAULT 0,
    expires_at   INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL,
    ip           TEXT,
    user_agent   TEXT,
    created_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

-- ---------------------------------------------------------------------------
-- 3. subscriptions: 订阅
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS subscriptions (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    type             TEXT NOT NULL CHECK(type IN ('url','upload','manual')),
    source_url       TEXT,
    raw_content      BLOB,
    ua               TEXT,
    sync_interval    INTEGER NOT NULL DEFAULT 21600,
    last_synced_at   INTEGER,
    last_sync_status TEXT,
    last_sync_error  TEXT,
    expire_at        INTEGER,
    traffic_total    INTEGER,
    traffic_used     INTEGER,
    tags             TEXT NOT NULL DEFAULT '[]',
    remark           TEXT,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions(user_id);

CREATE INDEX IF NOT EXISTS idx_subscriptions_last_synced ON subscriptions(last_synced_at);

-- ---------------------------------------------------------------------------
-- 4. nodes: 节点
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nodes (
    id                 TEXT PRIMARY KEY,
    subscription_id    TEXT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    raw_uri            TEXT NOT NULL,
    parsed_config_json TEXT NOT NULL,
    protocol           TEXT NOT NULL,
    server             TEXT NOT NULL,
    port               INTEGER NOT NULL,
    tag                TEXT NOT NULL,
    tags               TEXT NOT NULL DEFAULT '[]',
    is_chain_proxy     INTEGER NOT NULL DEFAULT 0,
    chain_parent_id    TEXT REFERENCES nodes(id) ON DELETE SET NULL,
    position           INTEGER NOT NULL DEFAULT 0,
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_nodes_sub ON nodes(subscription_id);

CREATE INDEX IF NOT EXISTS idx_nodes_protocol ON nodes(protocol);

CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_sub_dedupe ON nodes(subscription_id, server, port, protocol);

-- ---------------------------------------------------------------------------
-- 5. pipelines: 流水线
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipelines (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    yaml_content   TEXT NOT NULL,
    ast_json       TEXT NOT NULL,
    version        INTEGER NOT NULL DEFAULT 1,
    schema_version TEXT NOT NULL DEFAULT 'shiguang/v1',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pipelines_user ON pipelines(user_id);

-- ---------------------------------------------------------------------------
-- 6. pipeline_bindings: 流水线绑定订阅
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pipeline_bindings (
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    pipeline_id     TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL DEFAULT 0,
    enabled         INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (subscription_id, pipeline_id)
);

CREATE INDEX IF NOT EXISTS idx_pipeline_bindings_sub ON pipeline_bindings(subscription_id, position);

-- ---------------------------------------------------------------------------
-- 7. custom_rules: 自定义规则
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS custom_rules (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    type       TEXT NOT NULL CHECK(type IN ('dns','rules','rule-providers')),
    mode       TEXT NOT NULL CHECK(mode IN ('replace','prepend','append')),
    content    TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    sort       INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_custom_rules_user ON custom_rules(user_id, type, sort);

-- ---------------------------------------------------------------------------
-- 8. scripts: JS 脚本
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS scripts (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    hook        TEXT NOT NULL CHECK(hook IN ('pre_save_nodes','post_fetch')),
    code        TEXT NOT NULL,
    enabled     INTEGER NOT NULL DEFAULT 1,
    last_run_at INTEGER,
    last_error  TEXT,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scripts_user_hook ON scripts(user_id, hook, enabled);

-- ---------------------------------------------------------------------------
-- 9. agents: 探针
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agents (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK(kind IN ('native','nezha_compat')),
    version      TEXT,
    os           TEXT,
    arch         TEXT,
    public_ip    TEXT,
    last_seen_at INTEGER,
    status       TEXT NOT NULL DEFAULT 'offline' CHECK(status IN ('online','offline','degraded')),
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_user ON agents(user_id);

CREATE INDEX IF NOT EXISTS idx_agents_token_hash ON agents(token_hash);

CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen_at);

-- ---------------------------------------------------------------------------
-- 10. agent_records: 高频原始记录（7 天保留）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agent_records (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id      TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    recorded_at   INTEGER NOT NULL,
    cpu_percent   REAL,
    mem_used      INTEGER,
    mem_total     INTEGER,
    swap_used     INTEGER,
    swap_total    INTEGER,
    disk_used     INTEGER,
    disk_total    INTEGER,
    net_in        INTEGER,
    net_out       INTEGER,
    net_in_speed  INTEGER,
    net_out_speed INTEGER,
    conn_tcp      INTEGER,
    conn_udp      INTEGER,
    load1         REAL,
    load5         REAL,
    load15        REAL,
    uptime        INTEGER,
    process_count INTEGER
);

CREATE INDEX IF NOT EXISTS idx_agent_records_agent_time ON agent_records(agent_id, recorded_at);

-- ---------------------------------------------------------------------------
-- 11. traffic_records: 流量日聚合
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS traffic_records (
    date        TEXT NOT NULL,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id    TEXT REFERENCES agents(id) ON DELETE SET NULL,
    total_limit INTEGER,
    total_used  INTEGER NOT NULL DEFAULT 0,
    total_in    INTEGER NOT NULL DEFAULT 0,
    total_out   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (date, user_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_traffic_user_date ON traffic_records(user_id, date);

-- ---------------------------------------------------------------------------
-- 12. notification_channels: 通知通道
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notification_channels (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    name        TEXT NOT NULL,
    config_json TEXT NOT NULL,
    template    TEXT,
    event_types TEXT NOT NULL DEFAULT '[]',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notification_channels_user ON notification_channels(user_id, kind);

-- ---------------------------------------------------------------------------
-- 13. notification_events: 通知投递日志
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notification_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id  TEXT REFERENCES notification_channels(id) ON DELETE SET NULL,
    event_type  TEXT NOT NULL,
    dedupe_key  TEXT,
    payload     TEXT NOT NULL,
    status      TEXT NOT NULL CHECK(status IN ('pending','sent','failed','skipped_dedupe')),
    sent_at     INTEGER,
    error       TEXT,
    created_at  INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notification_events_user_time ON notification_events(user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_notification_events_dedupe ON notification_events(dedupe_key, created_at);

-- ---------------------------------------------------------------------------
-- 14. short_links: 短链
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS short_links (
    file_code  TEXT NOT NULL,
    user_code  TEXT NOT NULL,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_url TEXT NOT NULL,
    expires_at INTEGER,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (file_code, user_code)
);

CREATE INDEX IF NOT EXISTS idx_short_links_user ON short_links(user_id);

CREATE INDEX IF NOT EXISTS idx_short_links_expires ON short_links(expires_at);

-- ---------------------------------------------------------------------------
-- 15. audit_logs: 审计日志
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_logs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT NOT NULL,
    resource_type TEXT,
    resource_id   TEXT,
    ip            TEXT,
    user_agent    TEXT,
    payload       TEXT,
    success       INTEGER NOT NULL DEFAULT 1,
    created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_user_time ON audit_logs(user_id, created_at);

CREATE INDEX IF NOT EXISTS idx_audit_action_time ON audit_logs(action, created_at);

-- ---------------------------------------------------------------------------
-- 16. system_settings: 系统设置（k/v）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS system_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
