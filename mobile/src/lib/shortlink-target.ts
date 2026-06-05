/**
 * parseShortLinkTarget — extract subscription info from a short link's
 * target_url for display purposes. Mirror of web/src/lib/shortlink-target.ts
 * (web and mobile do not share code; keep both copies byte-identical).
 *
 * Subscription share URLs have the shape
 * `<origin>[/_app/<prefix>]/download/<name>?token=...&target=<client>`.
 * Any other URL (hand-created short link) returns null.
 *
 * Deliberately string-matched instead of `new URL()`: React Native's URL
 * implementation is incomplete.
 */
export interface ShortLinkTarget {
  /** Decoded subscription name from the /download/<name> path segment. */
  subscriptionName: string;
  /** Client target from the ?target= query param (e.g. "clash", "singbox"). */
  client?: string;
}

export function parseShortLinkTarget(targetUrl: string): ShortLinkTarget | null {
  const m = /\/download\/([^/?#]+)/.exec(targetUrl);
  if (!m) return null;
  return {
    subscriptionName: tryDecode(m[1]),
    client: matchQueryParam(targetUrl, "target"),
  };
}

function matchQueryParam(url: string, key: string): string | undefined {
  const m = new RegExp(`[?&]${key}=([^&#]+)`).exec(url);
  return m ? tryDecode(m[1]) : undefined;
}

function tryDecode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s; // keep the raw segment when decoding fails
  }
}
