/**
 * M-TRAFFIC API client (T-18).
 *
 * Wraps every /api/traffic/* endpoint with a TanStack hook so the route +
 * components can stay declarative. The summary + history hooks refresh in the
 * background while the admin tweaks the limit so the chart never goes stale.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  AgentTrafficSummary,
  TrafficChartPoint,
  TrafficSummary,
} from "@/types/api";

// ─── Shared types ────────────────────────────────────────────────────────────

export type TrafficHistoryRange = "7d" | "30d" | "90d";
export type TrafficHistoryView = "day" | "month";

export interface HistoryParams {
  range?: TrafficHistoryRange;
  view?: TrafficHistoryView;
}

export interface ThresholdRequest {
  percents: number[];
  total_limit?: number;
}

export interface LimitRequest {
  total_limit: number;
}

// ─── Queries ─────────────────────────────────────────────────────────────────

/** GET /api/traffic/summary — current-month rolled-up summary. */
export function useTrafficSummaryQuery() {
  return useQuery({
    queryKey: queryKeys.traffic.summary(),
    queryFn: () => apiFetch<TrafficSummary>("/api/traffic/summary"),
    // Summary mutates roughly hourly (next heartbeat ⇒ next aggregator run).
    // 60s polling keeps the progress bar accurate without spamming the API.
    refetchInterval: 60_000,
  });
}

/** GET /api/traffic/history — chart points (day/month buckets). */
export function useTrafficHistoryQuery(params: HistoryParams = {}) {
  const search = new URLSearchParams();
  if (params.range) search.set("range", params.range);
  if (params.view) search.set("view", params.view);
  const qs = search.toString();
  return useQuery({
    queryKey: [...queryKeys.traffic.all(), "history", params] as const,
    queryFn: () =>
      apiFetch<TrafficChartPoint[]>(
        `/api/traffic/history${qs ? `?${qs}` : ""}`,
      ),
  });
}

/** GET /api/traffic/by-agent — per-agent breakdown for the current month. */
export function useTrafficByAgentQuery() {
  return useQuery({
    queryKey: [...queryKeys.traffic.all(), "by-agent"] as const,
    queryFn: () =>
      apiFetch<AgentTrafficSummary[]>("/api/traffic/by-agent"),
  });
}

// ─── Mutations (admin) ───────────────────────────────────────────────────────

/** PUT /api/traffic/threshold — admin: configure alert percentages. */
export function useSetTrafficThresholdMutation() {
  return useMutation({
    mutationFn: (req: ThresholdRequest) =>
      apiFetch<{ percents: number[]; total_limit?: number }>(
        "/api/traffic/threshold",
        { method: "PUT", body: JSON.stringify(req) },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.traffic.all() });
    },
  });
}

/** PUT /api/traffic/limit — admin: configure the monthly bytes limit. */
export function useSetTrafficLimitMutation() {
  return useMutation({
    mutationFn: (req: LimitRequest) =>
      apiFetch<LimitRequest>("/api/traffic/limit", {
        method: "PUT",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.traffic.all() });
    },
  });
}
