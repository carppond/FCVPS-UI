import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/lib/cn";
import {
  useTrafficHistoryQuery,
  type TrafficHistoryRange,
  type TrafficHistoryView,
} from "@/api/traffic";

interface TrafficChartProps {
  className?: string;
}

/**
 * 30 day (default) traffic trend chart. Toggle row in the header lets the
 * user flip between day and month buckets, plus pick the time window. Series
 * colours come straight from the design tokens (--color-chart-1 / chart-2)
 * so dark and light themes stay in sync.
 */
export function TrafficChart({ className }: TrafficChartProps) {
  const { t } = useTranslation(["traffic", "common"]);
  const [view, setView] = useState<TrafficHistoryView>("day");
  const [range, setRange] = useState<TrafficHistoryRange>("30d");
  const { data, isLoading, isError } = useTrafficHistoryQuery({ view, range });

  const points = useMemo(() => {
    return (data ?? []).map((p) => ({
      date: p.date,
      inbound: p.total_in,
      outbound: p.total_out,
      total: p.total_in + p.total_out,
    }));
  }, [data]);

  return (
    <div
      className={cn(
        "flex flex-col gap-[var(--spacing-4)] rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]",
        className,
      )}
    >
      <div className="flex flex-col gap-[var(--spacing-2)] sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col">
          <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {t("traffic:chart.title")}
          </h3>
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("traffic:chart.subtitle")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-[var(--spacing-2)]">
          <div className="inline-flex overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border-strong)]">
            <ToggleButton
              active={view === "day"}
              onClick={() => setView("day")}
              label={t("traffic:chart.view_day")}
            />
            <ToggleButton
              active={view === "month"}
              onClick={() => setView("month")}
              label={t("traffic:chart.view_month")}
            />
          </div>
          <div className="inline-flex overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border-strong)]">
            <ToggleButton
              active={range === "7d"}
              onClick={() => setRange("7d")}
              label={t("traffic:chart.range_7d")}
            />
            <ToggleButton
              active={range === "30d"}
              onClick={() => setRange("30d")}
              label={t("traffic:chart.range_30d")}
            />
            <ToggleButton
              active={range === "90d"}
              onClick={() => setRange("90d")}
              label={t("traffic:chart.range_90d")}
            />
          </div>
        </div>
      </div>
      <div className="h-72 w-full">
        {isLoading ? (
          <div className="flex h-full items-center justify-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("common:status.loading")}
          </div>
        ) : isError || points.length === 0 ? (
          <EmptyState title={t("traffic:chart.empty")} />
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={points}>
              <CartesianGrid stroke="var(--color-border)" strokeDasharray="3 3" />
              <XAxis
                dataKey="date"
                stroke="var(--color-text-tertiary)"
                fontSize={12}
                tickLine={false}
              />
              <YAxis
                stroke="var(--color-text-tertiary)"
                fontSize={12}
                tickFormatter={shortBytes}
                tickLine={false}
              />
              <Tooltip
                contentStyle={{
                  background: "var(--color-bg-elevated)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  color: "var(--color-text-primary)",
                  fontSize: "var(--font-size-sm)",
                }}
                formatter={(value) => shortBytes(Number(value))}
              />
              <Legend
                wrapperStyle={{
                  color: "var(--color-text-secondary)",
                  fontSize: "var(--font-size-xs)",
                }}
              />
              <Line
                type="monotone"
                dataKey="inbound"
                name={t("traffic:chart.series_in") as string}
                stroke="var(--color-chart-1)"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="outbound"
                name={t("traffic:chart.series_out") as string}
                stroke="var(--color-chart-2)"
                strokeWidth={2}
                dot={false}
              />
              <Line
                type="monotone"
                dataKey="total"
                name={t("traffic:chart.series_total") as string}
                stroke="var(--color-chart-3)"
                strokeWidth={2}
                strokeDasharray="4 2"
                dot={false}
              />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>
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
        "h-7 rounded-none px-[var(--spacing-3)] text-[var(--font-size-xs)]",
        active
          ? "bg-[var(--color-primary)] text-[var(--color-primary-foreground)] hover:bg-[var(--color-primary)]"
          : "bg-transparent text-[var(--color-text-secondary)]",
      )}
    >
      {label}
    </Button>
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
