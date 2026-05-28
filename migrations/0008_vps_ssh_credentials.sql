-- 0008_vps_ssh_credentials.sql
-- Adds SSH credential fields to vps_assets for mobile SSH terminal feature.
-- Credentials are stored as-is (TEXT); transport encryption via HTTPS + auth.
-- Note: Mobile app fetches these over authenticated HTTPS only.

ALTER TABLE vps_assets ADD COLUMN ssh_password TEXT NOT NULL DEFAULT '';
ALTER TABLE vps_assets ADD COLUMN ssh_private_key TEXT NOT NULL DEFAULT '';
