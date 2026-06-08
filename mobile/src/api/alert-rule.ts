import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  AlertRule,
  CreateAlertRuleRequest,
  PagedResponse,
  UpdateAlertRuleRequest,
} from "../types/api";

export function useAlertRulesQuery() {
  return useQuery({
    queryKey: ["alert-rule", "list"],
    queryFn: () => apiFetch<PagedResponse<AlertRule>>("/api/alert-rules"),
  });
}

export function useCreateAlertRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateAlertRuleRequest) =>
      apiFetch<AlertRule>("/api/alert-rules", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert-rule"] });
    },
  });
}

export function useUpdateAlertRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateAlertRuleRequest }) =>
      apiFetch<AlertRule>(`/api/alert-rules/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert-rule"] });
    },
  });
}

export function useDeleteAlertRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/alert-rules/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert-rule"] });
    },
  });
}
