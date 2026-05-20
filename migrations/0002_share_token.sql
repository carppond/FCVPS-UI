-- 0002_share_token.sql
--
-- Add subscriptions.share_token used by sub-store v2 compat path
-- GET /download/:name?token=<share_token>.
-- (docs/05-tech-lead-plan.md §1.3 / T-8 裁决)
--
-- DEFAULT NULL keeps the column backward-compatible with pre-T-8 rows; the
-- SubscriptionRepo back-fills NULL rows lazily on first read by generating a
-- 32-byte base64url token. New rows are always inserted with a non-empty
-- value, so once every legacy row has been observed the column is effectively
-- NOT NULL.
--
-- SQLite does not support ADD CONSTRAINT, so the uniqueness invariant is
-- enforced by a partial unique index (NULLs are ignored — multiple legacy
-- rows can coexist until the back-fill touches them).

ALTER TABLE subscriptions ADD COLUMN share_token TEXT DEFAULT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_share_token
    ON subscriptions(share_token)
    WHERE share_token IS NOT NULL;
