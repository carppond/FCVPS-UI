/**
 * Dashboard v4 — Four stat cards.
 *
 * Grid of 4 feature-entry cards matching the preview:
 *   1. Subscriptions — total + ok/error breakdown
 *   2. Online nodes — online/total + country flags
 *   3. Traffic this month — used/limit + percentage
 *   4. Pending alerts — count + detail
 *
 * Each card has an icon, title, big number, detail tags, and hover elevation.
 */
import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Skeleton } from "@/components/ui/skeleton";
import { useAgentsQuery } from "@/api/agent";
import { useNodesQuery } from "@/api/node";
import { useSubscriptionsQuery } from "@/api/subscription";
import { useTrafficSummaryQuery } from "@/api/traffic";
import { useEvents } from "@/api/notify";
import { formatBytes } from "@/lib/format";

// ── Card shell ──────────────────────────────────────────────────────────────

interface CardShellProps {
  to: string;
  icon: React.ReactNode;
  iconBg: string;
  moreText: string;
  title: string;
  isLoading: boolean;
  children: React.ReactNode;
}

function CardShell({
  to,
  icon,
  iconBg,
  moreText,
  title,
  isLoading,
  children,
}: CardShellProps) {
  return (
    <Link
      to={to as unknown as "/"}
      className="flex cursor-pointer flex-col gap-2 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 backdrop-blur-2xl transition-all duration-150 hover:-translate-y-0.5 hover:border-[var(--color-border-strong)] hover:shadow-lg"
    >
      <div className="flex items-center justify-between">
        <span
          className="flex h-8 w-8 items-center justify-center rounded-lg text-base"
          style={{ background: iconBg }}
        >
          {icon}
        </span>
        <span className="text-[11px] text-[var(--color-text-tertiary)]">
          {moreText} →
        </span>
      </div>
      <div className="text-xs font-medium text-[var(--color-text-secondary)]">
        {title}
      </div>
      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-8 w-20" />
          <Skeleton className="h-4 w-32" />
        </div>
      ) : (
        children
      )}
    </Link>
  );
}

// ── Subscriptions card ──────────────────────────────────────────────────────

function SubscriptionsCard() {
  const { t } = useTranslation("dashboard");
  const query = useSubscriptionsQuery({ page: 1, pageSize: 200 });
  const items = query.data?.items ?? [];
  const total = query.data?.total ?? items.length;
  const okCount = items.filter((s) => s.last_sync_status === "ok").length;
  const errCount = items.filter((s) => s.last_sync_status === "error").length;

  return (
    <CardShell
      to="/subscriptions"
      icon="📦"
      iconBg="var(--color-primary-soft)"
      moreText={t("stats.subscriptions.manage")}
      title={t("stats.subscriptions.title")}
      isLoading={query.isLoading}
    >
      <div className="text-[28px] font-bold tabular-nums leading-none tracking-tight text-[var(--color-text-primary)]">
        {total}
      </div>
      <div className="flex gap-2 text-[11px] text-[var(--color-text-tertiary)]">
        {total === 0 ? (
          <span>{t("stats.subscriptions.empty")}</span>
        ) : (
          <>
            {okCount > 0 && (
              <span className="rounded bg-[var(--color-success-bg)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--color-success)]">
                {t("stats.subscriptions.ok_count", { count: okCount })}
              </span>
            )}
            {errCount > 0 && (
              <span className="rounded bg-[var(--color-warning-bg)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--color-warning)]">
                {t("stats.subscriptions.error_count", { count: errCount })}
              </span>
            )}
          </>
        )}
      </div>
    </CardShell>
  );
}

// ── Nodes card ──────────────────────────────────────────────────────────────

