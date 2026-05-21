/**
 * T-29: Recent events panel.
 *
 * Time-line list of the latest 20 events fused from two sources:
 *   - notification events (POST/FAIL deliveries) — owned by T-25.
 *   - audit log entries (left as a backend-side TODO; while the audit API
 *     does not exist yet we degrade gracefully to "notify only" so the panel
 *     still works in dev / staging without bouncing on 404).
 *
 * Each row renders an icon coloured by status + relative timestamp + i18n
 * label. The "view all" link jumps to /audit (admin) or /notifications
 * depending on the user role.
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  BellRing,
  CheckCircle2,
  ChevronRight,
  Clock,
  RefreshCcw,
} from "lucide-react";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useEvents } from "@/api/notify";
import { useAuthStore } from "@/stores/auth-store";
import type { NotificationEvent } from "@/types/api";

interface RecentRow {
  id: string;
  /** Unix ms. */
  at: number;
  kind: "notify_sent" | "notify_failed" | "agent_status" | "subscription_sync" | "audit";
  text: string;
  to?: string;
}

function iconFor(kind: RecentRow["kind"]) {
  switch (kind) {
    case "notify_sent":
      return <CheckCircle2 className="h-4 w-4 text-[var(--color-success)]" />;
    case "notify_failed":
      return <AlertTriangle className="h-4 w-4 text-[var(--color-error)]" />;
    case "agent_status":
      return <BellRing className="h-4 w-4 text-[var(--color-warning)]" />;
    case "subscription_sync":
      return <RefreshCcw className="h-4 w-4 text-[var(--color-info)]" />;
    case "audit":
    default:
      return <Clock className="h-4 w-4 text-[var(--color-text-tertiary)]" />;
  }
}

function notifyToRow(ev: NotificationEvent, t: (key: string) => string): RecentRow {
  const sent = ev.status === "sent";
  return {
    id: `notify-${ev.id}`,
    at: ev.created_at,
    kind: sent ? "notify_sent" : "notify_failed",
    text: sent
      ? t("grid.events.kind.notify_sent")
      : t("grid.events.kind.notify_failed"),
    to: "/notifications",
  };
}

function relativeTime(ts: number, lang: string): string {
  // RelativeTimeFormat is universally supported in modern browsers (Chrome 71+,
  // Safari 14+, Firefox 70+) — falls back to a static string only in jsdom.
  if (typeof Intl === "undefined" || !("RelativeTimeFormat" in Intl)) {
    return new Date(ts).toLocaleString();
  }
  const rtf = new Intl.RelativeTimeFormat(lang, { numeric: "auto" });
  const diffMs = ts - Date.now();
  const abs = Math.abs(diffMs);
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (abs < minute) return rtf.format(Math.round(diffMs / 1000), "second");
  if (abs < hour) return rtf.format(Math.round(diffMs / minute), "minute");
  if (abs < day) return rtf.format(Math.round(diffMs / hour), "hour");
  return rtf.format(Math.round(diffMs / day), "day");
}

/** Latest events panel — fixed at 20 rows max. */
export function RecentEvents() {
  const { t, i18n } = useTranslation("dashboard");
  const { user } = useAuthStore();

  const notify = useEvents({ page: 1, pageSize: 20 });

  const rows = useMemo<RecentRow[]>(() => {
    const out: RecentRow[] = [];
    for (const ev of notify.data?.items ?? []) {
      out.push(notifyToRow(ev, t));
    }
    // Sort by timestamp desc then trim. Once the audit API lands we'll merge
    // its rows here using the same shape.
    return out.sort((a, b) => b.at - a.at).slice(0, 20);
  }, [notify.data, t]);

  const allLink = user?.role === "admin" ? "/audit" : "/notifications";

  return (
    <Card className="flex h-full flex-col gap-[var(--spacing-3)] p-[var(--spacing-4)]">
      <header className="flex items-start justify-between">
        <div className="flex flex-col">
          <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {t("grid.events.title")}
          </h3>
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("grid.events.subtitle")}
          </span>
        </div>
        <Link
          to={allLink as unknown as "/"}
          className="flex items-center gap-1 text-[var(--font-size-xs)] text-[var(--color-primary)] hover:underline"
        >
          {t("grid.events.view_all")}
          <ChevronRight className="h-3 w-3" />
        </Link>
      </header>

      {notify.isLoading ? (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-7 w-full" />
          ))}
        </div>
      ) : notify.isError ? (
        <ErrorState message={t("error.load_failed")} />
      ) : rows.length === 0 ? (
        <EmptyState title={t("grid.events.no_events")} />
      ) : (
        <ul className="flex flex-col">
          {rows.map((row) => (
            <li
              key={row.id}
              className="flex items-center gap-3 border-b border-[var(--color-border)] py-[var(--spacing-2)] last:border-b-0"
            >
              <span className="flex h-6 w-6 shrink-0 items-center justify-center">
                {iconFor(row.kind)}
              </span>
              <span className="grow text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
                {row.text}
              </span>
              <time
                className="shrink-0 font-mono text-[var(--font-size-xs)] tabular-nums text-[var(--color-text-tertiary)]"
                dateTime={new Date(row.at).toISOString()}
              >
                {relativeTime(row.at, i18n.language)}
              </time>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
