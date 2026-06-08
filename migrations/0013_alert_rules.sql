-- 0013_alert_rules.sql
-- 探针告警规则:针对 agent 指标(CPU/内存/磁盘 %)或在线状态设阈值,
-- 后台引擎周期评估,命中且过冷却期则经 notify 发告警。
-- agent_id 为 NULL 表示规则作用于该用户的所有探针。
CREATE TABLE IF NOT EXISTS alert_rules (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    name         TEXT NOT NULL,
    enabled      INTEGER NOT NULL DEFAULT 1,
    agent_id     TEXT,                      -- NULL = 该用户全部探针
    metric       TEXT NOT NULL,             -- cpu | mem | disk | offline
    threshold    REAL NOT NULL DEFAULT 0,   -- 百分比(offline 忽略)
    duration_sec INTEGER NOT NULL DEFAULT 0, -- 需持续多久才触发(秒)
    cooldown_sec INTEGER NOT NULL DEFAULT 3600, -- 再次告警冷却(秒)
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_alert_rules_user ON alert_rules(user_id);
CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled);
