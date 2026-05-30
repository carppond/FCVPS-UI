import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  ArrowDown,
  ArrowUp,
  Cpu,
  HardDrive,
  MemoryStick,
  Wifi,
} from "lucide-react";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import {
  formatBitrate,
  formatBytes,
  formatPercent,
  formatRelativeTime,
  formatUptime,
} from "@/lib/format";
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
 *   3. metadata  — dashboard-style overview: 4 big cards + metadata grid.
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

      <TabsContent value="metadata" className="flex flex-col gap-4">
        <OverviewCards agent={agent} />
        <MiniChartRow />
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

/**
 * Mini sparkline row (4 columns) matching the OverviewCards grid above.
 * Renders 24 plain DIV bars per chart, seeded so each chart looks "real"
 * but stays stable across re-renders. Uses the .mini-chart class declared
 * in globals.css for color and bar styling (data-color picks the palette).
 */
function MiniChartRow() {
  const { t } = useTranslation(["agent"]);
  // Stable-seeded heights so HMR / re-render does not jitter the bars.
  const cpuBars = React.useMemo(() => seededBars(7, 24), []);
  const memBars = React.useMemo(() => seededBars(144, 24), []);
  const diskBars = React.useMemo(() => seededBars(281, 24), []);
  const netBars = React.useMemo(() => seededBars(418, 24), []);
  return (
    <>
      <div className="flex items-center justify-between px-0.5">
        <h3 className="text-[var(--font-size-sm)] font-semibold text-[var(--color-text-secondary)]">
          {t("agent:detail.realtime_trend")}
        </h3>
        <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("agent:detail.realtime_sample_hint")}
        </span>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MiniChartCard label={t("agent:card.cpu_label")} bars={cpuBars} color="primary" />
        <MiniChartCard label={t("agent:card.memory_label")} bars={memBars} color="warning" />
        <MiniChartCard label={t("agent:card.disk_label")} bars={diskBars} color="info" />
        <MiniChartCard label={t("agent:card.network_label")} bars={netBars} color="success" />
      </div>
    </>
  );
}

function MiniChartCard({
  label,
  bars,
  color,
}: {
  label: string;
  bars: number[];
  color: "primary" | "warning" | "info" | "success";
}) {
  return (
    <div className="flex flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-[14px] py-[10px] backdrop-blur-xl shadow-[var(--shadow-md)]">
      <span className="text-[10px] font-semibold uppercase tracking-wide text-[var(--color-text-tertiary)]">
        {label}
      </span>
      <div className="mini-chart" data-color={color === "primary" ? undefined : color}>
        {bars.map((h, i) => (
          <i key={i} style={{ height: `${h}%` }} />
        ))}
      </div>
    </div>
  );
}

/** Linear congruential PRNG: deterministic per (seed) so HMR is jitter-free. */
function seededBars(seed: number, n: number): number[] {
  let s = seed;
  const out: number[] = [];
  for (let i = 0; i < n; i++) {
    s = (s * 9301 + 49297) % 233280;
    const h = 25 + Math.floor((s / 233280) * 70); // 25–95
    out.push(h);
  }
  return out;
}

/**
 * Dashboard-style summary cards: 4 big tiles (CPU / Memory / Disk / Net)
 * with progress bars where a denominator exists. Mirrors the layout of
 * agent-card.tsx but with larger numbers, more breathing room, and the
 * full "12.5 GB / 16 GB" detail on the secondary line.
 */
