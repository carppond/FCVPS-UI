-- 0009_agent_traffic_limit.sql
-- Per-agent monthly traffic quota (decoupled from VPS assets / account-wide limit).
--   traffic_limit : operator-entered monthly cap in BYTES (0 = unset).
--   bwg_*         : BandwagonHost (64clouds) API auto-fetch — credentials stored
--                   as-is (TEXT), consistent with vps_assets SSH creds (0008);
--                   protected by HTTPS + auth in transit. bwg_used/bwg_limit are
--                   the provider-reported figures cached by a background poller.
-- Used (measured) comes from agent_records aggregation; limit/source are derived
-- at read time (bandwagon when synced, else manual, else none).

ALTER TABLE agents ADD COLUMN traffic_limit INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agents ADD COLUMN bwg_veid TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN bwg_api_key TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN bwg_used INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agents ADD COLUMN bwg_limit INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agents ADD COLUMN bwg_reset_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE agents ADD COLUMN bwg_synced_at INTEGER NOT NULL DEFAULT 0;
