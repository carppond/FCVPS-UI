/**
 * T-29: Subscription health panel.
 *
 * Cards listing each of the user's subscriptions with:
 *   - name + node count
 *   - last sync timestamp (relative)
 *   - sync status badge mapped to one of success / failed / pending / unknown
 *
 * Clicking a row navigates to /subscriptions/{id}. Bound to the same query
 * key as the M-SUB list so a sync triggered elsewhere (e.g. from cmd-k)
 * refreshes the health panel automatically.
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { CheckCircle2, ChevronRight, RefreshCcw, XCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useSubscriptionsQuery } from "@/api/subscription";
import type { Subscription, SyncStatus } from "@/types/api";

interface StatusVisual {
  icon: React.ReactNode;
  badge: "default" | "secondary" | "destructive" | "outline";
  labelKey: string;
}

function visualFor(status: SyncStatus | undefined): StatusVisual {
  switch (status) {
    case "ok":
      return {
        icon: <CheckCircle2 className="h-4 w-4 text-[var(--color-success)]" />,
        badge: "secondary",
        labelKey: "grid.subs.status.success",
      };
    case "error":
      return {
        icon: <XCircle className="h-4 w-4 text-[var(--color-error)]" />,
        badge: "destructive",
        labelKey: "grid.subs.status.failed",
      };
    case "pending":
      return {
        icon: <RefreshCcw className="h-4 w-4 text-[var(--color-warning)]" />,
        badge: "outline",
        labelKey: "grid.subs.status.pending",
      };
    default:
      return {
        icon: <RefreshCcw className="h-4 w-4 text-[var(--color-text-tertiary)]" />,
        badge: "outline",
        labelKey: "grid.subs.status.unknown",
      };
  }
}

function relativeShort(ms: number | undefined, lang: string): string | null {
  if (!ms) return null;
  if (typeof Intl === "undefined" || !("RelativeTimeFormat" in Intl)) {
    return new Date(ms).toLocaleString();
  }
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: "auto" });
  const diff = ms - Date.now();
  const abs = Math.abs(diff);
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (abs < minute) return rtf.format(Math.round(diff / 1000), "second");
  if (abs < hour) return rtf.format(Math.round(diff / minute), "minute");
  if (abs < day) return rtf.format(Math.round(diff / hour), "hour");
  return rtf.format(Math.round(diff / day), "day");
}

function Row({ sub }: { sub: Subscription }) {
  const { t, i18n } = useTranslation("dashboard");
  const visual = visualFor(sub.last_sync_status);
  const relative = relativeShort(sub.last_synced_at, i18n.language);

  return (
    <Link
      to={"/subscriptions/$id" as unknown as "/"}
      params={{ id: sub.id } as never}
      className="flex items-center gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-[var(--spacing-3)] transition-colors duration-[var(--duration-fast)] hover:bg-[var(--color-surface-hover)]"
    >
      <span className="flex h-6 w-6 shrink-0 items-center justify-center">
        {visual.icon}
      </span>
      <div className="flex grow flex-col gap-0.5 overflow-hidden">
        <span className="truncate text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
          {sub.name}
        </span>
        <span className="truncate text-[var(--font-size-xs)] text-[var(--color-text-tertiary)] tabular-nums">
          {t("grid.subs.node_count", { count: sub.node_count })} ·{" "}
          {relative
            ? t("grid.subs.last_synced_relative", { relative })
            : t("grid.subs.never_synced")}
        </span>
      </div>
      <Badge variant={visual.badge} className="shrink-0">
        {t(visual.labelKey)}
      </Badge>
      <ChevronRight className="h-3.5 w-3.5 shrink-0 text-[var(--color-text-disabled)]" />
    </Link>
  );
}

/** Per-subscription health panel — top 5 by last_synced_at. */
export function SubHealth() {
  const { t } = useTranslation("dashboard");
  const query = useSubscriptionsQuery({ page: 1, pageSize: 10 });

  const rows = useMemo(() => {
    const items = query.data?.items ?? [];
    // Most-recently synced first, with never-synced rows last.
    return [...items].sort((a, b) => {
      const ax = a.last_synced_at ?? 0;
      const bx = b.last_synced_at ?? 0;
      return bx - ax;
    });
  }, [query.data]);

  return (
    <Card className="flex h-full flex-col gap-[var(--spacing-3)] p-[var(--spacing-4)]">
      <header className="flex items-start justify-between">
        <div className="flex flex-col">
          <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {t("grid.subs.title")}
          </h3>
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("grid.subs.subtitle")}
          </span>
        </div>
        <Link
          to={"/subscriptions" as unknown as "/"}
          className="flex items-center gap-1 text-[var(--font-size-xs)] text-[var(--color-primary)] hover:underline"
        >
          {t("grid.subs.view_all")}
          <ChevronRight className="h-3 w-3" />
        </Link>
      </header>

      {query.isLoading ? (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState message={t("error.load_failed")} />
      ) : rows.length === 0 ? (
        <EmptyState title={t("grid.subs.no_subs")} />
      ) : (
        <div className="flex flex-col gap-2">
          {rows.map((sub) => (
            <Row key={sub.id} sub={sub} />
          ))}
        </div>
      )}
    </Card>
  );
}
