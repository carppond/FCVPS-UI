/**
 * M-OPS Short-link API client (T-28).
 *
 * Endpoints covered:
 *   GET    /api/shortlinks
 *   POST   /api/shortlinks
 *   DELETE /api/shortlinks/:fileCode/:userCode
 *
 * The public redirect path /s/:code is not a TanStack query — admins paste
 * the short URL into the address bar.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import type { CreateShortLinkRequest, ShortLink } from "@/types/api";

// ─── Query keys ─────────────────────────────────────────────────────────────

const shortlinkKeys = {
  all: () => ["shortlink"] as const,
  list: () => ["shortlink", "list"] as const,
};

// ─── Hooks ──────────────────────────────────────────────────────────────────

/** GET /api/shortlinks — current user's short links, newest first. */
export function useShortLinks() {
  return useQuery({
    queryKey: shortlinkKeys.list(),
    queryFn: () => apiFetch<ShortLink[]>("/api/shortlinks"),
  });
}

/** POST /api/shortlinks — create a new short link. */
export function useCreateShortLink() {
  return useMutation({
    mutationFn: (payload: CreateShortLinkRequest) =>
      apiFetch<ShortLink>("/api/shortlinks", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: shortlinkKeys.list() });
    },
  });
}

/** DELETE /api/shortlinks/:fileCode/:userCode — delete by composite code. */
export function useDeleteShortLink() {
  return useMutation({
    mutationFn: ({ fileCode, userCode }: { fileCode: string; userCode: string }) =>
      apiFetch<{ deleted: boolean }>(
        `/api/shortlinks/${encodeURIComponent(fileCode)}/${encodeURIComponent(userCode)}`,
        { method: "DELETE" },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: shortlinkKeys.list() });
    },
  });
}
