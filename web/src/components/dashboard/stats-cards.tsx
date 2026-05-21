/**
 * T-29: Dashboard stats cards.
 *
 * Four overview tiles rendered above the 2×2 grid:
 *   1. Online agents / total — mini sparkline of (online_count) over time
 *      derived from the live list (no historical endpoint).
 *   2. Total nodes — count + per-protocol mini distribution row.
 *   3. Traffic used this month — progress bar against the configured limit.
 *   4. Pending alerts — number of `failed` notification events in the last 24h.
 *
 * Every tile owns its own loading + error state so a single failed query does
 * not blank the whole row (matches the four-state contract from the dev
 * cheatsheet: normal / Skeleton / EmptyState / ErrorState).
 */
import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { AlertCircle, ChevronRight, Radio, Server } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { useAgentsQuery } from "@/api/agent";
import { useNodesQuery } from "@/api/node";
import { useTrafficSummaryQuery } from "@/api/traffic";
import { useEvents } from "@/api/notify";
import type { AgentListItem, NodeProtocol } from "@/types/api";

// ── tiny shared building blocks ─────────────────────────────────────────────

interface StatCardProps {
  title: string;
  to?: string;
  isLoading: boolean;
  isError: boolean;
  errorMessage: string;
  children: React.ReactNode;
}

function StatCard({ title, to, isLoading, isError, errorMessage, children }: StatCardProps) {
  const body = isError ? (
    <div className="flex items-center gap-2 text-[var(--color-error)]">
      <AlertCircle className="h-4 w-4" />
      <span className="text-[var(--font-size-xs)]">{errorMessage}</span>
    </div>
  ) : isLoading ? (
    <div className="flex flex-col gap-2">
      <Skeleton className="h-8 w-20" />
      <Skeleton className="h-3 w-32" />
    </div>
  ) : (
    children
  );

  const inner = (
    <Card
      className={cn(
        "flex h-full flex-col gap-[var(--spacing-3)] p-[var(--spacing-4)]",
        "transition-colors duration-[var(--duration-fast)]",
        to && "hover:bg-[var(--color-surface-hover)]",
      )}
    >
      <div className="flex items-center justify-between">
        <span className="text-[var(--font-size-xs)] uppercase tracking-wider text-[var(--color-text-tertiary)]">
          {title}
        </span>
        {to && <ChevronRight className="h-3.5 w-3.5 text-[var(--color-text-disabled)]" />}
      </div>
      {body}
    </Card>
  );

  if (!to) return inner;
  // TanStack Router treats the `to` argument as a tight union of typed paths.
  // For the dashboard we only ever link to top-level static paths, so we cast
  // to keep the JSX terse without losing the runtime path string.
  return (
    <Link to={to as unknown as "/"} className="block h-full">
      {inner}
    </Link>
  );
}

// ── 1. Online agents tile ───────────────────────────────────────────────────

function AgentsTile() {
  const { t } = useTranslation("dashboard");
  const query = useAgentsQuery({ page: 1, pageSize: 200 });
  const items = query.data?.items ?? [];

  const total = items.length;
  const online = items.filter((a) => a.online || a.status === "online").length;
  const offline = total - online;

  // Sparkline data: derived from the agent latest_metrics net_in_speed array
  // so the tile feels alive without a dedicated history endpoint. Each bar is
  // one agent's relative network velocity, capped so a single hot host does
  // not dominate the visual.
  const spark = useMemo<number[]>(() => {
    const speeds = items
      .map((a) => (a.latest_metrics?.net_in_speed ?? 0) + (a.latest_metrics?.net_out_speed ?? 0))
      .filter((v) => v >= 0);
    if (speeds.length === 0) return [];
    const max = Math.max(...speeds, 1);
    return speeds.slice(0, 16).map((v) => v / max);
  }, [items]);

  return (
    <StatCard
      title={t("stats.agents.title")}
      to="/agents"
      isLoading={query.isLoading}
      isError={query.isError}
      errorMessage={t("error.load_failed")}
    >
      <div className="flex items-end justify-between gap-[var(--spacing-3)]">
        <div className="flex flex-col gap-1">
          <div className="text-[var(--font-size-3xl)] font-semibold tabular-nums text-[var(--color-text-primary)]">
            {t("stats.agents.online_total", { online, total })}
          </div>
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {total === 0
              ? t("stats.agents.empty")
              : offline > 0
                ? t("stats.agents.offline_count", { count: offline })
                : t("stats.agents.all_online")}
          </span>
        </div>
        <Sparkline points={spark} />
      </div>
    </StatCard>
  );
}

function Sparkline({ points }: { points: number[] }) {
  const { t } = useTranslation("common");
  if (points.length === 0) {
    return <div className="h-8 w-20" aria-hidden />;
  }
  return (
    <div
      className="flex h-8 items-end gap-[2px]"
      role="img"
      aria-label={t("common:aria.sparkline")}
    >
      {points.map((v, i) => (
        <span
          key={i}
          className="w-1 rounded-[var(--radius-sm)] bg-[var(--color-primary)]"
          style={{ height: `${Math.max(8, v * 100)}%` }}
        />
      ))}
    </div>
  );
}

