import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";
import { formatBitrate, formatBytes, formatPercent } from "@/lib/format";
import type { AgentMetric } from "@/types/api";

export type MetricSeries = "cpu" | "memory" | "net";

interface MetricChartProps {
  /** Metrics array (oldest → newest). */
  metrics: AgentMetric[];
  /** Which family of series to render. */
  series: MetricSeries;
  /** Display a Skeleton placeholder instead of the chart. */
  loading?: boolean;
  className?: string;
}

/**
 * Recharts area chart for a single agent's metric family. Colour palette comes
 * straight from the `--color-chart-*` tokens — never use the accent palette
 * for chart series (per _dev-cheatsheet.md §数据可视化).
 *
 *   - cpu    : single series, percent (0-100)
 *   - memory : two stacked series, bytes (used / total)
 *   - net    : two series, bytes/sec (inbound / outbound)
 *
 * The X axis is rendered with the wall-clock recorded_at so realtime and
 * historical charts share the same look.
 */
export function MetricChart({
  metrics,
  series,
  loading,
  className,
}: MetricChartProps) {
  const { t } = useTranslation("agent");
  const points = React.useMemo(() => buildPoints(metrics, series), [
    metrics,
    series,
  ]);

  if (loading) {
    return <Skeleton className={cn("h-64 w-full", className)} />;
  }
  if (points.length === 0) {
    return (
      <div
        className={cn(
          "flex h-64 items-center justify-center rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)]",
          className,
        )}
      >
        <EmptyState title={t("detail.no_metrics")} />
      </div>
    );
  }

  return (
    <div
      className={cn(
        "h-64 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3",
        className,
      )}
    >
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={points}>
          <defs>
            <linearGradient id={`g-${series}-1`} x1="0" y1="0" x2="0" y2="1">
              <stop
                offset="0%"
                stopColor="var(--color-chart-1)"
                stopOpacity={0.35}
              />
              <stop
                offset="100%"
                stopColor="var(--color-chart-1)"
                stopOpacity={0}
              />
            </linearGradient>
            <linearGradient id={`g-${series}-2`} x1="0" y1="0" x2="0" y2="1">
              <stop
                offset="0%"
                stopColor="var(--color-chart-2)"
                stopOpacity={0.35}
              />
              <stop
                offset="100%"
                stopColor="var(--color-chart-2)"
                stopOpacity={0}
              />
            </linearGradient>
          </defs>
          <CartesianGrid stroke="var(--color-border)" strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            stroke="var(--color-text-tertiary)"
            fontSize={11}
            tickLine={false}
            minTickGap={32}
          />
          <YAxis
            stroke="var(--color-text-tertiary)"
            fontSize={11}
            tickLine={false}
            tickFormatter={tickFormatterFor(series)}
            width={56}
          />
          <Tooltip
            contentStyle={{
              background: "var(--color-bg-elevated)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-md)",
              color: "var(--color-text-primary)",
              fontSize: "var(--font-size-sm)",
            }}
            formatter={(value, name) => [
              tooltipFormatterFor(series)(Number(value)),
              String(name),
            ]}
          />
          <Legend
            wrapperStyle={{
              color: "var(--color-text-secondary)",
              fontSize: "var(--font-size-xs)",
            }}
          />
          {series === "cpu" && (
            <Area
              type="monotone"
              dataKey="cpu"
              name={t("metric.cpu") as string}
              stroke="var(--color-chart-1)"
              fill={`url(#g-${series}-1)`}
              strokeWidth={2}
              isAnimationActive={false}
            />
          )}
          {series === "memory" && (
            <Area
              type="monotone"
              dataKey="memUsed"
              name={t("metric.memory") as string}
              stroke="var(--color-chart-2)"
              fill={`url(#g-${series}-2)`}
              strokeWidth={2}
              isAnimationActive={false}
            />
          )}
          {series === "net" && (
            <>
              <Area
                type="monotone"
                dataKey="netIn"
                name={t("metric.net_in") as string}
                stroke="var(--color-chart-1)"
                fill={`url(#g-${series}-1)`}
                strokeWidth={2}
                isAnimationActive={false}
              />
              <Area
                type="monotone"
                dataKey="netOut"
                name={t("metric.net_out") as string}
                stroke="var(--color-chart-2)"
                fill={`url(#g-${series}-2)`}
                strokeWidth={2}
                isAnimationActive={false}
              />
            </>
          )}
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}

interface ChartPoint {
  time: string;
  cpu?: number;
  memUsed?: number;
  netIn?: number;
  netOut?: number;
}

/**
 * Y-axis tick formatter per series family. CPU uses integer percent for a
 * clean axis, memory uses byte units, and net uses byte-rate units (B/s,
 * KB/s, …) to match what the per-card numbers show.
 */
function tickFormatterFor(series: MetricSeries): (v: unknown) => string {
  if (series === "cpu") return (v) => `${Math.round(Number(v))}%`;
  if (series === "net") return (v) => formatBitrate(Number(v));
  return (v) => formatBytes(Number(v));
}

/**
 * Tooltip formatter — same family logic as the axis, but with 1 fractional
 * digit on percent so hover detail matches the surrounding cards.
 */
function tooltipFormatterFor(series: MetricSeries): (v: number) => string {
  if (series === "cpu") return (v) => formatPercent(v);
  if (series === "net") return (v) => formatBitrate(v);
  return (v) => formatBytes(v);
}

function buildPoints(metrics: AgentMetric[], series: MetricSeries): ChartPoint[] {
  return metrics.map((m) => {
    const time = new Date(m.recorded_at).toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
    const base: ChartPoint = { time };
    if (series === "cpu") base.cpu = m.cpu_percent;
    if (series === "memory") base.memUsed = m.mem_used;
    if (series === "net") {
      base.netIn = m.net_in_speed;
      base.netOut = m.net_out_speed;
    }
    return base;
  });
}
