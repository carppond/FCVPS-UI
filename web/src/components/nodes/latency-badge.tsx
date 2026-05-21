import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/cn";

/**
 * Visualises a TCPing latency reading.
 *
 *  - `null` / undefined  → 灰，未测
 *  - `-1`                → 红，不可达
 *  - `< 100`             → 绿
 *  - `100 - 300`         → 黄
 *  - `> 300`             → 红 (用 warning bg + error fg 与不可达区分)
 *
 * The component prints `<ms>ms` as `tabular-nums` so the number column lines
 * up across rows (per _dev-cheatsheet.md numeric-first rule).
 */
interface LatencyBadgeProps {
  latencyMs?: number | null;
  className?: string;
}

export function LatencyBadge({ latencyMs, className }: LatencyBadgeProps) {
  const { t } = useTranslation("node");

  if (latencyMs === null || latencyMs === undefined) {
    return (
      <Badge
        variant="outline"
        className={cn(
          "bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)]",
          "tabular-nums",
          className,
        )}
      >
        {t("latency.untested")}
      </Badge>
    );
  }

  if (latencyMs < 0) {
    return (
      <Badge
        variant="outline"
        className={cn(
          "bg-[var(--color-error-bg)] text-[var(--color-error)]",
          "tabular-nums",
          className,
        )}
      >
        {t("latency.unreachable")}
      </Badge>
    );
  }

  const tone = latencyMs < 100
    ? "bg-[var(--color-success-bg)] text-[var(--color-success)]"
    : latencyMs < 300
      ? "bg-[var(--color-warning-bg)] text-[var(--color-warning)]"
      : "bg-[var(--color-error-bg)] text-[var(--color-error)]";

  return (
    <Badge
      variant="outline"
      className={cn(tone, "tabular-nums", className)}
    >
      {latencyMs} ms
    </Badge>
  );
}
