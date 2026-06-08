import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  AlertRule,
  CreateAlertRuleRequest,
  PagedResponse,
  UpdateAlertRuleRequest,
} from "@/types/api";

export function useAlertRulesQuery() {
  return useQuery({
    queryKey: queryKeys.alertRule.list(),
    queryFn: () => apiFetch<PagedResponse<AlertRule>>("/api/alert-rules"),
  });
}

export function useCreateAlertRuleMutation() {
  return useMutation({
    mutationFn: (data: CreateAlertRuleRequest) =>
      apiFetch<AlertRule>("/api/alert-rules", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.alertRule.all() });
    },
  });
}

export function useUpdateAlertRuleMutation() {
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateAlertRuleRequest }) =>
      apiFetch<AlertRule>(`/api/alert-rules/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.alertRule.all() });
    },
  });
}

export function useDeleteAlertRuleMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/alert-rules/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.alertRule.all() });
    },
  });
}
