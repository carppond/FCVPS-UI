/**
 * Classify a raw network/fetch error (the Go error string produced by the
 * hub) into a known kind so the UI can show an actionable, localized hint
 * instead of the bare error text. Mirror of web/src/lib/sync-error.ts.
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
