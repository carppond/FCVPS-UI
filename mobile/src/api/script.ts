import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, Script } from "../types/api";

export function useScriptsQuery() {
  return useQuery({
    queryKey: ["script", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<Script>>(
        "/api/scripts?page=1&page_size=100",
      ),
  });
}
