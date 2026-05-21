import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import type { OTAHistoryItem, OTAReleaseInfo } from "@/types/api";

// ─── Query keys ──────────────────────────────────────────────────────────────
// Kept local because OTA is admin-only and the rest of the app does not need
// to invalidate against these keys.

const otaKeys = {
  all: () => ["ota"] as const,
  status: () => ["ota", "status"] as const,
  history: () => ["ota", "history"] as const,
};

// ─── Hooks ───────────────────────────────────────────────────────────────────

/**
 * GET /api/admin/ota/status — cached release info (no GitHub call).
 *
 * Light-weight read; safe to mount on the OTA page and have it refresh in the
 * background while the user reviews the changelog.
 */
export function useOtaStatus() {
  return useQuery({
    queryKey: otaKeys.status(),
    queryFn: () => apiFetch<OTAReleaseInfo>("/api/admin/ota/status"),
    // Status only reflects the cached release. Refreshing every minute keeps
    // the UI in sync after a background daily-poll updates the cache.
    refetchInterval: 60_000,
  });
}

/**
 * GET /api/admin/ota/check — forces an immediate GitHub poll. Used by the
 * "Check now" button so admins can refresh without waiting 24 hours.
 */
export function useOtaCheck() {
  return useMutation({
    mutationFn: () => apiFetch<OTAReleaseInfo>("/api/admin/ota/check"),
    onSuccess: (data) => {
      // Prime the status cache so the UI doesn't briefly flash stale data.
      queryClient.setQueryData(otaKeys.status(), data);
    },
  });
}

/**
 * GET /api/admin/ota/history — in-memory log of past upgrade attempts.
 */
export function useOtaHistory() {
  return useQuery({
    queryKey: otaKeys.history(),
    queryFn: () => apiFetch<OTAHistoryItem[]>("/api/admin/ota/history"),
  });
}

/**
 * POST /api/admin/ota/apply — kick off the download / verify / restart flow.
 *
 * The endpoint returns 202 immediately; real progress is delivered through
 * the SSE channel (`ota_progress` events). Consumers should subscribe to the
 * stream BEFORE calling this mutation so no events are lost.
 */
export function useApplyOta() {
  return useMutation({
    mutationFn: () =>
      apiFetch<{ accepted: boolean; target_version: string }>(
        "/api/admin/ota/apply",
        { method: "POST" },
      ),
    onSuccess: () => {
      // History will fill in once Apply completes; pre-invalidate so the
      // refetch fires automatically when the SSE "done" event lands.
      queryClient.invalidateQueries({ queryKey: otaKeys.history() });
    },
  });
}
