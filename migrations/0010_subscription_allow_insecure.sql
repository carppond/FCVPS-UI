-- 0010_subscription_allow_insecure.sql
-- Per-subscription "skip TLS verification" flag for the URL fetcher.
-- Lets the hub aggregate from trusted upstreams whose TLS cert is self-signed
-- or expired (e.g. a 3X-UI subscription service with a lapsed Let's Encrypt
-- cert). Default 0 (verify) — opt-in only, since skipping verification removes
-- MITM protection on the fetch (which carries node configs).

ALTER TABLE subscriptions ADD COLUMN allow_insecure INTEGER NOT NULL DEFAULT 0;
