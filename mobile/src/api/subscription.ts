import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  PagedResponse,
  Subscription,
  SubscriptionDetail,
  SubscriptionSyncLog,
  UpdateSubscriptionRequest,
  SyncResult,
} from "../types/api";

export function useSubscriptionsQuery() {
  return useQuery({
    queryKey: ["subscription", "list"],
    queryFn: () =>
      apiFetch<PagedResponse<Subscription>>("/api/subscriptions?page=1&page_size=500"),
  });
}

export function useSubscriptionDetail(id: string) {
  return useQuery({
    queryKey: ["subscription", "detail", id],
    queryFn: () => apiFetch<SubscriptionDetail>(`/api/subscriptions/${id}`),
    enabled: !!id,
  });
}

export function useSubscriptionSyncLogs(id: string) {
  return useQuery({
    queryKey: ["subscription", "sync-logs", id],
    queryFn: () =>
      apiFetch<PagedResponse<SubscriptionSyncLog>>(`/api/subscriptions/${id}/sync-logs`),
    enabled: !!id,
  });
}

export function useSyncSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<SyncResult>(`/api/subscriptions/${id}/sync`, { method: "POST" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscription"] });
    },
  });
}

export function useDeleteSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/subscriptions/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscription"] });
    },
  });
}

export function useUpdateSubscription() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateSubscriptionRequest }) =>
      apiFetch<Subscription>(`/api/subscriptions/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["subscription"] });
    },
  });
}
