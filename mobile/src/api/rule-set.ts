import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  PagedResponse,
  RuleSetProvider,
  RuleSetPreset,
  CreateRuleSetRequest,
} from "../types/api";

export function useRuleSetsQuery() {
  return useQuery({
    queryKey: ["rule-set", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<RuleSetProvider>>(
        "/api/rule-sets?page=1&page_size=200",
      ),
  });
}

export function useSyncAllRuleSets() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<void>("/api/rule-sets/sync-all", { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rule-set"] });
    },
  });
}

export function useRuleSetPresets() {
  return useQuery({
    queryKey: ["rule-set", "presets"],
    queryFn: () => apiFetch<RuleSetPreset[]>("/api/rule-sets/presets"),
    enabled: false,
  });
}

export function useCreateRuleSet() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateRuleSetRequest) =>
      apiFetch("/api/rule-sets", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rule-set"] });
    },
  });
}

export function useDeleteRuleSet() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/api/rule-sets/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["rule-set"] });
    },
  });
}
