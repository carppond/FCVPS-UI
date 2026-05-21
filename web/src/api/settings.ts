import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { prefixedPath } from "@/lib/silent-prefix";
import type { SilentModeResponse } from "@/types/api";

// ─── Query keys ──────────────────────────────────────────────────────────────
// Settings + silent-mode rotation live in the same scope so a successful
// rotation invalidates the cached settings (the prefix field changes).

const settingsKeys = {
  all: () => ["settings"] as const,
  map: () => ["settings", "map"] as const,
};

// SettingsMap mirrors the backend response: a raw key→value map where the
// server has already masked sensitive entries (passwords, bot tokens) with the
// literal "******".
export type SettingsMap = Record<string, string>;

/** GET /api/admin/settings — full k/v map with sensitive keys masked. */
export function useSettings() {
  return useQuery({
    queryKey: settingsKeys.map(),
    queryFn: () => apiFetch<SettingsMap>("/api/admin/settings"),
  });
}

/**
 * PUT /api/admin/settings — batch update. The caller passes the changed
 * subset; values equal to "******" are ignored server-side so the form can
 * round-trip masked secrets without forcing the admin to re-enter them.
 */
export function useUpdateSettings() {
  return useMutation({
    mutationFn: (payload: SettingsMap) =>
      apiFetch<SettingsMap>("/api/admin/settings", {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: settingsKeys.map() });
    },
  });
}

/**
 * POST /api/admin/silent-mode/rotate — generates a new 32-hex prefix, purges
 * every active session, and returns the new login URL. The URL is the ONLY
 * surface that ever shows the clear-text prefix — subsequent GET /settings
 * responses mask it again.
 */
export function useRotateSilentMode() {
  return useMutation({
    mutationFn: () =>
      apiFetch<SilentModeResponse>("/api/admin/silent-mode/rotate", {
        method: "POST",
      }),
    onSuccess: () => {
      // Settings map carries the masked prefix; refresh so the post-rotate
      // value lines up with the new state.
      queryClient.invalidateQueries({ queryKey: settingsKeys.map() });
    },
  });
}

/**
 * POST /api/admin/backup — triggers backup creation and returns the binary
 * blob (tar.gz). We bypass apiFetch because that helper assumes JSON.
 *
 * Returns a Blob the caller can hand to URL.createObjectURL → <a download>.
 */
export async function downloadBackup(token: string | undefined): Promise<Blob> {
  const url = prefixedPath("/api/admin/backup");
  const headers: Record<string, string> = {};
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const res = await fetch(url, { method: "POST", headers });
  if (!res.ok) {
    // Best-effort JSON error body — fall back to status text otherwise.
    let message = `Backup failed (${res.status})`;
    try {
      const body = (await res.json()) as { message?: string };
      if (body.message) message = body.message;
    } catch {
      /* ignore */
    }
    throw new Error(message);
  }
  return res.blob();
}

/**
 * POST /api/admin/backup/restore — multipart upload of a tar.gz; the server
 * spools to disk, validates, then atomically swaps the live DB file.
 *
 * The promise resolves with `{ restored: true, restart_required: true }` on
 * success; callers MUST surface a "service is restarting" banner because the
 * hub will exit shortly after.
 */
export async function restoreBackup(
  file: File,
  token: string | undefined,
): Promise<{ restored: boolean; restart_required: boolean }> {
  const url = prefixedPath("/api/admin/backup/restore");
  const headers: Record<string, string> = {};
  if (token) headers["Authorization"] = `Bearer ${token}`;
  const formData = new FormData();
  formData.append("archive", file);
  const res = await fetch(url, { method: "POST", headers, body: formData });
  let body: { message?: string; data?: { restored: boolean; restart_required: boolean } };
  try {
    body = await res.json();
  } catch {
    throw new Error(`Restore failed (${res.status})`);
  }
  if (!res.ok) {
    throw new Error(body.message ?? `Restore failed (${res.status})`);
  }
  return body.data ?? { restored: false, restart_required: false };
}
