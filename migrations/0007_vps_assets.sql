-- Migration: VPS asset management table (M-ASSET)
CREATE TABLE IF NOT EXISTS vps_assets (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ip TEXT,
    ssh_port INTEGER DEFAULT 22,
    ssh_user TEXT,
    os TEXT,
    location TEXT,
    provider TEXT NOT NULL,
    price REAL NOT NULL,
    currency TEXT DEFAULT 'CNY',
    billing_cycle TEXT NOT NULL CHECK (billing_cycle IN ('monthly','quarterly','semi_annual','annual','biennial','triennial')),
    bandwidth TEXT,
    monthly_traffic INTEGER DEFAULT 0,
    cpu TEXT,
    memory TEXT,
    disk TEXT,
    expire_at TEXT NOT NULL,
    notes TEXT,
    agent_id TEXT,
    tags TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_vps_assets_user ON vps_assets(user_id);
CREATE INDEX IF NOT EXISTS idx_vps_assets_expire ON vps_assets(expire_at);
