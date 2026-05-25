/**
 * Dashboard v4 — Traffic trend bar chart.
 *
 * Uses recharts BarChart with upload (blue) / download (green) bars.
 * Includes 7/30/90 day range toggle tabs at the top-right.
 * Legend shows color indicators for upload / download.
 */
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/lib/cn";
import { formatBytes } from "@/lib/format";
import {
  useTrafficHistoryQuery,
  useTrafficSummaryQuery,
  type TrafficHistoryRange,
} from "@/api/traffic";

type RangeTab = "7d" | "30d" | "90d";

export function DashboardTrafficChart() {
  const { t } = useTranslation("dashboard");
  const [range, setRange] = useState<RangeTab>("7d");
  const { data, isLoading } = useTrafficHistoryQuery({
    view: "day",
    range: range as TrafficHistoryRange,
  });
  const summary = useTrafficSummaryQuery();

  const points = useMemo(() => {
    return (data ?? []).map((p) => ({
      date: p.date.slice(5), // "MM-DD"
      upload: p.total_out,
      download: p.total_in,
    }));
  }, [data]);

  const summaryLine = useMemo(() => {
    if (!summary.data) return "";
    const used = formatBytes(summary.data.total_used);
    const limit = summary.data.total_limit
      ? formatBytes(summary.data.total_limit)
      : null;
    return limit ? `${used} / ${limit}` : used;
  }, [summary.data]);

  const ranges: RangeTab[] = ["7d", "30d", "90d"];

  return (
    <div className="flex flex-col overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] backdrop-blur-2xl">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[var(--color-border)] px-[18px] py-3.5">
        <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">
          {t("traffic_chart.title")}
        </h3>
        <div className="flex gap-1">
          {ranges.map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={cn(
                "rounded-lg border px-2 py-0.5 font-sans text-[10px] font-medium transition-all duration-150",
                r === range
                  ? "border-[var(--color-primary)] bg-[var(--color-primary)] text-white"
                  : "border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)]",
              )}
            >
              {t(`traffic_chart.${r}` as const)}
            </button>
          ))}
        </div>
      </div>

      {/* Chart body */}
      <div className="h-[200px] px-[18px] pt-4 pb-2">
        {isLoading ? (
          <div className="flex h-full items-end gap-2">
            {Array.from({ length: 7 }).map((_, i) => (
              <Skeleton
                key={i}
                className="flex-1"
                style={{ height: `${30 + Math.random() * 50}%` }}
              />
            ))}
          </div>
        ) : points.length === 0 ? (
          <EmptyState title={t("stats.traffic.empty")} />
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={points} barGap={2}>
              <CartesianGrid
                stroke="var(--color-border)"
                strokeDasharray="3 3"
                vertical={false}
              />
              <XAxis
                dataKey="date"
                stroke="var(--color-text-tertiary)"
                fontSize={10}
                tickLine={false}
                axisLine={false}
                fontFamily="var(--font-mono)"
              />
              <YAxis
                stroke="var(--color-text-tertiary)"
                fontSize={10}
                tickLine={false}
                axisLine={false}
                tickFormatter={shortBytes}
                width={40}
              />
              <Tooltip
                contentStyle={{
                  background: "var(--color-surface-solid)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  color: "var(--color-text-primary)",
                  fontSize: "12px",
                }}
                formatter={(value) => formatBytes(Number(value))}
                cursor={{ fill: "var(--color-surface-hover)", opacity: 0.5 }}
              />
              <Bar
                dataKey="upload"
                name={t("traffic_chart.upload")}
                fill="var(--color-info)"
                opacity={0.55}
                radius={[2, 2, 0, 0]}
              />
              <Bar
                dataKey="download"
                name={t("traffic_chart.download")}
                fill="var(--color-success)"
                opacity={0.45}
                radius={[2, 2, 0, 0]}
              />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Footer: legend + summary */}
      <div className="flex items-center justify-between px-[18px] pb-3.5 pt-1.5 text-[11px] text-[var(--color-text-tertiary)]">
        <div className="flex gap-3">
          <span className="flex items-center gap-1">
            <i className="inline-block h-2 w-2 rounded-sm bg-[var(--color-info)]" />
            {t("traffic_chart.upload")}
          </span>
          <span className="flex items-center gap-1">
            <i className="inline-block h-2 w-2 rounded-sm bg-[var(--color-success)]" />
            {t("traffic_chart.download")}
          </span>
        </div>
        <span className="font-mono">{summaryLine}</span>
      </div>
    </div>
  );
}

function shortBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0";
  const units = ["B", "K", "M", "G", "T"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)}${units[i]}`;
}
