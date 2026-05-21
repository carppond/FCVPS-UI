/**
 * T-29: command-palette controller hook.
 *
 * Centralises the open/close state, the global keyboard binding (Cmd+K /
 * Ctrl+K), and the "recent commands" history so any consumer can wire its
 * shortcut without re-implementing the listener.
 *
 * The store is intentionally a plain Zustand slice rather than React context
 * so non-tree callers (e.g. the top-bar search button) can `useCmdKStore`
 * to flip the dialog without prop-drilling.
 */
import { useEffect, useCallback } from "react";
import { create } from "zustand";

interface CmdKState {
  open: boolean;
  setOpen: (open: boolean) => void;
  toggle: () => void;
}

/**
 * Tiny store. Kept off `persist()` because we only need open state in memory.
 * Exposed via `useCmdKStore` so tests can call `useCmdKStore.setState({ open: false })`
 * between cases without re-mounting the component tree.
 */
export const useCmdKStore = create<CmdKState>((set) => ({
  open: false,
  setOpen: (open) => set({ open }),
  toggle: () => set((s) => ({ open: !s.open })),
}));

/**
 * Registers the global Cmd+K / Ctrl+K listener at the document level. Calling
 * this hook in a single mount point (the layout-level <CmdK />) is enough —
 * mounting it again is a no-op but harmless.
 *
 * The handler intentionally swallows the default browser behaviour (Chrome
 * focuses its address bar on Cmd+K, which would tear the user out of the
 * SPA) and only fires when the active element is not a contentEditable /
 * input / textarea — otherwise the user typing "k" while editing would pop
 * the palette mid-word.
 */
export function useCmdKShortcut(): void {
  const toggle = useCmdKStore((s) => s.toggle);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const isToggleCombo = (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k";
      if (!isToggleCombo) return;
      const target = e.target as HTMLElement | null;
      // The cmd-k own input is allowed — we close on second press from inside.
      const tagName = target?.tagName?.toLowerCase();
      const isEditable =
        tagName === "input" ||
        tagName === "textarea" ||
        Boolean(target?.isContentEditable);
      // Always trigger; ignore the editable check ONLY when the input belongs
      // to a non-cmd-k surface. We still call preventDefault to stop the
      // browser combo from leaking out (e.g. Firefox uses Cmd+K for search).
      if (isEditable && !target?.closest("[data-cmdk-root]")) return;
      e.preventDefault();
      toggle();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [toggle]);
}

/** Public hook for callers that only care about open/close state. */
export function useCmdK() {
  const open = useCmdKStore((s) => s.open);
  const setOpen = useCmdKStore((s) => s.setOpen);
  const close = useCallback(() => setOpen(false), [setOpen]);
  return { open, setOpen, close };
}

// ── recent commands persistence ─────────────────────────────────────────────

const RECENTS_KEY = "sgvps_cmdk_recents";
const RECENTS_MAX = 5;

export interface CmdKRecent {
  /** Stable id ("page:/dashboard", "action:sync_all", "resource:node/abc"). */
  id: string;
  /** Human-readable label fetched at insert time (translated). */
  label: string;
  /** Optional href for navigation entries. */
  to?: string;
  /** Stored as unix ms so we can age them out if needed. */
  at: number;
}

export function loadRecents(): CmdKRecent[] {
  try {
    const raw = window.localStorage.getItem(RECENTS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as CmdKRecent[];
    return Array.isArray(parsed) ? parsed.slice(0, RECENTS_MAX) : [];
  } catch {
    return [];
  }
}

export function pushRecent(entry: Omit<CmdKRecent, "at">): void {
  if (typeof window === "undefined") return;
  const next: CmdKRecent = { ...entry, at: Date.now() };
  const filtered = loadRecents().filter((r) => r.id !== entry.id);
  filtered.unshift(next);
  const trimmed = filtered.slice(0, RECENTS_MAX);
  try {
    window.localStorage.setItem(RECENTS_KEY, JSON.stringify(trimmed));
  } catch {
    /* quota / privacy mode — swallow */
  }
}

export function clearRecents(): void {
  try {
    window.localStorage.removeItem(RECENTS_KEY);
  } catch {
    /* ignore */
  }
}
