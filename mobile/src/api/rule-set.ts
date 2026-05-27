import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, RuleSetProvider } from "../types/api";

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
