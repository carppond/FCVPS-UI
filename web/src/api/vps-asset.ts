import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  APIResponse,
  CreateVpsAssetRequest,
  PagedResponse,
  UpdateVpsAssetRequest,
  VpsAsset,
  VpsAssetSummary,
} from "@/types/api";

export interface ListVpsAssetsParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
  provider?: string;
  status?: string;
  location?: string;
}

function buildQuery(params: ListVpsAssetsParams): string {
  const s = new URLSearchParams();
  if (params.page !== undefined) s.set("page", String(params.page));
  if (params.pageSize !== undefined) s.set("page_size", String(params.pageSize));
  if (params.keyword) s.set("keyword", params.keyword);
  if (params.provider) s.set("provider", params.provider);
  if (params.status) s.set("status", params.status);
  if (params.location) s.set("location", params.location);
  const qs = s.toString();
  return qs ? `?${qs}` : "";
}

export function useVpsAssetsQuery(params: ListVpsAssetsParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.vpsAsset.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<VpsAsset>>(
        `/api/vps-assets${buildQuery(params)}`,
      ),
  });
}

export function useVpsAssetQuery(id: string) {
  return useQuery({
    queryKey: queryKeys.vpsAsset.detail(id),
    queryFn: () => apiFetch<VpsAsset>(`/api/vps-assets/${id}`),
    enabled: !!id,
  });
}

export function useVpsAssetSummaryQuery() {
  return useQuery({
    queryKey: queryKeys.vpsAsset.summary(),
    queryFn: () => apiFetch<VpsAssetSummary>("/api/vps-assets/summary"),
  });
}

export function useCreateVpsAssetMutation() {
  return useMutation({
    mutationFn: (data: CreateVpsAssetRequest) =>
      apiFetch<VpsAsset>("/api/vps-assets", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vpsAsset.all() });
    },
  });
}

export function useUpdateVpsAssetMutation() {
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateVpsAssetRequest }) =>
      apiFetch<VpsAsset>(`/api/vps-assets/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vpsAsset.all() });
    },
  });
}

export function useDeleteVpsAssetMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/vps-assets/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.vpsAsset.all() });
    },
  });
}
