-- 0009_widget_tokens.sql — read-only tokens for the iOS/Android home-screen
-- traffic widget.
--
-- A widget token is a scoped, separately-revocable credential the mobile app
-- mints and stores in its platform shared container so the widget extension
-- can fetch a tiny traffic payload WITHOUT carrying the full session token
-- (which can do everything). Only sha256(token) is persisted, mirroring the
-- sessions table. One active token per user: minting replaces the previous row
-- so "disable widget" / rotation is a single delete.

CREATE TABLE IF NOT EXISTS widget_tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL,
    created_at   INTEGER NOT NULL,
    last_used_at INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_widget_tokens_hash ON widget_tokens(token_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_widget_tokens_user ON widget_tokens(user_id);
