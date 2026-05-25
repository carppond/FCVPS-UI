/**
 * Dashboard v4 — Recent events list panel.
 *
 * Compact event list in the bottom-right panel showing the latest 6 events
 * with colored circle icon, message, and relative timestamp.
 * Icon color is determined by event type/status.
 */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  AlertTriangle,
  CheckCircle2,
  Radio,
  XCircle,
} from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { useEvents } from "@/api/notify";
import type { NotificationEvent } from "@/types/api";

// ── helpers ─────────────────────────────────────────────────────────────────

interface EventRow {
  id: string;
  at: number;
  icon: React.ReactNode;
  iconBg: string;
  text: string;
}

function eventToRow(ev: NotificationEvent, t: (k: string) => string): EventRow {
  const isSent = ev.status === "sent";
  let icon: React.ReactNode;
  let iconBg: string;
  let text: string;

  const evType: string = ev.event_type ?? "";

  if (evType === "subscription_sync_failed") {
    icon = <XCircle className="h-3 w-3 text-[var(--color-error)]" />;
    iconBg = "var(--color-error-bg)";
    text = t("events.kind.notify_failed");
  } else if (evType === "node_offline") {
    icon = <AlertTriangle className="h-3 w-3 text-[var(--color-warning)]" />;
    iconBg = "var(--color-warning-bg)";
    text = t("events.kind.agent_status");
  } else if (evType === "ota_available" || evType === "script_alert") {
    icon = <Radio className="h-3 w-3 text-[var(--color-info)]" />;
    iconBg = "var(--color-info-bg)";
    text = t("events.kind.notify_sent");
  } else if (evType === "backup_completed") {
    icon = <CheckCircle2 className="h-3 w-3 text-[var(--color-success)]" />;
    iconBg = "var(--color-success-bg)";
    text = t("events.kind.subscription_sync");
  } else if (isSent) {
    icon = <CheckCircle2 className="h-3 w-3 text-[var(--color-success)]" />;
    iconBg = "var(--color-success-bg)";
    text = t("events.kind.notify_sent");
  } else {
    icon = <AlertTriangle className="h-3 w-3 text-[var(--color-warning)]" />;
    iconBg = "var(--color-warning-bg)";
    text = t("events.kind.notify_failed");
  }

  // If there's a message/payload summary, append it
  const detail = (ev as unknown as Record<string, string>).message ??
    (ev as unknown as Record<string, string>).summary ?? "";
  if (detail) text = detail;

  return {
    id: `evt-${ev.id}`,
    at: ev.created_at,
    icon,
    iconBg,
    text,
  };
}

function relativeTime(ts: number, lang: string): string {
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

// ── Main component ──────────────────────────────────────────────────────────

export function RecentEvents() {
  const { t, i18n } = useTranslation("dashboard");
  const query = useEvents({ page: 1, pageSize: 6 });

  const rows = useMemo<EventRow[]>(() => {
    const items = query.data?.items ?? [];
    return items
      .map((ev) => eventToRow(ev, t))
      .sort((a, b) => b.at - a.at)
      .slice(0, 6);
  }, [query.data, t]);

  return (
    <div className="flex flex-col overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] backdrop-blur-2xl">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[var(--color-border)] px-[18px] py-3.5">
        <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">
          {t("events.title")}
        </h3>
        <Link
          to={"/notifications" as unknown as "/"}
          className="cursor-pointer text-[11px] text-[var(--color-text-tertiary)] transition-colors hover:text-[var(--color-text-primary)]"
        >
          {t("events.view_all")} →
        </Link>
      </div>

      {/* Event rows */}
      {query.isLoading ? (
        <div className="flex flex-col">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex items-center gap-2.5 px-[18px] py-2.5">
              <Skeleton className="h-6 w-6 rounded-md" />
              <Skeleton className="h-4 flex-1" />
              <Skeleton className="h-3 w-12" />
            </div>
          ))}
        </div>
      ) : rows.length === 0 ? (
        <div className="p-6">
          <EmptyState title={t("events.no_events")} />
        </div>
      ) : (
        <div>
          {rows.map((row) => (
            <div
              key={row.id}
              className="flex items-center gap-2.5 border-b border-[var(--color-border)] px-[18px] py-2.5 text-[12.5px] transition-colors last:border-b-0 hover:bg-[var(--color-bg-elevated)]"
            >
              <span
                className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-md"
                style={{ background: row.iconBg }}
              >
                {row.icon}
              </span>
              <span className="flex-1 truncate text-[var(--color-text-primary)]">
                {row.text}
              </span>
              <time
                className="flex-shrink-0 whitespace-nowrap font-mono text-[10px] text-[var(--color-text-tertiary)]"
                dateTime={new Date(row.at).toISOString()}
              >
                {relativeTime(row.at, i18n.language)}
              </time>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
