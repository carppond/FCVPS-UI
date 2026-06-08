-- 0014_subscription_sync_logs.sql
-- 订阅同步历史:每次同步(成功或失败)记一条,便于排查"从何时开始失败 / 节点数
-- 波动"。subscriptions 表只保留最新一次状态;这里保留多条历史(按订阅滚动保留)。
CREATE TABLE IF NOT EXISTS subscription_sync_logs (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    status          TEXT NOT NULL,          -- ok | error
    node_count      INTEGER NOT NULL DEFAULT 0,
    error           TEXT,                   -- 失败原因(成功为空)
    created_at      INTEGER NOT NULL,
    FOREIGN KEY (subscription_id) REFERENCES subscriptions(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sync_logs_sub_time
    ON subscription_sync_logs(subscription_id, created_at DESC);
