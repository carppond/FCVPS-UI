import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  PagedResponse,
  VpsAsset,
  VpsAssetSummary,
  UpdateVpsAssetRequest,
} from "../types/api";

export function useVpsAssetsQuery() {
  return useQuery({
    queryKey: ["vps-asset", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<VpsAsset>>("/api/vps-assets?page=1&page_size=500"),
  });
}

export function useVpsAssetSummaryQuery() {
  return useQuery({
    queryKey: ["vps-asset", "summary"],
    queryFn: () => apiFetch<VpsAssetSummary>("/api/vps-assets/summary"),
  });
}

export function useVpsAssetDetail(id: string) {
  return useQuery({
    queryKey: ["vps-asset", "detail", id],
    queryFn: () => apiFetch<VpsAsset>(`/api/vps-assets/${id}`),
    enabled: !!id,
  });
}

export function useUpdateVpsAsset() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateVpsAssetRequest }) =>
      apiFetch<VpsAsset>(`/api/vps-assets/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["vps-asset"] });
    },
  });
}

export function useDeleteVpsAsset() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/vps-assets/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["vps-asset"] });
    },
  });
}
