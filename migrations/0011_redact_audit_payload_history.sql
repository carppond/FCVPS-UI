-- 0011_redact_audit_payload_history.sql
-- Security cleanup: before the SummarizePayload masking was wired into the
-- audit persist path, raw request bodies were stored verbatim in
-- audit_logs.payload — so historical rows for login / change-password /
-- notify-channel / disable-2FA actions may contain PLAINTEXT passwords,
-- tokens and secrets. This one-shot scrub redacts any historical payload that
-- looks like it carries a sensitive field. Only the payload is cleared; the
-- action / user / IP / timestamp metadata is preserved for the audit trail.
--
-- New rows are already masked at write time (internal/audit/logger.go), so this
-- only affects data written before that fix shipped.

UPDATE audit_logs
   SET payload = '{"_redacted":"sensitive payload scrubbed by migration 0011"}'
 WHERE payload IS NOT NULL
   AND (
        payload LIKE '%password%'
     OR payload LIKE '%pass_word%'
     OR payload LIKE '%token%'
     OR payload LIKE '%secret%'
     OR payload LIKE '%recovery%'
     OR payload LIKE '%totp%'
     OR payload LIKE '%otp%'
     OR payload LIKE '%api_key%'
     OR payload LIKE '%apikey%'
     OR payload LIKE '%credential%'
   );
