/**
 * M-RULE-SET API client — mirrors GET/POST/PUT/DELETE /api/rule-sets/* in
 * internal/handler/rule_set_handler.go. Each mutation invalidates
 * `queryKeys.ruleSet.*` so the list view refetches in step.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  CreateRuleSetRequest,
  PagedResponse,
  RuleSetPreset,
  RuleSetProvider,
  UpdateRuleSetRequest,
} from "@/types/api";

export interface ListRuleSetsParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
}

function buildRuleSetsQuery(params: ListRuleSetsParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.keyword) search.set("keyword", params.keyword);
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

/** GET /api/rule-sets — paged list. */
export function useRuleSets(params: ListRuleSetsParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.ruleSet.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<RuleSetProvider>>(
        `/api/rule-sets${buildRuleSetsQuery(params)}`,
      ),
  });
}

/** GET /api/rule-sets/{id} — single rule-set detail. */
export function useRuleSet(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.ruleSet.detail(id ?? ""),
    queryFn: () => apiFetch<RuleSetProvider>(`/api/rule-sets/${id}`),
    enabled: Boolean(id),
  });
}

/** GET /api/rule-sets/presets — built-in meta-rules-dat presets (19 entries). */
export function useRuleSetPresets() {
  return useQuery({
    queryKey: queryKeys.ruleSet.presets(),
    queryFn: () => apiFetch<RuleSetPreset[]>("/api/rule-sets/presets"),
    staleTime: Infinity,
  });
}

/** POST /api/rule-sets — create a custom rule provider. */
export function useCreateRuleSet() {
  return useMutation({
    mutationFn: (payload: CreateRuleSetRequest) =>
      apiFetch<RuleSetProvider>("/api/rule-sets", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.ruleSet.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.ruleSet.all() });
    },
  });
}

/** PUT /api/rule-sets/{id} — partial update. */
export function useUpdateRuleSet() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: UpdateRuleSetRequest;
    }) =>
      apiFetch<RuleSetProvider>(`/api/rule-sets/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: (rs) => {
      queryClient.setQueryData(queryKeys.ruleSet.detail(rs.id), rs);
      queryClient.invalidateQueries({ queryKey: queryKeys.ruleSet.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.ruleSet.all() });
    },
  });
}

/** DELETE /api/rule-sets/{id} — remove the rule provider. */
export function useDeleteRuleSet() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/rule-sets/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.ruleSet.all() });
    },
  });
}

/** POST /api/rule-sets/sync-all — sync all enabled rule sets. */
export function useSyncAllRuleSets() {
  return useMutation({
    mutationFn: () =>
      apiFetch<{ ok: number; failed: number }>("/api/rule-sets/sync-all", {
        method: "POST",
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.ruleSet.all() });
    },
  });
}

/**
 * POST /api/rule-sets/{id}/sync — immediate sync (HEAD probe; backend updates
 * last_synced_at / last_sync_status accordingly).
 */
export function useSyncRuleSet() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<RuleSetProvider>(`/api/rule-sets/${id}/sync`, {
        method: "POST",
      }),
    onSuccess: (rs) => {
      queryClient.setQueryData(queryKeys.ruleSet.detail(rs.id), rs);
      queryClient.invalidateQueries({ queryKey: queryKeys.ruleSet.list() });
    },
  });
}
