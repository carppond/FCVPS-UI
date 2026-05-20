import { safeGet, safeSet } from "@/lib/storage";

const STORAGE_KEY = "silent_prefix";

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
