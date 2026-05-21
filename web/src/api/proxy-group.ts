/**
 * M-PROXY-GROUP API client — mirrors GET/POST/PUT/DELETE /api/proxy-groups/*
 * in internal/handler/proxy_group_handler.go. The list endpoint returns groups
 * already sorted by `sort_order`; the reorder endpoint takes a flat list of
 * ids that the server uses to re-assign sort values atomically.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  CreateProxyGroupRequest,
  ProxyGroupCategory,
  ProxyGroupPreset,
  ProxyGroupReorderRequest,
  UpdateProxyGroupRequest,
} from "@/types/api";

/** GET /api/proxy-groups — flat list sorted by sort_order ascending. */
export function useProxyGroups() {
  return useQuery({
    queryKey: queryKeys.proxyGroup.list(),
    queryFn: () => apiFetch<ProxyGroupCategory[]>("/api/proxy-groups"),
  });
}

/** GET /api/proxy-groups/{id}. */
export function useProxyGroup(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.proxyGroup.detail(id ?? ""),
    queryFn: () => apiFetch<ProxyGroupCategory>(`/api/proxy-groups/${id}`),
    enabled: Boolean(id),
  });
}

/** GET /api/proxy-groups/presets — built-in presets (18 entries). */
export function useProxyGroupPresets() {
  return useQuery({
    queryKey: queryKeys.proxyGroup.presets(),
    queryFn: () => apiFetch<ProxyGroupPreset[]>("/api/proxy-groups/presets"),
    staleTime: Infinity,
  });
}

/** POST /api/proxy-groups — create a new group. */
export function useCreateProxyGroup() {
  return useMutation({
    mutationFn: (payload: CreateProxyGroupRequest) =>
      apiFetch<ProxyGroupCategory>("/api/proxy-groups", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.proxyGroup.all() });
    },
  });
}

/** PUT /api/proxy-groups/{id} — partial update. */
export function useUpdateProxyGroup() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: UpdateProxyGroupRequest;
    }) =>
      apiFetch<ProxyGroupCategory>(`/api/proxy-groups/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    onSuccess: (grp) => {
      queryClient.setQueryData(queryKeys.proxyGroup.detail(grp.id), grp);
      queryClient.invalidateQueries({ queryKey: queryKeys.proxyGroup.list() });
    },
  });
}

/** DELETE /api/proxy-groups/{id}. */
export function useDeleteProxyGroup() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/proxy-groups/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.proxyGroup.all() });
    },
  });
}

/** POST /api/proxy-groups/reorder — atomic sort update by id list. */
export function useReorderProxyGroups() {
  return useMutation({
    mutationFn: (payload: ProxyGroupReorderRequest) =>
      apiFetch<{ updated: number }>("/api/proxy-groups/reorder", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.proxyGroup.list() });
    },
  });
}
