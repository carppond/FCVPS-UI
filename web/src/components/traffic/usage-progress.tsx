import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";

interface UsageProgressProps {
  /** Bytes used so far this period. */
  used: number;
  /** Bytes allowed this period. 0 / undefined means "no limit". */
  limit?: number;
  /** Days remaining until the next reset (controls the helper line). */
  daysToReset?: number;
}

/**
 * Hero progress bar used at the top of the Traffic page. Three colour bands
 * mirror the threshold ladder (green < 80 < amber < 90 < red); the bar is
 * also rendered when the limit is unset so the user still sees the "no
 * limit" affordance instead of a blank space.
 */
export function UsageProgress({ used, limit, daysToReset }: UsageProgressProps) {
  const { t } = useTranslation(["traffic", "common"]);
  const hasLimit = typeof limit === "number" && limit > 0;
  const percent = hasLimit ? Math.min(100, (used / (limit as number)) * 100) : 0;
  const remaining = hasLimit ? Math.max(0, (limit as number) - used) : 0;
  const band = percent >= 90 ? "danger" : percent >= 80 ? "warning" : "success";

  return (
    <div className="flex flex-col gap-[var(--spacing-3)]">
      <div className="flex items-baseline justify-between gap-[var(--spacing-2)]">
        <div className="flex items-baseline gap-[var(--spacing-2)]">
          <span className="text-[var(--font-size-3xl)] font-semibold text-[var(--color-text-primary)]">
            {formatBytes(used)}
          </span>
          <span className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {hasLimit
              ? `/ ${formatBytes(limit as number)}`
              : t("traffic:summary.no_limit")}
          </span>
        </div>
        {hasLimit ? (
          <span
            className={cn(
              "text-[var(--font-size-sm)] font-medium",
              band === "danger"
                ? "text-[var(--color-error)]"
                : band === "warning"
                ? "text-[var(--color-warning)]"
                : "text-[var(--color-success)]",
            )}
          >
            {t("traffic:summary.usage_percent", { percent: percent.toFixed(1) })}
          </span>
        ) : null}
      </div>
      <div
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={hasLimit ? percent : undefined}
        className="relative h-3 w-full overflow-hidden rounded-[var(--radius-full)] bg-[var(--color-surface-hover)]"
      >
        <div
          className={cn(
            "h-full rounded-[var(--radius-full)] transition-[width] duration-[var(--duration-normal)]",
            band === "danger"
              ? "bg-[var(--color-error)]"
              : band === "warning"
              ? "bg-[var(--color-warning)]"
              : "bg-[var(--color-success)]",
          )}
          style={{ width: `${hasLimit ? percent : 0}%` }}
        />
      </div>
      <div className="flex items-center justify-between text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        <span>
          {hasLimit
            ? `${t("traffic:summary.remaining")}: ${formatBytes(remaining)}`
            : ""}
        </span>
        {typeof daysToReset === "number" ? (
          <span>{t("traffic:summary.days_to_reset", { count: daysToReset })}</span>
        ) : null}
      </div>
    </div>
  );
}

// formatBytes returns a human-readable size in B / KB / MB / GB / TB. We
// keep the formatter local — the Traffic page is the only consumer for now,
// and the global util library does not yet ship a bytes helper.
function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 2)} ${units[i]}`;
}
