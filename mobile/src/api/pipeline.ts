import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, Pipeline } from "../types/api";

export function usePipelinesQuery() {
  return useQuery({
    queryKey: ["pipeline", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<Pipeline>>(
        "/api/pipelines?page=1&page_size=100",
      ),
  });
}
