import { useTranslation } from "react-i18next";
import { CheckCircle2, XCircle, Clock } from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";
import { formatDate, formatRelativeTime } from "@/lib/format";
import { cn } from "@/lib/cn";
import type { Subscription, SyncStatus } from "@/types/api";

interface SubSyncHistoryProps {
  subscription: Subscription;
}

/**
 * Sync history tab.
 *
 * The backend does not yet expose a per-subscription audit/history endpoint
 * (per docs/04-api-contract.md §1.2 + §M-SUB). For v1 we render the latest
 * sync row only — the same data that powers the list-page badge — and
 * announce that full history is forthcoming.
 */
export function SubSyncHistory({ subscription }: SubSyncHistoryProps) {
  const { t } = useTranslation("subscription");

  if (!subscription.last_synced_at) {
    return (
      <EmptyState
        icon={<Clock />}
        title={t("subscription:detail.sync_history.empty")}
        description={t("subscription:detail.sync_history.history_unavailable")}
      />
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        {t("subscription:detail.sync_history.history_unavailable")}
      </p>
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
        <HistoryRow
          status={subscription.last_sync_status}
          timestamp={subscription.last_synced_at}
          nodeCount={subscription.node_count}
          error={subscription.last_sync_error}
        />
      </div>
    </div>
  );
}

interface HistoryRowProps {
  status?: SyncStatus;
  timestamp: number;
  nodeCount: number;
  error?: string;
}

function HistoryRow({ status, timestamp, nodeCount, error }: HistoryRowProps) {
  const { t } = useTranslation("subscription");

  const icon = status === "error" ? (
    <XCircle className="h-4 w-4 text-[var(--color-error)]" />
  ) : status === "pending" ? (
    <Clock className="h-4 w-4 text-[var(--color-info)]" />
  ) : (
    <CheckCircle2 className="h-4 w-4 text-[var(--color-success)]" />
  );

  return (
    <div className="flex items-start gap-3 p-4">
      <div className="mt-0.5">{icon}</div>
      <div className="flex flex-1 flex-col gap-1">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
            {t(`subscription:status.${status ?? "ok"}`)}
          </span>
          <span
            className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)] tabular-nums"
            title={formatDate(timestamp)}
          >
            {formatRelativeTime(timestamp)}
          </span>
        </div>
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-secondary)] tabular-nums">
          {t("subscription:detail.sync_history.node_count", { count: nodeCount })}
        </p>
        {error && (
          <p className={cn(
            "rounded-[var(--radius-sm)] border border-[var(--color-error-bg)]",
            "bg-[var(--color-error-bg)] px-2 py-1",
            "font-mono text-[var(--font-size-xs)] text-[var(--color-error)]",
          )}>
            {error}
          </p>
        )}
      </div>
    </div>
  );
}
