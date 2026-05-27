import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, CustomRule } from "../types/api";

export function useRulesQuery() {
  return useQuery({
    queryKey: ["rule", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<CustomRule>>("/api/rules?page=1&page_size=200"),
  });
}
