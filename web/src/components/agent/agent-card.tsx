import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  ArrowDown,
  ArrowUp,
  Cpu,
  HardDrive,
  MemoryStick,
  Wifi,
} from "lucide-react";
import { cn } from "@/lib/cn";
import {
  formatBitrate,
  formatBytes,
  formatPercent,
  formatRelativeTime,
} from "@/lib/format";
import type { AgentListItem, AgentMetric } from "@/types/api";
import { AgentStatusDot } from "./agent-status-dot";
import { AgentKindBadge } from "./agent-kind-badge";

interface AgentCardProps {
  agent: AgentListItem;
  /** Linkable variant — used by Dashboard (T-29) for quick navigation. */
  href?: string;
  className?: string;
}

/**
 * Compact card showing the current state of one agent. Designed to be
 * embedded both in the Dashboard's "quick glance" row and in the agent
 * list page's grid view. Looks identical regardless of role — admin
 * affordances live on the detail page.
 *
 * Renders 4 metrics (CPU / Memory / Disk / Network) with mini progress
 * bars where a denominator exists. Defensive against bad samples: clamps
 * percentages, shows "—" for missing data, and treats the very first
 * heartbeat (net_speed === 0 immediately after onboarding) as warming up.
 */
export function AgentCard({ agent, href, className }: AgentCardProps) {
  const { t } = useTranslation("agent");
  const m = agent.latest_metrics;
  const offlineLabel = agent.last_seen_at
    ? t("card.offline_hint", {
        when: formatRelativeTime(agent.last_seen_at),
      })
    : t("card.no_metrics_hint");

  const inner = (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4",
        "transition-colors duration-[var(--duration-fast)] hover:border-[var(--color-border-strong)]",
        className,
      )}
    >
      <header className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex flex-col gap-1">
          <p className="truncate text-[var(--font-size-base)] font-semibold text-[var(--color-text-primary)]">
            {agent.name}
          </p>
          <div className="flex items-center gap-2">
            <AgentKindBadge kind={agent.kind} />
            <AgentStatusDot status={agent.status} withLabel />
          </div>
        </div>
      </header>

      {agent.status === "online" && m ? (
        <CardMetrics metrics={m} t={t} />
      ) : (
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {offlineLabel}
        </p>
      )}
    </div>
  );

  if (!href) return inner;
  return (
    <Link to={href} className="block">
      {inner}
    </Link>
  );
}

interface CardMetricsProps {
  metrics: AgentMetric;
  t: (key: string, opts?: Record<string, unknown>) => string;
}

function CardMetrics({ metrics: m, t }: CardMetricsProps) {
  // Clamp values defensively — backend or gopsutil can occasionally emit
  // mem_used > mem_total or transient negatives during boot.
  const cpuPct = clamp(m.cpu_percent, 0, 100);
  const memPct =
    m.mem_total > 0 ? clamp((m.mem_used / m.mem_total) * 100, 0, 100) : null;
  const diskPct =
    m.disk_total > 0 ? clamp((m.disk_used / m.disk_total) * 100, 0, 100) : null;

  // First-heartbeat detection: the collector needs two samples to compute
  // a rate, so net_speed is structurally zero on the very first report.
  // We use mem_total>0 as a proxy for "agent actually reported data".
  const isWarmingUp =
    m.mem_total > 0 && m.net_in_speed === 0 && m.net_out_speed === 0;

  return (
    <div className="grid grid-cols-2 gap-x-3 gap-y-3 tabular-nums">
      <Metric
        icon={<Cpu className="h-3.5 w-3.5" />}
        label={t("card.cpu_label")}
        primary={formatPercent(cpuPct)}
        progress={cpuPct}
      />
      <Metric
        icon={<MemoryStick className="h-3.5 w-3.5" />}
        label={t("card.memory_label")}
        primary={memPct === null ? "—" : formatPercent(memPct)}
        secondary={
          memPct === null
            ? undefined
            : formatBytes(Math.min(m.mem_used, m.mem_total))
        }
        progress={memPct ?? undefined}
      />
      <Metric
        icon={<HardDrive className="h-3.5 w-3.5" />}
        label={t("card.disk_label")}
        primary={diskPct === null ? "—" : formatPercent(diskPct)}
        secondary={
          diskPct === null
            ? t("card.no_disk")
            : `${formatBytes(Math.min(m.disk_used, m.disk_total))} / ${formatBytes(m.disk_total)}`
        }
        progress={diskPct ?? undefined}
      />
      <Metric
        icon={<Wifi className="h-3.5 w-3.5" />}
        label={t("card.network_label")}
        primary={
          isWarmingUp ? (
            <span className="text-[var(--color-text-tertiary)] text-[var(--font-size-xs)] font-normal">
              {t("card.warming_up")}
            </span>
          ) : (
            <span className="flex flex-col text-[var(--font-size-xs)] font-medium leading-tight">
              <span className="flex items-center gap-0.5">
                <ArrowUp className="h-3 w-3 text-[var(--color-chart-2)]" />
                {formatBitrate(Math.max(0, m.net_out_speed))}
              </span>
              <span className="flex items-center gap-0.5">
                <ArrowDown className="h-3 w-3 text-[var(--color-chart-1)]" />
                {formatBitrate(Math.max(0, m.net_in_speed))}
              </span>
            </span>
          )
        }
      />
    </div>
  );
}

interface MetricProps {
  icon: React.ReactNode;
  label: string;
  /** Primary metric value (string for normal numbers, ReactNode for the
   *  network split layout). */
  primary: React.ReactNode;
  secondary?: string;
  /** Percentage 0-100; renders a mini progress bar when provided. */
  progress?: number;
}

function Metric({ icon, label, primary, secondary, progress }: MetricProps) {
  return (
    <div className="flex flex-col gap-1 min-w-0">
      <span className="flex items-center gap-1 text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
        {icon}
        {label}
      </span>
      <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
        {primary}
      </span>
      {progress !== undefined && (
        <ProgressBar value={progress} tone={progressTone(progress)} />
      )}
      {secondary && (
        <span className="truncate text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {secondary}
        </span>
      )}
    </div>
  );
}

function ProgressBar({
  value,
  tone,
}: {
  value: number;
  tone: "ok" | "warn" | "danger";
}) {
  const toneClass =
    tone === "danger"
      ? "bg-[var(--color-error)]"
      : tone === "warn"
        ? "bg-[var(--color-warning)]"
        : "bg-[var(--color-primary)]";
  return (
    <div className="h-1 w-full overflow-hidden rounded-full bg-[var(--color-bg-elevated)]">
      <div
        className={cn("h-full transition-all duration-[var(--duration-fast)]", toneClass)}
        style={{ width: `${clamp(value, 0, 100)}%` }}
      />
    </div>
  );
}

function progressTone(pct: number): "ok" | "warn" | "danger" {
  if (pct >= 90) return "danger";
  if (pct >= 75) return "warn";
  return "ok";
}

function clamp(n: number, min: number, max: number): number {
  if (!Number.isFinite(n)) {
    // Don't spam the console in tests / SSR — but warn once for visibility.
    if (typeof console !== "undefined") {
      console.warn("[agent-card] non-finite metric value", n);
    }
    return min;
  }
  return Math.max(min, Math.min(max, n));
}
