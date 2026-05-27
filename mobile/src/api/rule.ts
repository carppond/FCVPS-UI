import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  PagedResponse,
  CustomRule,
  RuleTemplate,
  CreateRuleRequest,
  UpdateRuleRequest,
} from "../types/api";

export function useRulesQuery() {
  return useQuery({
    queryKey: ["rule", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<CustomRule>>("/api/rules?page=1&page_size=200"),
  });
}

export function useRuleTemplates() {
  return useQuery({
    queryKey: ["rule", "templates"],
    queryFn: () => apiFetch<RuleTemplate[]>("/api/rules/templates"),
  });
}

export function useCreateRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateRuleRequest) =>
      apiFetch("/api/rules", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rule"] });
    },
  });
}

export function useUpdateRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateRuleRequest }) =>
      apiFetch(`/api/rules/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rule"] });
    },
  });
}

export function useDeleteRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/api/rules/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rule"] });
    },
  });
}
