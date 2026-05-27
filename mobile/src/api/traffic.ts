import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { TrafficSummary } from "../types/api";

export function useTrafficSummary() {
  return useQuery({
    queryKey: ["traffic", "summary"],
    queryFn: () =>
      apiFetch<TrafficSummary>("/api/traffic/summary"),
  });
}
