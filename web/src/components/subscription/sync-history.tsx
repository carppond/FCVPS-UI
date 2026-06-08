import { useTranslation } from "react-i18next";
import { CheckCircle2, XCircle } from "lucide-react";
import { useSubscriptionSyncLogsQuery } from "@/api/subscription";
import { Skeleton } from "@/components/ui/skeleton";

/**
 * Full sync-history list for the subscription detail page — replaces the old
 * "latest result only" placeholder now that the backend records history.
 */
export function SyncHistory({ subscriptionId }: { subscriptionId: string }) {
  const { t } = useTranslation(["subscription"]);
  const { data, isLoading } = useSubscriptionSyncLogsQuery(subscriptionId);
  const logs = data?.items ?? [];

  if (isLoading) {
    return (
      <div className="flex flex-col gap-1.5">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-9 rounded-md" />
        ))}
      </div>
    );
  }

  if (logs.length === 0) {
    return (
      <p className="py-3 text-center text-[11.5px] text-[var(--color-text-tertiary)]">
        {t("subscription:detail.sync_history.empty")}
      </p>
    );
  }

  return (
    <div className="flex flex-col divide-y divide-[var(--color-border)]">
      {logs.map((log) => {
        const ok = log.status === "ok";
        return (
          <div
            key={log.id}
            className="flex items-center gap-3 py-2 text-[var(--font-size-sm)] first:pt-0 last:pb-0"
          >
            {ok ? (
              <CheckCircle2 className="h-4 w-4 shrink-0 text-[var(--color-success)]" />
            ) : (
              <XCircle className="h-4 w-4 shrink-0 text-[var(--color-danger)]" />
            )}
            <span className="shrink-0 text-[var(--color-text-tertiary)] tabular-nums">
              {new Date(log.created_at).toLocaleString()}
            </span>
            <span className="min-w-0 flex-1 truncate text-[var(--color-text-secondary)]">
              {ok
                ? `${t("subscription:status.ok")} · ${t("subscription:detail.sync_history.node_count", { count: log.node_count })}`
                : (log.error || t("subscription:status.error"))}
            </span>
          </div>
        );
      })}
    </div>
  );
}
