-- 0006_silent_mode_default_disabled.sql
--
-- T-26 follow-up: silent mode is now opt-in. Previously the hub auto-
-- generated a `silent_mode_prefix` on first boot and the middleware treated
-- "prefix exists" as "enforce". That made the dev loop painful (every
-- restart with a wiped DB minted a new prefix, invalidating stored URLs).
--
-- Going forward an explicit `silent_mode_enabled` row gates the middleware.
-- This migration:
--   1. Inserts `silent_mode_enabled='false'` when the row is missing (fresh
--      installs default to OFF).
--   2. Intentionally LEAVES any existing `silent_mode_prefix` row alone —
--      disabling does not destroy the prefix so re-enabling reuses the same
--      entry URL. Operators who had silent mode auto-enforced under the old
--      behaviour will see it turn OFF on first boot of the new code and can
--      flip it back on through Admin → Settings.

INSERT INTO system_settings (key, value, updated_at)
SELECT 'silent_mode_enabled', 'false', CAST(strftime('%s', 'now') AS INTEGER) * 1000
WHERE NOT EXISTS (
    SELECT 1 FROM system_settings WHERE key = 'silent_mode_enabled'
);
