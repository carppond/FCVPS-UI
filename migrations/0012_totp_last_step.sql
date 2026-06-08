-- 0012_totp_last_step.sql
-- TOTP replay protection: record the time-step of the last successfully used
-- code so a 6-digit code can't be reused within its (±skew) validity window.
-- 0 means "no code consumed yet".
ALTER TABLE users ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;
