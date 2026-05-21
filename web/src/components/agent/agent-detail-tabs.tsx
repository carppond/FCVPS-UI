import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import { formatBytes, formatDate } from "@/lib/format";
import { useAuthStore } from "@/stores/auth-store";
import {
  useAgentMetricsQuery,
  type MetricRange,
} from "@/api/agent";
import type {
  AgentListItem,
  AgentMetric,
} from "@/types/api";
import { MetricChart } from "./metric-chart";

interface AgentDetailTabsProps {
  agent: AgentListItem;
  /** Realtime metric samples accumulated from SSE `agent_metrics` events. */
  realtimeMetrics: AgentMetric[];
}

const RANGES: MetricRange[] = ["1h", "6h", "24h"];

/**
 * 4-tab agent detail body:
 *
 *   1. realtime  — SSE-driven live charts (CPU / Memory / Net).
 *   2. history   — fetched metrics, toggle 1h / 6h / 24h.
 *   3. metadata  — OS / arch / version / kind / boot_time / capabilities.
 *   4. commands  — admin-only command history (placeholder until T-26 wires
 *      the audit log filter).
 *
 * Admin gating mirrors the sidebar pattern in components/layout/sidebar.tsx.
 */
export function AgentDetailTabs({
  agent,
  realtimeMetrics,
}: AgentDetailTabsProps) {
  const { t } = useTranslation(["agent", "common"]);
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");
  const [range, setRange] = React.useState<MetricRange>("1h");
  const history = useAgentMetricsQuery(agent.id, range);

  return (
    <Tabs defaultValue="realtime" className="flex flex-col gap-4">
      <TabsList>
        <TabsTrigger value="realtime">{t("agent:tabs.realtime")}</TabsTrigger>
        <TabsTrigger value="history">{t("agent:tabs.history")}</TabsTrigger>
        <TabsTrigger value="metadata">{t("agent:tabs.metadata")}</TabsTrigger>
        {isAdmin && (
          <TabsTrigger value="commands">{t("agent:tabs.commands")}</TabsTrigger>
        )}
      </TabsList>

      <TabsContent value="realtime" className="flex flex-col gap-3">
        <MetricsGrid metrics={realtimeMetrics} />
      </TabsContent>

      <TabsContent value="history" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border-strong)]">
            {RANGES.map((r) => (
              <ToggleButton
                key={r}
                active={range === r}
                onClick={() => setRange(r)}
                label={t(`agent:range.${r}`)}
              />
            ))}
          </div>
        </div>
        <MetricsGrid
          metrics={history.data ?? []}
          loading={history.isLoading}
        />
      </TabsContent>

      <TabsContent value="metadata">
        <MetadataGrid agent={agent} />
      </TabsContent>

      {isAdmin && (
        <TabsContent value="commands">
          <CommandsPlaceholder />
        </TabsContent>
      )}
    </Tabs>
  );
}

function MetricsGrid({
  metrics,
  loading,
}: {
  metrics: AgentMetric[];
  loading?: boolean;
}) {
  return (
    <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
      <MetricChart metrics={metrics} series="cpu" loading={loading} />
      <MetricChart metrics={metrics} series="memory" loading={loading} />
      <MetricChart metrics={metrics} series="net" loading={loading} />
    </div>
  );
}

function ToggleButton({
  active,
  onClick,
  label,
}: {
  active: boolean;
  onClick: () => void;
  label: string;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      onClick={onClick}
      className={cn(
        "h-7 rounded-none px-3 text-[var(--font-size-xs)]",
        active
          ? "bg-[var(--color-primary)] text-[var(--color-primary-foreground)] hover:bg-[var(--color-primary)]"
          : "bg-transparent text-[var(--color-text-secondary)]",
      )}
    >
      {label}
    </Button>
  );
}

function MetadataGrid({ agent }: { agent: AgentListItem }) {
  const { t } = useTranslation(["agent", "common"]);
  const m = agent.latest_metrics;
  const fields: Array<{ label: string; value: React.ReactNode }> = [
    {
      label: t("agent:detail.field_kind"),
      value: t(`agent:kind.${agent.kind}`),
    },
    {
      label: t("agent:detail.field_os"),
      value: agent.os || "—",
    },
    {
      label: t("agent:detail.field_arch"),
      value: agent.arch || "—",
    },
    {
      label: t("agent:detail.field_version"),
      value: agent.version || "—",
    },
    {
      label: t("agent:detail.field_public_ip"),
      value: agent.public_ip || "—",
    },
    {
      label: t("agent:detail.field_last_seen"),
      value: agent.last_seen_at ? formatDate(agent.last_seen_at) : "—",
    },
    {
      label: t("agent:detail.field_created"),
      value: formatDate(agent.created_at),
    },
    {
      label: t("agent:detail.field_uptime"),
      value: m ? formatUptime(m.uptime) : "—",
    },
    {
      label: t("agent:metric.disk"),
      value: m
        ? `${formatBytes(m.disk_used)} / ${formatBytes(m.disk_total)}`
        : "—",
    },
  ];
  return (
    <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {fields.map((f) => (
        <div
          key={f.label}
          className="flex flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3"
        >
          <dt className="text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
            {f.label}
          </dt>
          <dd className="text-[var(--font-size-sm)] text-[var(--color-text-primary)] tabular-nums">
            {f.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function CommandsPlaceholder() {
  const { t } = useTranslation("agent");
  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6 text-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
      {t("detail.no_commands")}
    </div>
  );
}

function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${days}d ${hours}h ${minutes}m`;
}
