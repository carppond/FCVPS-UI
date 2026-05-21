-- 0003_node_latency.sql
--
-- Adds last_latency_ms / last_tested_at columns to nodes so the M-NODE
-- TCPing endpoints (T-11) can persist their results. The columns are
-- nullable (DEFAULT NULL) — nodes that have never been measured carry
-- NULL and are surfaced to the UI as "untested".
--
-- last_latency_ms semantics:
--   * NULL    — never tested
--   * -1      — last test was unreachable (timeout / connection refused)
--   * 0..N    — round-trip in milliseconds

ALTER TABLE nodes ADD COLUMN last_latency_ms INTEGER DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN last_tested_at  INTEGER DEFAULT NULL;
