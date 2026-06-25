/**
 * Batch subscription operations (own subscriptions only).
 *
 * Mirrors internal/handler/subscription_batch_handler.go. Each mutation posts a
 * caller-supplied id list and resolves to a SubscriptionBatchResult carrying
 * per-id ok/error plus succeeded/failed counts. Every mutation invalidates the
 * subscription list (and tags/nodes where relevant) so the grid refetches.
 */
import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  SubscriptionBatchResult,
  SubscriptionBatchTagsRequest,
  SubscriptionBatchUpdateRequest,
} from "@/types/api";

function invalidateList() {
  queryClient.invalidateQueries({ queryKey: queryKeys.subscription.list() });
}

/** POST /api/subscriptions/batch-sync */
export function useBatchSyncSubscriptionsMutation() {
  return useMutation({
    mutationFn: (ids: string[]) =>
      apiFetch<SubscriptionBatchResult>("/api/subscriptions/batch-sync", {
        method: "POST",
        body: JSON.stringify({ ids }),
      }),
    onSuccess: () => {
      invalidateList();
      queryClient.invalidateQueries({ queryKey: queryKeys.node.all() });
    },
  });
}

/** POST /api/subscriptions/batch-delete */
export function useBatchDeleteSubscriptionsMutation() {
  return useMutation({
    mutationFn: (ids: string[]) =>
      apiFetch<SubscriptionBatchResult>("/api/subscriptions/batch-delete", {
        method: "POST",
        body: JSON.stringify({ ids }),
      }),
    onSuccess: () => {
      invalidateList();
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.tags() });
      queryClient.invalidateQueries({ queryKey: queryKeys.node.all() });
    },
  });
}

/** POST /api/subscriptions/batch-tags */
export function useBatchTagsSubscriptionsMutation() {
  return useMutation({
    mutationFn: (payload: SubscriptionBatchTagsRequest) =>
      apiFetch<SubscriptionBatchResult>("/api/subscriptions/batch-tags", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      invalidateList();
      queryClient.invalidateQueries({ queryKey: queryKeys.subscription.tags() });
    },
  });
}

/** POST /api/subscriptions/batch-update */
export function useBatchUpdateSubscriptionsMutation() {
  return useMutation({
    mutationFn: (payload: SubscriptionBatchUpdateRequest) =>
      apiFetch<SubscriptionBatchResult>("/api/subscriptions/batch-update", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: invalidateList,
  });
}
