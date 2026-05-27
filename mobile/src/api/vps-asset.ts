import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, VpsAsset, VpsAssetSummary } from "../types/api";

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
