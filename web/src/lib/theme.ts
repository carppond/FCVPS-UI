import { safeGet, safeSet } from "@/lib/storage";

const STORAGE_KEY = "sgvps_theme";

/** Supported theme values. */
export type Theme = "light" | "dark" | "system";

/** Resolve 'system' to an actual 'light' | 'dark' value. */
function resolveTheme(theme: Theme): "light" | "dark" {
  if (theme !== "system") return theme;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

/**
 * Apply a theme by setting data-theme on <html> and persisting to localStorage.
 * The 'system' value resolves to the OS preference at call time.
 */
export function applyTheme(theme: Theme): void {
  const resolved = resolveTheme(theme);
  document.documentElement.setAttribute("data-theme", resolved);
  safeSet(STORAGE_KEY, theme);
}

/** Return the currently persisted theme preference (default: 'dark'). */
export function getCurrentTheme(): Theme {
  const stored = safeGet(STORAGE_KEY);
  if (stored === "light" || stored === "dark" || stored === "system") return stored;
  return "dark";
}

/**
 * Watch for OS-level color scheme changes.
 * The callback receives the resolved 'light' | 'dark' value.
 * Returns a cleanup function.
 */
export function watchSystemTheme(cb: (resolved: "light" | "dark") => void): () => void {
  const mq = window.matchMedia("(prefers-color-scheme: dark)");
  const handler = (e: MediaQueryListEvent) => cb(e.matches ? "dark" : "light");
  mq.addEventListener("change", handler);
  return () => mq.removeEventListener("change", handler);
}
