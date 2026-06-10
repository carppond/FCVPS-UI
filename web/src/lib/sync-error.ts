/**
 * Classify a raw subscription sync error (stored server-side as the Go error
 * string) into a known kind so the UI can show an actionable, localized hint
 * instead of the bare error text.
 *
 * The raw strings come from Go's stdlib and are stable across versions:
 *   - "tls: failed to verify certificate: x509: certificate has expired …"
 *   - "x509: certificate signed by unknown authority"
 *   - "x509: cannot validate certificate for <ip> …"
 */
export type SyncErrorKind = "tls_cert";

const TLS_CERT_RE = /x509:|tls: failed to verify/i;

export function classifySyncError(error?: string | null): SyncErrorKind | null {
  if (!error) return null;
  if (TLS_CERT_RE.test(error)) return "tls_cert";
  return null;
}
