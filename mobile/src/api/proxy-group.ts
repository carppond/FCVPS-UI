import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  PagedResponse,
  ProxyGroupCategory,
  ProxyGroupPreset,
  CreateProxyGroupRequest,
} from "../types/api";

export function useProxyGroupsQuery() {
  return useQuery({
    queryKey: ["proxy-group", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<ProxyGroupCategory>>(
        "/api/proxy-groups?page=1&page_size=200",
      ),
  });
}

export function useProxyGroupPresets() {
  return useQuery({
    queryKey: ["proxy-group", "presets"],
    queryFn: () => apiFetch<ProxyGroupPreset[]>("/api/proxy-groups/presets"),
    enabled: false,
  });
}

export function useCreateProxyGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateProxyGroupRequest) =>
      apiFetch("/api/proxy-groups", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["proxy-group"] });
    },
  });
}
