import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, Subscription, SubscriptionDetail } from "../types/api";

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
