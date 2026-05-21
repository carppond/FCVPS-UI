/**
 * M-RULE API client (T-12).
 *
 * Mirrors the Go handler surface declared in internal/handler/rule_handler.go.
 * Every mutation invalidates `queryKeys.rule.*` so the list / preview views
 * refetch in step with the persisted data.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  CreateRuleRequest,
  CustomRule,
  PagedResponse,
  RuleTemplate,
  RuleType,
  UpdateRuleOrderRequest,
  UpdateRuleRequest,
} from "@/types/api";

// ─── Query params ────────────────────────────────────────────────────────────

export interface ListRulesParams {
  page?: number;
  pageSize?: number;
  type?: RuleType;
  keyword?: string;
}

/** Wire-shape returned by GET /api/rules/preview/{subID}. */
export interface RulePreviewResponse {
  base_yaml: string;
  final_yaml: string;
  rule_count: number;
}

function buildRulesQuery(params: ListRulesParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.type) search.set("type", params.type);
  if (params.keyword) search.set("keyword", params.keyword);
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

// ─── Queries ─────────────────────────────────────────────────────────────────

/** GET /api/rules — paged list with optional filters. */
export function useRulesQuery(params: ListRulesParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.rule.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<CustomRule>>(
        `/api/rules${buildRulesQuery(params)}`,
      ),
  });
}

/** GET /api/rules/{id} — single-rule detail. */
export function useRuleQuery(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.rule.detail(id ?? ""),
    queryFn: () => apiFetch<CustomRule>(`/api/rules/${id}`),
    enabled: Boolean(id),
  });
}

/** GET /api/rules/templates — built-in template catalog (3 entries). */
export function useRuleTemplatesQuery() {
  return useQuery({
    queryKey: [...queryKeys.rule.all(), "templates"],
    queryFn: () => apiFetch<RuleTemplate[]>("/api/rules/templates"),
    staleTime: Infinity,
  });
}

/**
 * GET /api/rules/preview/{subID} — apply every enabled rule on top of the
 * subscription's current Clash YAML. Suspended until a subscription is picked
 * by the parent component.
 */
export function useRulePreviewQuery(subscriptionId: string | undefined) {
  return useQuery({
    queryKey: [...queryKeys.rule.all(), "preview", subscriptionId ?? ""],
    queryFn: () =>
      apiFetch<RulePreviewResponse>(`/api/rules/preview/${subscriptionId}`),
    enabled: Boolean(subscriptionId),
  });
}

// ─── Mutations ──────────────────────────────────────────────────────────────

/** POST /api/rules — create a new rule. */
export function useCreateRuleMutation() {
  return useMutation({
    mutationFn: (payload: CreateRuleRequest) =>
      apiFetch<CustomRule>("/api/rules", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rule.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.rule.all() });
    },
  });
}

/** PUT /api/rules/{id} — partial update (empty fields are preserved). */
export function useUpdateRuleMutation() {
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateRuleRequest }) =>
      apiFetch<CustomRule>(`/api/rules/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: (rule) => {
      queryClient.setQueryData(queryKeys.rule.detail(rule.id), rule);
      queryClient.invalidateQueries({ queryKey: queryKeys.rule.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.rule.all() });
    },
  });
}

/** DELETE /api/rules/{id} — remove the rule. */
export function useDeleteRuleMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/rules/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rule.all() });
    },
  });
}

/** POST /api/rules/reorder — atomic sort update for a batch of rules. */
export function useReorderRulesMutation() {
  return useMutation({
    mutationFn: (payload: UpdateRuleOrderRequest) =>
      apiFetch<{ updated: number }>("/api/rules/reorder", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.rule.all() });
    },
  });
}
