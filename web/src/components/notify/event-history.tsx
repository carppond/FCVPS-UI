import * as React from "react";
import { useTranslation } from "react-i18next";
import { CheckCircle2, XCircle, Clock, MinusCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { ChannelIcon } from "@/components/notify/channel-icon";
import { EVENT_TYPES } from "@/components/notify/channel-form";
import { useEvents } from "@/api/notify";
import { formatDate } from "@/lib/format";
import { classifySyncError } from "@/lib/sync-error";
import { cn } from "@/lib/cn";
import type {
  EventStatus,
  EventType,
  NotificationChannel,
} from "@/types/api";

interface EventHistoryProps {
  /** Channel list — used to render the per-row kind icon + name. */
  channels: NotificationChannel[];
  /** Optional channel filter. */
  channelId?: string;
  /** Optional event-type filter. */
  eventType?: EventType | "";
}

const PAGE_SIZE = 20;

/**
 * Paged delivery-history table backed by GET /api/notify/events.
 *
 * Renders the canonical 4-state contract:
 *   loading  → Skeleton
 *   error    → ErrorState with retry
 *   empty    → EmptyState
 *   loaded   → table rows
 *
 * Status uses a coloured icon (CheckCircle2 / XCircle / Clock / MinusCircle)
 * plus a textual badge — color alone never carries meaning per the design
 * tokens' a11y guidance.
 */
export function EventHistory({
  channels,
  channelId,
  eventType,
}: EventHistoryProps) {
  const { t } = useTranslation(["notify", "common"]);
  const [page, setPage] = React.useState(1);
  const [statusFilter, setStatusFilter] = React.useState<EventStatus | "">("");
  const [eventFilter, setEventFilter] = React.useState<EventType | "">(
    eventType ?? "",
  );

  React.useEffect(() => {
    setPage(1);
  }, [channelId, eventType]);

  const eventsQ = useEvents({
    page,
    pageSize: PAGE_SIZE,
    channelId,
    eventType: eventFilter,
    status: statusFilter,
  });

  const channelById = React.useMemo(() => {
    const m = new Map<string, NotificationChannel>();
    for (const c of channels) m.set(c.id, c);
    return m;
  }, [channels]);

  return (
    <section
      className="flex flex-col gap-[var(--spacing-3)]"
      data-testid="notify-event-history"
    >
      <header className="flex flex-wrap items-end justify-between gap-[var(--spacing-2)]">
        <div>
          <h3 className="text-[var(--font-size-base)] font-semibold text-[var(--color-text-primary)]">
            {t("notify:history.title")}
          </h3>
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("notify:history.subtitle")}
          </p>
        </div>
        <div className="flex items-center gap-[var(--spacing-2)]">
          <select
            value={eventFilter}
            onChange={(e) => {
              setEventFilter(e.target.value as EventType | "");
              setPage(1);
            }}
            className="h-8 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-2 text-[var(--font-size-xs)] text-[var(--color-text-primary)]"
            data-testid="notify-history-event-filter"
          >
            <option value="">{t("notify:history.all_events")}</option>
            {EVENT_TYPES.map((evt) => (
              <option key={evt} value={evt}>
                {t(`notify:events.${evt}.name`)}
              </option>
            ))}
          </select>
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value as EventStatus | "");
              setPage(1);
            }}
            className="h-8 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-2 text-[var(--font-size-xs)] text-[var(--color-text-primary)]"
            data-testid="notify-history-status-filter"
          >
            <option value="">{t("notify:history.all_statuses")}</option>
            <option value="sent">{t("notify:history.status.sent")}</option>
            <option value="failed">{t("notify:history.status.failed")}</option>
            <option value="pending">{t("notify:history.status.pending")}</option>
            <option value="skipped_dedupe">
              {t("notify:history.status.skipped_dedupe")}
            </option>
          </select>
        </div>
      </header>

      {eventsQ.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : eventsQ.isError ? (
        <ErrorState
          message={t("notify:history.load_failed")}
          onRetry={() => void eventsQ.refetch()}
          retryLabel={t("common:actions.retry")}
        />
      ) : !eventsQ.data || eventsQ.data.items.length === 0 ? (
        <EmptyState
          title={t("notify:history.empty_title")}
          description={t("notify:history.empty_description")}
        />
      ) : (
        <>
          <div className="overflow-hidden rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)]">
            <table className="w-full text-[var(--font-size-sm)]">
              <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
                <tr>
                  <th className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-center text-[var(--font-size-xs)] font-medium uppercase tracking-wide">
                    {t("notify:history.col_time")}
                  </th>
                  <th className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-center text-[var(--font-size-xs)] font-medium uppercase tracking-wide">
                    {t("notify:history.col_event")}
                  </th>
                  <th className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-center text-[var(--font-size-xs)] font-medium uppercase tracking-wide">
                    {t("notify:history.col_channel")}
                  </th>
                  <th className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-center text-[var(--font-size-xs)] font-medium uppercase tracking-wide">
                    {t("notify:history.col_status")}
                  </th>
                  <th className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-center text-[var(--font-size-xs)] font-medium uppercase tracking-wide">
                    {t("notify:history.col_detail")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {eventsQ.data.items.map((ev) => {
                  const channel = ev.channel_id
                    ? channelById.get(ev.channel_id)
                    : undefined;
                  return (
                    <tr
                      key={ev.id}
                      className="border-b border-[var(--color-border)] last:border-0 hover:bg-[var(--color-surface-hover)]"
                      data-testid={`notify-history-row-${ev.id}`}
                    >
                      <td className="px-[var(--spacing-3)] py-[var(--spacing-2)] tabular-nums text-[var(--color-text-secondary)]">
                        {formatDate(ev.sent_at ?? ev.created_at)}
                      </td>
                      <td className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-[var(--color-text-primary)]">
                        {t(`notify:events.${ev.event_type}.name`)}
                      </td>
                      <td className="px-[var(--spacing-3)] py-[var(--spacing-2)]">
                        {channel ? (
                          <span className="inline-flex items-center gap-[var(--spacing-1)] text-[var(--color-text-secondary)]">
                            <ChannelIcon
                              kind={channel.kind}
                              className="h-4 w-4 text-[var(--color-text-tertiary)]"
                            />
                            {channel.name}
                          </span>
                        ) : (
                          <span className="text-[var(--color-text-disabled)]">
                            {t("notify:history.channel_unknown")}
                          </span>
                        )}
                      </td>
                      <td className="px-[var(--spacing-3)] py-[var(--spacing-2)]">
                        <StatusBadge status={ev.status} />
                      </td>
                      <td className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-[var(--color-text-secondary)]">
                        {ev.error ? (
                          // Localized summary only; raw error via tooltip.
                          <span
                            title={ev.error}
                            className="truncate text-[var(--color-error)]"
                          >
                            {(() => {
                              const kind = classifySyncError(ev.error);
                              return kind
                                ? t(`errors:NET_${kind.toUpperCase()}`)
                                : t("notify:history.send_failed");
                            })()}
                          </span>
                        ) : (
                          <span className="text-[var(--color-text-tertiary)]">
                            —
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            <span>
              {(page - 1) * PAGE_SIZE + 1} –{" "}
              {Math.min(page * PAGE_SIZE, eventsQ.data.total)} /{" "}
              {eventsQ.data.total}
            </span>
            <div className="flex items-center gap-[var(--spacing-2)]">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                {t("common:actions.back")}
              </Button>
              <span>
                {page} /{" "}
                {Math.max(1, Math.ceil(eventsQ.data.total / PAGE_SIZE))}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={page * PAGE_SIZE >= eventsQ.data.total}
                onClick={() => setPage((p) => p + 1)}
              >
                {">"}
              </Button>
            </div>
          </div>
        </>
      )}
    </section>
  );
}

function StatusBadge({ status }: { status: EventStatus }) {
  const { t } = useTranslation("notify");
  switch (status) {
    case "sent":
      return (
        <Badge
          variant="default"
          className={cn(
            "inline-flex items-center gap-1 bg-[var(--color-success-bg)] text-[var(--color-success)]",
          )}
        >
          <CheckCircle2 className="h-3 w-3" />
          {t("notify:history.status.sent")}
        </Badge>
      );
    case "failed":
      return (
        <Badge variant="destructive" className="inline-flex items-center gap-1">
          <XCircle className="h-3 w-3" />
          {t("notify:history.status.failed")}
        </Badge>
      );
    case "pending":
      return (
        <Badge variant="secondary" className="inline-flex items-center gap-1">
          <Clock className="h-3 w-3" />
          {t("notify:history.status.pending")}
        </Badge>
      );
    case "skipped_dedupe":
    default:
      return (
        <Badge variant="outline" className="inline-flex items-center gap-1">
          <MinusCircle className="h-3 w-3" />
          {t("notify:history.status.skipped_dedupe")}
        </Badge>
      );
  }
}
