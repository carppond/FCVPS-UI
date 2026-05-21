/**
 * M-SUB API client (T-10).
 *
 * Mirrors the Go handler surface declared in internal/handler/subscription_handler.go.
 * Every mutation invalidates the relevant TanStack query key from
 * `queryKeys.subscription` so the list / detail views refetch automatically.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { prefixedPath } from "@/lib/silent-prefix";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import { useAuthStore } from "@/stores/auth-store";
import type {
  APIResponse,
  CreateSubscriptionRequest,
  PagedResponse,
  RotateShareTokenResponse,
  Subscription,
  SubscriptionDetail,
  SyncResult,
  UpdateSubscriptionRequest,
} from "@/types/api";

// Re-export so callers can keep importing from this module without churn.
export type { RotateShareTokenResponse, SubscriptionDetail } from "@/types/api";

// ─── Query params ───────────────────────────────────────────────────────────

export interface ListSubscriptionsParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
  /** Admin-only: when true, list across all users. Maps to owner_id="" + admin role. */
  allUsers?: boolean;
}

function buildSubscriptionsQuery(params: ListSubscriptionsParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.keyword) search.set("keyword", params.keyword);
  if (params.allUsers) search.set("all_users", "true");
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

// ─── Queries ────────────────────────────────────────────────────────────────

/** GET /api/subscriptions — paged list. */
export function useSubscriptionsQuery(params: ListSubscriptionsParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.subscription.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<Subscription>>(
        `/api/subscriptions${buildSubscriptionsQuery(params)}`,
      ),
  });
}

/** GET /api/subscriptions/{id} — single-subscription detail (includes share_token). */
export function useSubscriptionQuery(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.subscription.detail(id ?? ""),
    queryFn: () => apiFetch<SubscriptionDetail>(`/api/subscriptions/${id}`),
    enabled: Boolean(id),
  });
}

// ─── Mutations: CRUD ────────────────────────────────────────────────────────

/** POST /api/subscriptions — create url/manual subscription. */
export function useCreateSubscriptionMutation() {
  return useMutation({
    mutationFn: (payload: CreateSubscriptionRequest) =>
      apiFetch<Subscription>("/api/subscriptions", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.tags() });
    },
  });
}

/** PATCH /api/subscriptions/{id} — modify metadata. */
export function useUpdateSubscriptionMutation() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: UpdateSubscriptionRequest;
    }) =>
      apiFetch<Subscription>(`/api/subscriptions/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: (sub) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.list() });
      queryClient.invalidateQueries({
        queryKey: queryKeys.subscription.detail(sub.id),
      });
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.tags() });
    },
  });
}

/** DELETE /api/subscriptions/{id}. */
export function useDeleteSubscriptionMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/subscriptions/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.node.all() });
    },
  });
}

// ─── Mutations: sync + share + upload ───────────────────────────────────────

/** POST /api/subscriptions/{id}/sync — manual sync trigger. */
export function useSyncSubscriptionMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<SyncResult>(`/api/subscriptions/${id}/sync`, { method: "POST" }),
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.list() });
      queryClient.invalidateQueries({
        queryKey: queryKeys.subscription.detail(id),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.node.list(id),
      });
    },
  });
}

/** POST /api/subscriptions/{id}/rotate-share-token — issues a fresh share_token. */
export function useRotateShareTokenMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<RotateShareTokenResponse>(
        `/api/subscriptions/${id}/rotate-share-token`,
        { method: "POST" },
      ),
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.subscription.detail(id),
      });
    },
  });
}

/**
 * POST /api/subscriptions/upload — multipart/form-data upload of a YAML file.
 *
 * Bypasses apiFetch because that helper sets Content-Type: application/json
 * which would break multipart boundary negotiation. We mirror the auth +
 * silent-prefix + error handling that apiFetch provides.
 */
export interface UploadSubscriptionPayload {
  name: string;
  file: File;
  tags?: string[];
  remark?: string;
  syncInterval?: number;
}

export function useUploadSubscriptionMutation() {
  return useMutation({
    mutationFn: async (payload: UploadSubscriptionPayload) => {
      const fd = new FormData();
      fd.append("name", payload.name);
      fd.append("file", payload.file);
      if (payload.tags && payload.tags.length > 0) {
        fd.append("tags", payload.tags.join(","));
      }
      if (payload.remark) fd.append("remark", payload.remark);
      if (payload.syncInterval !== undefined) {
        fd.append("sync_interval", String(payload.syncInterval));
      }

      const url = prefixedPath("/api/subscriptions/upload");
      const token = useAuthStore.getState().token;
      const headers: Record<string, string> = {};
      if (token) headers["Authorization"] = `Bearer ${token}`;

      const response = await fetch(url, {
        method: "POST",
        body: fd,
        headers,
      });

      let body: APIResponse<Subscription> | null = null;
      try {
        body = (await response.json()) as APIResponse<Subscription>;
      } catch {
        // ignore parse error — handled below
      }

      if (!response.ok || (body && body.code)) {
        const code = body?.code ?? `HTTP_${response.status}`;
        const message = body?.message ?? "Upload failed";
        throw Object.assign(new Error(message), { code, status: response.status });
      }
      return body!.data as Subscription;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.tags() });
    },
  });
}

// ─── Tag suggestions (derived) ──────────────────────────────────────────────

/**
 * GET /api/subscriptions (page_size large) — distinct tag pool for tag-input
 * auto-suggest. There is no dedicated /api/subscriptions/tags endpoint in
 * 04-api-contract.md, so we derive it from the list response. Cached longer
 * (5 min) since tag churn is low.
 */
export function useSubscriptionTagSuggestionsQuery() {
  return useQuery({
    queryKey: queryKeys.subscription.tags(),
    queryFn: async () => {
      const page = await apiFetch<PagedResponse<Subscription>>(
        "/api/subscriptions?page=1&page_size=100",
      );
      const seen = new Set<string>();
      for (const sub of page.items) {
        for (const tag of sub.tags) seen.add(tag);
      }
      return Array.from(seen).sort();
    },
    staleTime: 5 * 60 * 1000,
  });
}
