import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { AgentListItem, PagedResponse } from "../types/api";

export function useAgentsQuery() {
  return useQuery({
    queryKey: ["agent", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<AgentListItem>>("/api/agents?page=1&page_size=100"),
  });
}
