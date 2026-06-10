/**
 * Classify a raw subscription sync error (the Go error string stored by the
 * hub) into a known kind so the UI can show an actionable, localized hint
 * instead of the bare error text. Mirror of web/src/lib/sync-error.ts.
 */
export type SyncErrorKind = "tls_cert";

const TLS_CERT_RE = /x509:|tls: failed to verify/i;

export function classifySyncError(error?: string | null): SyncErrorKind | null {
  if (!error) return null;
  if (TLS_CERT_RE.test(error)) return "tls_cert";
  return null;
}