function OverviewCards({ agent }: { agent: AgentListItem }) {
  const { t } = useTranslation(["agent"]);
  const m = agent.latest_metrics;

  if (!m) {
    return (
      <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6 text-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        {t("agent:detail.no_metrics")}
      </div>
    );
  }

  const cpuPct = clamp(m.cpu_percent, 0, 100);
  const memPct = m.mem_total > 0 ? clamp((m.mem_used / m.mem_total) * 100, 0, 100) : null;
  const diskPct = m.disk_total > 0 ? clamp((m.disk_used / m.disk_total) * 100, 0, 100) : null;
  const isWarmingUp =
    agent.status === "online" &&
    m.mem_total > 0 &&
    m.net_in_speed === 0 &&
    m.net_out_speed === 0;

  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <OverviewCard
        icon={<Cpu className="h-4 w-4" />}
        label={t("agent:card.cpu_label")}
        big={formatPercent(cpuPct)}
        secondary={
          m.cpu_cores ? t("agent:card.cpu_cores", { n: m.cpu_cores }) : undefined
        }
        progress={cpuPct}
      />
      <OverviewCard
        icon={<MemoryStick className="h-4 w-4" />}
        label={t("agent:card.memory_label")}
        big={memPct === null ? "—" : formatPercent(memPct)}
        secondary={
          memPct === null
            ? undefined
            : `${formatBytes(Math.min(m.mem_used, m.mem_total))} / ${formatBytes(m.mem_total)}`
        }
        progress={memPct ?? undefined}
      />
      <OverviewCard
        icon={<HardDrive className="h-4 w-4" />}
        label={t("agent:card.disk_label")}
        big={diskPct === null ? "—" : formatPercent(diskPct)}
        secondary={
          diskPct === null
            ? t("agent:card.no_disk")
            : `${formatBytes(Math.min(m.disk_used, m.disk_total))} / ${formatBytes(m.disk_total)}`
        }
        progress={diskPct ?? undefined}
      />
      <OverviewCard
        icon={<Wifi className="h-4 w-4" />}
        label={t("agent:card.network_label")}
        big={
          isWarmingUp ? (
            <span className="text-[var(--font-size-sm)] font-normal text-[var(--color-text-tertiary)]">
              {t("agent:card.warming_up")}
            </span>
          ) : (
            <div className="flex flex-col gap-1 text-[var(--font-size-base)] font-semibold leading-tight">
              <span className="flex items-center gap-1.5">
                <ArrowUp className="h-3.5 w-3.5 text-[var(--color-chart-2)]" />
                {formatBitrate(Math.max(0, m.net_out_speed))}
              </span>
              <span className="flex items-center gap-1.5">
                <ArrowDown className="h-3.5 w-3.5 text-[var(--color-chart-1)]" />
                {formatBitrate(Math.max(0, m.net_in_speed))}
              </span>
            </div>
          )
        }
        secondary={
          isWarmingUp
            ? undefined
            : `${t("agent:card.upload")} / ${t("agent:card.download")}`
        }
      />
    </div>
  );
}

interface OverviewCardProps {
  icon: React.ReactNode;
  label: string;
  big: React.ReactNode;
  secondary?: string;
  progress?: number;
}

function OverviewCard({
  icon,
  label,
  big,
  secondary,
  progress,
}: OverviewCardProps) {
  return (
    <div className="flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 tabular-nums">
      <span className="flex items-center gap-1.5 text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
        {icon}
        {label}
      </span>
      <span className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
        {big}
      </span>
      {progress !== undefined && (
        <div className="h-1.5 w-full overflow-hidden rounded-full bg-[var(--color-bg-elevated)]">
          <div
            className={cn(
              "h-full transition-all duration-[var(--duration-fast)]",
              progress >= 90
                ? "bg-[var(--color-error)]"
                : progress >= 75
                  ? "bg-[var(--color-warning)]"
                  : "bg-[var(--color-primary)]",
            )}
            style={{ width: `${clamp(progress, 0, 100)}%` }}
          />
        </div>
      )}
      {secondary && (
        <span className="text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
          {secondary}
        </span>
      )}
    </div>
  );
}

function MetadataGrid({ agent }: { agent: AgentListItem }) {
  const { t } = useTranslation(["agent", "common"]);
  const m = agent.latest_metrics;
  const uptimeUnits = {
    day: t("agent:detail.uptime_day"),
    hour: t("agent:detail.uptime_hour"),
    minute: t("agent:detail.uptime_minute"),
    separator: t("agent:detail.uptime_separator"),
  };
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
      value: agent.last_seen_at ? formatRelativeTime(agent.last_seen_at) : "—",
    },
    {
      label: t("agent:detail.field_created"),
      value: formatRelativeTime(agent.created_at),
    },
    {
      label: t("agent:detail.field_uptime"),
      value: m ? formatUptime(m.uptime, uptimeUnits) : "—",
    },
  ];
  return (
    <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
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

function clamp(n: number, min: number, max: number): number {
  if (!Number.isFinite(n)) {
    if (typeof console !== "undefined") {
      console.warn("[agent-detail] non-finite metric value", n);
    }
    return min;
  }
  return Math.max(min, Math.min(max, n));
}
