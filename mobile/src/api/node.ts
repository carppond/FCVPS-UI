import { useQuery, useMutation } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, Node, TCPingRequest, TCPingResponse } from "../types/api";

export function useNodesQuery() {
  return useQuery({
    queryKey: ["node", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<Node>>("/api/nodes?page=1&page_size=500"),
  });
}

export function useTcpingMutation() {
  return useMutation({
    mutationFn: (data: TCPingRequest) =>
      apiFetch<TCPingResponse>("/api/tcping/batch", {
        method: "POST",
        body: JSON.stringify(data),
      }),
  });
}
