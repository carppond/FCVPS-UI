import { safeGet, safeSet } from "@/lib/storage";

const STORAGE_KEY = "silent_prefix";

/**
 * Silent-mode entry URLs look like /_app/<32-hex>/<route...>. Strict regex so
 * a route that happens to start with /_app/<short> (e.g. typo, debug path)
 * never gets misinterpreted as a silent-mode prefix.
 */
const SILENT_PREFIX_RE = /^\/_app\/([0-9a-f]{32})(\/|$)/i;

/**
 * Read the silent-mode URL prefix from localStorage.
 * Returns empty string if not set (no prefix).
 */
export function getPrefix(): string {
  return safeGet(STORAGE_KEY) ?? "";
}

/** Persist the silent-mode URL prefix to localStorage. */
export function setPrefix(prefix: string): void {
  safeSet(STORAGE_KEY, prefix);
}

/**
 * Prepend the silent prefix to a path.
 * If prefix is empty, the original path is returned unchanged.
 *
 * @example
 *   setPrefix("/_app/abc123");
 *   prefixedPath("/api/auth/login"); // "/_app/abc123/api/auth/login"
 */
export function prefixedPath(path: string): string {
  const prefix = getPrefix();
  if (!prefix) return path;
  // Avoid double slashes.
  const cleanPrefix = prefix.endsWith("/") ? prefix.slice(0, -1) : prefix;
  const cleanPath = path.startsWith("/") ? path : `/${path}`;
  return `${cleanPrefix}${cleanPath}`;
}

/**
 * Parse a pathname for a silent-mode prefix.
 *
 * The hub auto-generates a 32-hex `silent_mode_prefix` at boot and requires
 * every non-whitelisted request to enter via `/_app/<prefix>/...`. The SPA
 * must learn this value on first load so subsequent `apiFetch` calls go
 * through the prefix transparently (the middleware then strips it before
 * dispatching to the canonical handler routes).
 *
 * Returns `null` if the path does not start with a valid prefix.
 *
 * @example
 *   parseSilentPrefix("/_app/abc...32hex.../login")
 *     → { prefix: "/_app/abc...", strippedPath: "/login" }
 */
export function parseSilentPrefix(
  pathname: string,
): { prefix: string; strippedPath: string } | null {
  const m = pathname.match(SILENT_PREFIX_RE);
  if (!m) return null;
  const prefix = `/_app/${m[1].toLowerCase()}`;
  const strippedPath = pathname.slice(prefix.length) || "/";
  return { prefix, strippedPath };
}

/**
 * SPA-boot entry point: if the current URL carries a silent-mode prefix,
 * persist it to localStorage and strip it from the browser URL so the
 * router matches canonical routes like `/login` instead of failing on
 * `/_app/<hex>/login`. Idempotent.
 *
 * Returns true when a prefix was extracted (mainly for tests).
 */
export function extractSilentPrefixFromURL(): boolean {
  if (typeof window === "undefined") return false;
  const parsed = parseSilentPrefix(window.location.pathname);
  if (!parsed) return false;

  if (getPrefix() !== parsed.prefix) {
    setPrefix(parsed.prefix);
  }
  const newUrl =
    parsed.strippedPath + window.location.search + window.location.hash;
  window.history.replaceState(null, "", newUrl);
  return true;
}
