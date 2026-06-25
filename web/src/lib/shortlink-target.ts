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

/**
 * displayShortUrl re-roots a backend-composed short URL onto the browser's
 * current origin (scheme + host + port). Behind a reverse proxy that forwards
 * `Host $host`, the server-composed short_url loses its port (e.g. ":8443");
 * the admin is browsing at the correct origin, so we trust window.location to
 * keep the port — matching how the subscription share card builds its link.
 */
export function displayShortUrl(shortUrl: string): string {
  if (typeof window === "undefined" || !shortUrl) return shortUrl;
  const path = shortUrl.replace(/^https?:\/\/[^/]+/i, "");
  return path.startsWith("/") ? window.location.origin + path : shortUrl;
}
