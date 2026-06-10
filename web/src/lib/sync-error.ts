/**
 * Classify a raw network/fetch error (the Go error string produced by the
 * hub, e.g. subscription sync failures) into a known kind so the UI can show
 * an actionable, localized hint instead of the bare error text.
 *
 * The raw strings come from Go's stdlib and are stable across versions:
 *   - "tls: failed to verify certificate: x509: certificate has expired …"
 *   - "x509: certificate signed by unknown authority"
 *   - "context deadline exceeded (Client.Timeout exceeded …)"
 *   - "dial tcp 1.2.3.4:443: connect: connection refused"
 *   - "lookup example.com: no such host"
 */
export type SyncErrorKind = "tls_cert" | "timeout" | "refused" | "dns";

const PATTERNS: Array<[SyncErrorKind, RegExp]> = [
  ["tls_cert", /x509:|tls: failed to verify/i],
  ["timeout", /context deadline exceeded|Client\.Timeout|i\/o timeout|handshake timeout/i],
  ["refused", /connection refused/i],
  ["dns", /no such host/i],
];

export function classifySyncError(error?: string | null): SyncErrorKind | null {
  if (!error) return null;
  for (const [kind, re] of PATTERNS) {
    if (re.test(error)) return kind;
  }
  return null;
}