// ── 2. Total nodes tile ─────────────────────────────────────────────────────

const PROTOCOL_ORDER: NodeProtocol[] = [
  "vmess",
  "vless",
  "ss",
  "ssr",
  "trojan",
  "hysteria",
  "hysteria2",
  "tuic",
  "wireguard",
  "anytls",
  "socks5",
  "naive",
];

function NodesTile() {
  const { t } = useTranslation("dashboard");
  // Page size 1 keeps payload tiny — we only need the `total` plus the page
  // sample for the protocol mix. The list endpoint returns `total` in the
  // paged envelope.
  const summary = useNodesQuery({ page: 1, pageSize: 200 });
  const items = summary.data?.items ?? [];
  const total = summary.data?.total ?? items.length;

  const byProto = useMemo(() => {
    const counts = new Map<NodeProtocol, number>();
    for (const node of items) {
      counts.set(node.protocol, (counts.get(node.protocol) ?? 0) + 1);
    }
    return PROTOCOL_ORDER.filter((p) => counts.get(p))
      .slice(0, 4)
      .map((p) => ({ protocol: p, count: counts.get(p) ?? 0 }));
  }, [items]);

  return (
    <StatCard
      title={t("stats.nodes.title")}
      to="/nodes"
      isLoading={summary.isLoading}
      isError={summary.isError}
      errorMessage={t("error.load_failed")}
    >
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Server className="h-5 w-5 text-[var(--color-text-tertiary)]" />
          <div className="text-[var(--font-size-3xl)] font-semibold tabular-nums text-[var(--color-text-primary)]">
            {total}
          </div>
        </div>
        {byProto.length === 0 ? (
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("stats.nodes.empty")}
          </span>
        ) : (
          <div className="flex flex-wrap gap-x-3 gap-y-1">
            {byProto.map((row) => (
              <span
                key={row.protocol}
                className="font-mono text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]"
              >
                {row.protocol}·{row.count}
              </span>
            ))}
          </div>
        )}
      </div>
    </StatCard>
  );
}

// ── 3. Traffic tile ─────────────────────────────────────────────────────────

function TrafficTile() {
  const { t } = useTranslation("dashboard");
  const query = useTrafficSummaryQuery();
  const data = query.data;

  return (
    <StatCard
      title={t("stats.traffic.title")}
      to="/traffic"
      isLoading={query.isLoading}
      isError={query.isError}
      errorMessage={t("error.load_failed")}
    >
      <div className="flex flex-col gap-2">
        <div className="text-[var(--font-size-2xl)] font-semibold tabular-nums text-[var(--color-text-primary)]">
          {data
            ? data.total_limit
              ? t("stats.traffic.used_of_limit", {
                  used: humanBytes(data.total_used),
                  limit: humanBytes(data.total_limit),
                })
              : humanBytes(data.total_used)
            : "—"}
        </div>
        {data?.total_limit ? (
          <>
            <Progress value={data.usage_percent} label={t("stats.traffic.quota_label")} />
            <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("stats.traffic.usage_percent", {
                percent: Math.round(data.usage_percent),
              })}
            </span>
          </>
        ) : (
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("stats.traffic.no_limit")}
          </span>
        )}
      </div>
    </StatCard>
  );
}

// ── 4. Pending alerts tile ──────────────────────────────────────────────────

function AlertsTile() {
  const { t } = useTranslation("dashboard");
  const from = useMemo(() => Date.now() - 24 * 60 * 60 * 1000, []);
  const query = useEvents({ status: "failed", from, page: 1, pageSize: 1 });
  const total = query.data?.total ?? 0;

  return (
    <StatCard
      title={t("stats.alerts.title")}
      to="/notifications"
      isLoading={query.isLoading}
      isError={query.isError}
      errorMessage={t("error.load_failed")}
    >
      <div className="flex items-center gap-2">
        <Radio
          className={cn(
            "h-5 w-5",
            total > 0
              ? "text-[var(--color-error)]"
              : "text-[var(--color-success)]",
          )}
        />
        <div className="text-[var(--font-size-3xl)] font-semibold tabular-nums text-[var(--color-text-primary)]">
          {total}
        </div>
      </div>
      <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        {total > 0
          ? t("stats.alerts.count_24h", { count: total })
          : t("stats.alerts.all_clear")}
      </span>
    </StatCard>
  );
}

// ── helpers ─────────────────────────────────────────────────────────────────

function humanBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

// Re-export for the consumers (and tests) that want a single named entry point.
export type { AgentListItem };

/** 4-column grid of stat cards. Stacks on small screens. */
export function StatsCards() {
  return (
    <div className="grid grid-cols-1 gap-[var(--spacing-4)] sm:grid-cols-2 xl:grid-cols-4">
      <AgentsTile />
      <NodesTile />
      <TrafficTile />
      <AlertsTile />
    </div>
  );
}
