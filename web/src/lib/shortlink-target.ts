/**
 * parseShortLinkTarget — extract subscription info from a short link's
 * target_url for display purposes.
 *
 * Subscription share URLs (the only short links the app itself creates, via
 * the subscription detail share card) have the shape
 * `<origin>[/_app/<prefix>]/download/<name>?token=...&target=<client>`.
 * Hand-created short links pointing anywhere else return null and the UI
 * falls back to showing just the raw target URL.
 *
 * Deliberately string-matched instead of `new URL()`: React Native's URL
 * implementation is incomplete (mobile duplicates this helper) and the
 * regex keeps both platforms byte-identical.
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
