import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { TrafficSummary } from "../types/api";

// Shared so both the screen hook and the foreground-refresh path (root layout
// → widget push) fetch with the same key/fn instead of duplicating the route.
export const trafficSummaryQuery = {
  queryKey: ["traffic", "summary"] as const,
  queryFn: () => apiFetch<TrafficSummary>("/api/traffic/summary"),
};

export function useTrafficSummary() {
  return useQuery(trafficSummaryQuery);
}