function NodesCard() {
  const { t } = useTranslation("dashboard");
  const agentsQ = useAgentsQuery({ page: 1, pageSize: 200 });
  const nodesQ = useNodesQuery({ page: 1, pageSize: 200 });
  const nodeItems = useMemo(() => nodesQ.data?.items ?? [], [nodesQ.data]);
  const total = nodesQ.data?.total ?? nodeItems.length;

  // For "online" count, we use the agents (probes) that test reachability,
  // but nodes themselves don't have an online flag in the current API.
  // So we show total with agent online count as a proxy.
  const agentItems = agentsQ.data?.items ?? [];
  const agentOnline = agentItems.filter(
    (a) => a.online || a.status === "online",
  ).length;

  // Group by country_code from node metadata if available
  const countryDist = useMemo(() => {
    const counts = new Map<string, number>();
    for (const node of nodeItems) {
      const cc =
        (node as unknown as Record<string, string>).country_code ?? "";
      if (cc) counts.set(cc, (counts.get(cc) ?? 0) + 1);
    }
    return Array.from(counts.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, 4);
  }, [nodeItems]);

  return (
    <CardShell
      to="/nodes"
      icon="🌐"
      iconBg="var(--color-success-bg)"
      moreText={t("stats.nodes.all")}
      title={t("stats.nodes.title")}
      isLoading={nodesQ.isLoading}
    >
      <div className="text-[28px] font-bold tabular-nums leading-none tracking-tight text-[var(--color-text-primary)]">
        {agentOnline > 0 ? (
          <>
            {agentOnline}{" "}
            <span className="text-sm font-medium text-[var(--color-text-tertiary)]">
              / {total}
            </span>
          </>
        ) : (
          total
        )}
      </div>
      <div className="flex gap-2 text-[11px] text-[var(--color-text-tertiary)]">
        {total === 0 ? (
          <span>{t("stats.nodes.empty")}</span>
        ) : countryDist.length > 0 ? (
          countryDist.map(([cc, count]) => (
            <span key={cc}>
              {cc} {count}
            </span>
          ))
        ) : (
          <span>
            {total} {t("stats.nodes.title").toLowerCase()}
          </span>
        )}
      </div>
    </CardShell>
  );
}

// ── Traffic card ────────────────────────────────────────────────────────────

function TrafficCard() {
  const { t } = useTranslation("dashboard");
  const query = useTrafficSummaryQuery();
  const data = query.data;

  const usedStr = data ? formatBytes(data.total_used) : "—";
  const limitStr = data?.total_limit ? formatBytes(data.total_limit) : null;
  const percent = data?.usage_percent
    ? Math.round(data.usage_percent)
    : null;

  return (
    <CardShell
      to="/traffic"
      icon="📊"
      iconBg="var(--color-info-bg)"
      moreText={t("stats.traffic.detail")}
      title={t("stats.traffic.title")}
      isLoading={query.isLoading}
    >
      <div className="text-[28px] font-bold tabular-nums leading-none tracking-tight text-[var(--color-text-primary)]">
        {usedStr.split(" ")[0]}{" "}
        <span className="text-sm font-medium text-[var(--color-text-tertiary)]">
          {usedStr.split(" ")[1]}
        </span>
      </div>
      <div className="flex gap-2 text-[11px] text-[var(--color-text-tertiary)]">
        {limitStr ? (
          <>
            <span>
              / {limitStr} · {percent}%
            </span>
          </>
        ) : (
          <span>{t("stats.traffic.empty")}</span>
        )}
      </div>
    </CardShell>
  );
}

// ── Alerts card ─────────────────────────────────────────────────────────────

function AlertsCard() {
  const { t } = useTranslation("dashboard");
  const from = useMemo(() => Date.now() - 24 * 60 * 60 * 1000, []);
  const query = useEvents({ status: "failed", from, page: 1, pageSize: 5 });
  const total = query.data?.total ?? 0;
  const items = useMemo(() => query.data?.items ?? [], [query.data]);

  // Build a short detail summary from event types
  const detailParts = useMemo(() => {
    const kinds = new Map<string, number>();
    for (const ev of items) {
      const key = ev.event_type ?? "unknown";
      kinds.set(key, (kinds.get(key) ?? 0) + 1);
    }
    return Array.from(kinds.entries())
      .slice(0, 3)
      .map(([k, n]) => `${k} ${n}`);
  }, [items]);

  return (
    <CardShell
      to="/notifications"
      icon="🔔"
      iconBg="var(--color-error-bg)"
      moreText={t("stats.alerts.view")}
      title={t("stats.alerts.title")}
      isLoading={query.isLoading}
    >
      <div
        className="text-[28px] font-bold tabular-nums leading-none tracking-tight"
        style={{
          color:
            total > 0
              ? "var(--color-warning)"
              : "var(--color-text-primary)",
        }}
      >
        {total}
      </div>
      <div className="flex gap-2 text-[11px] text-[var(--color-text-tertiary)]">
        {total === 0 ? (
          <span>{t("stats.alerts.all_clear")}</span>
        ) : (
          detailParts.map((part, i) => <span key={i}>{part}</span>)
        )}
      </div>
    </CardShell>
  );
}

// ── Export ───────────────────────────────────────────────────────────────────

/** 4-column grid of v4 stat cards. */
export function StatsCards() {
  return (
    <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 xl:grid-cols-4">
      <SubscriptionsCard />
      <NodesCard />
      <TrafficCard />
      <AlertsCard />
    </div>
  );
}
