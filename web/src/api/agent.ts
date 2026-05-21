/**
 * M-AGENT API client (T-16).
 *
 * Mirrors `internal/handler/agent_handler.go` and returns the same DTOs the
 * backend ships (Agent + the agentListItem wrapper with `online` and
 * `latest_metrics`). Mutations invalidate the relevant TanStack query keys
 * so the list / detail views stay in sync after writes.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  AgentCreateResponse,
  AgentListItem,
  AgentMetric,
  CreateAgentRequest,
  PagedResponse,
  RotateTokenResponse,
  UpdateAgentRequest,
} from "@/types/api";

// ─── Query params ────────────────────────────────────────────────────────────

export interface ListAgentsParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
}

export type MetricRange = "1h" | "6h" | "24h";

const RANGE_MS: Record<MetricRange, number> = {
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
  "24h": 24 * 60 * 60 * 1000,
};

function buildAgentsQuery(params: ListAgentsParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.keyword) search.set("keyword", params.keyword);
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

// ─── Queries ─────────────────────────────────────────────────────────────────

/** GET /api/agents — paged list with optional keyword filter. */
export function useAgentsQuery(params: ListAgentsParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.agent.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<AgentListItem>>(
        `/api/agents${buildAgentsQuery(params)}`,
      ),
  });
}

/** GET /api/agents/{id}. */
export function useAgentQuery(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.agent.detail(id ?? ""),
    queryFn: () => apiFetch<AgentListItem>(`/api/agents/${id}`),
    enabled: Boolean(id),
  });
}

/**
 * GET /api/agents/{id}/records — high-frequency metric samples. The backend
 * accepts `from=<unix-ms>&limit=<n>`; we translate the UX-friendly
 * "1h | 6h | 24h" range knob into the right `from`.
 *
 * `limit` is intentionally generous (5760 = 24h × 60 × 4) so the response
 * never gets truncated for the longest range; the backend itself caps it
 * to ~720 per request when called without an override.
 */
export function useAgentMetricsQuery(
  id: string | undefined,
  range: MetricRange = "1h",
) {
  return useQuery({
    queryKey: [...queryKeys.agent.metrics(id ?? ""), range],
    queryFn: () => {
      const from = Date.now() - RANGE_MS[range];
      const limit = range === "24h" ? 5760 : range === "6h" ? 1440 : 720;
      return apiFetch<AgentMetric[]>(
        `/api/agents/${id}/records?from=${from}&limit=${limit}`,
      );
    },
    enabled: Boolean(id),
  });
}

// ─── Mutations ──────────────────────────────────────────────────────────────

/** POST /api/agents — create + return the one-shot plaintext token. */
export function useCreateAgentMutation() {
  return useMutation({
    mutationFn: (payload: CreateAgentRequest) =>
      apiFetch<AgentCreateResponse>("/api/agents", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agent.all() });
    },
  });
}

/** PATCH /api/agents/{id} — rename. */
export function useUpdateAgentMutation() {
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateAgentRequest }) =>
      apiFetch<AgentListItem>(`/api/agents/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agent.all() });
      queryClient.setQueryData(queryKeys.agent.detail(data.id), data);
    },
  });
}

/** DELETE /api/agents/{id}. */
export function useDeleteAgentMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/agents/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agent.all() });
    },
  });
}

/** POST /api/agents/{id}/rotate-token — return new plaintext token. */
export function useRotateTokenMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<RotateTokenResponse>(`/api/agents/${id}/rotate-token`, {
        method: "POST",
      }),
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agent.detail(id) });
    },
  });
}

/**
 * POST /api/agents/{id}/command — dispatch an admin command (restart, etc.)
 * over the WebSocket channel. Returns the assigned `cmd_id` which callers can
 * surface in a toast for traceability.
 */
export function useSendCommandMutation() {
  return useMutation({
    mutationFn: ({
      id,
      cmd,
      args,
    }: {
      id: string;
      cmd: string;
      args?: Record<string, string>;
    }) =>
      apiFetch<{ cmd_id: string }>(`/api/agents/${id}/command`, {
        method: "POST",
        body: JSON.stringify({ cmd, args }),
      }),
  });
}
