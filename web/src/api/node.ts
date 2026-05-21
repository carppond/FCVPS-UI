/**
 * M-NODE API client (T-11).
 *
 * Mirrors the Go handler surface declared in internal/handler/node_handler.go
 * + tcping_handler.go. Every mutation invalidates the relevant TanStack query
 * key from `queryKeys.node` so the list / detail views refetch in step with
 * the persisted data.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  AddNodeRequest,
  Node,
  NodeWithLatency,
  PagedResponse,
  TCPingRequest,
  TCPingResponse,
  TCPingResult,
  UpdateNodeRequest,
} from "@/types/api";

// ─── Query params ────────────────────────────────────────────────────────────

export interface ListNodesParams {
  page?: number;
  pageSize?: number;
  search?: string;
  protocol?: string;
  tag?: string;
  subscriptionId?: string;
  sort?: "latency_asc" | "latency_desc" | "created_asc" | "created_desc";
}

function buildNodesQuery(params: ListNodesParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.search) search.set("search", params.search);
  if (params.protocol) search.set("protocol", params.protocol);
  if (params.tag) search.set("tag", params.tag);
  if (params.subscriptionId)
    search.set("subscription_id", params.subscriptionId);
  if (params.sort) search.set("sort", params.sort);
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

// ─── Queries ─────────────────────────────────────────────────────────────────

/** GET /api/nodes — paged list with optional filters / sort. */
export function useNodesQuery(params: ListNodesParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.node.list(params.subscriptionId), params],
    queryFn: () =>
      apiFetch<PagedResponse<NodeWithLatency>>(
        `/api/nodes${buildNodesQuery(params)}`,
      ),
  });
}

/** GET /api/nodes/{id} — single-node detail. */
export function useNodeQuery(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.node.detail(id ?? ""),
    queryFn: () => apiFetch<NodeWithLatency>(`/api/nodes/${id}`),
    enabled: Boolean(id),
  });
}

// ─── Mutations ──────────────────────────────────────────────────────────────

/** POST /api/subscriptions/{subID}/nodes — manual create. */
export function useCreateNodeMutation() {
  return useMutation({
    mutationFn: ({
      subscriptionId,
      payload,
    }: {
      subscriptionId: string;
      payload: AddNodeRequest;
    }) =>
      apiFetch<Node>(`/api/subscriptions/${subscriptionId}/nodes`, {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.node.all() });
    },
  });
}

/** PATCH /api/nodes/{id} — partial update of tags / chain_parent_id. */
export function useUpdateNodeMutation() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: UpdateNodeRequest;
    }) =>
      apiFetch<NodeWithLatency>(`/api/nodes/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.node.all() });
      queryClient.setQueryData(queryKeys.node.detail(data.id), data);
    },
  });
}

/** DELETE /api/nodes/{id} — remove a single node. */
export function useDeleteNodeMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/nodes/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.node.all() });
    },
  });
}

/** POST /api/nodes/{id}/copy-uri — returns the raw_uri for the clipboard. */
export function useCopyNodeURIMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<{ raw_uri: string }>(`/api/nodes/${id}/copy-uri`, {
        method: "POST",
      }),
  });
}

// ─── TCPing ─────────────────────────────────────────────────────────────────

/** POST /api/tcping/single — probe (server, port) without persistence. */
export function useTCPingSingleMutation() {
  return useMutation({
    mutationFn: (payload: { server: string; port: number; timeout_ms?: number }) =>
      apiFetch<{ latency_ms: number; reachable: boolean; error?: string }>(
        "/api/tcping/single",
        { method: "POST", body: JSON.stringify(payload) },
      ),
  });
}

/**
 * POST /api/tcping/batch — probes up to 200 nodes with concurrency=50.
 * After the call resolves we invalidate node queries so cached latency
 * reflects the new measurements.
 */
export function useTCPingBatchMutation() {
  return useMutation({
    mutationFn: (payload: TCPingRequest) =>
      apiFetch<TCPingResponse>("/api/tcping/batch", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.node.all() });
    },
  });
}

/** POST /api/nodes/{id}/tcping — single-node probe + persist. */
export function useTCPingNodeMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<TCPingResult>(`/api/nodes/${id}/tcping`, { method: "POST" }),
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.node.detail(id) });
      queryClient.invalidateQueries({ queryKey: queryKeys.node.list() });
    },
  });
}
