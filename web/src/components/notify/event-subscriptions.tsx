import * as React from "react";
import { useTranslation } from "react-i18next";
import { Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { ChannelIcon } from "@/components/notify/channel-icon";
import { EVENT_TYPES } from "@/components/notify/channel-form";
import { useSaveSubscriptionMatrix } from "@/api/notify";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import type { EventType, NotificationChannel } from "@/types/api";

interface EventSubscriptionsProps {
  channels: NotificationChannel[];
}

/**
 * Event × Channel subscription matrix.
 *
 *   - Rows: 7 known EventTypes (canonical list from `EVENT_TYPES`).
 *   - Cols: every NotificationChannel owned by the current user.
 *   - Cell: checkbox — checked = the channel subscribes to that event.
 *
 * Local state holds the staged matrix; the "save" button diffs against the
 * channels prop and PATCHes only the channels whose subscription set changed
 * (one mutation per channel — see useSaveSubscriptionMatrix in api/notify).
 */
export function EventSubscriptions({ channels }: EventSubscriptionsProps) {
  const { t } = useTranslation(["notify", "common"]);
  const { handle: handleError } = useApiError();
  const saveMutation = useSaveSubscriptionMatrix();

  // matrix[channelId] = Set<EventType>
  const [matrix, setMatrix] = React.useState<Record<string, Set<EventType>>>(
    () => seedMatrix(channels),
  );

  // Re-seed whenever the upstream channels change (e.g. after a create).
  // We intentionally use channels.length + an `event_types` fingerprint so
  // touching a label doesn't blow away the user's pending edits.
  const fingerprint = React.useMemo(
    () =>
      channels
        .map((c) => `${c.id}:${[...c.event_types].sort().join(",")}`)
        .join("|"),
    [channels],
  );
  React.useEffect(() => {
    setMatrix(seedMatrix(channels));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fingerprint]);

  const toggle = React.useCallback(
    (channelId: string, evt: EventType) => {
      setMatrix((prev) => {
        const next = { ...prev };
        const current = new Set(next[channelId] ?? []);
        if (current.has(evt)) current.delete(evt);
        else current.add(evt);
        next[channelId] = current;
        return next;
      });
    },
    [],
  );

  // Compute the set of channels whose stored event_types differs from the
  // staged matrix — only these get PATCHed when the user clicks save.
  const dirtyUpdates = React.useMemo(() => {
    const updates: Array<{ id: string; event_types: EventType[] }> = [];
    for (const c of channels) {
      const stored = new Set(c.event_types);
      const staged = matrix[c.id] ?? new Set<EventType>();
      if (!setEquals(stored, staged)) {
        updates.push({
          id: c.id,
          event_types: [...staged],
        });
      }
    }
    return updates;
  }, [channels, matrix]);

  const handleSave = async () => {
    if (dirtyUpdates.length === 0) return;
    try {
      await saveMutation.mutateAsync(dirtyUpdates);
      toast.success(t("notify:matrix.saved"));
    } catch (err) {
      handleError(err);
    }
  };

  const handleReset = () => {
    setMatrix(seedMatrix(channels));
  };

  if (channels.length === 0) {
    return (
      <div
        className="rounded-[var(--radius-md)] border border-dashed border-[var(--color-border)] p-[var(--spacing-6)] text-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]"
        data-testid="notify-matrix-empty"
      >
        {t("notify:matrix.empty")}
      </div>
    );
  }

  return (
    <div
      className="flex flex-col gap-[var(--spacing-3)]"
      data-testid="notify-matrix"
    >
      <header className="flex items-center justify-between">
        <div>
          <h3 className="text-[var(--font-size-base)] font-semibold text-[var(--color-text-primary)]">
            {t("notify:matrix.title")}
          </h3>
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("notify:matrix.subtitle")}
          </p>
        </div>
        <div className="flex items-center gap-[var(--spacing-2)]">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={dirtyUpdates.length === 0 || saveMutation.isPending}
            onClick={handleReset}
          >
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="button"
            size="sm"
            disabled={dirtyUpdates.length === 0 || saveMutation.isPending}
            onClick={handleSave}
            data-testid="notify-matrix-save"
          >
            <Save className="h-4 w-4" />
            {t("notify:matrix.save_count", { count: dirtyUpdates.length })}
          </Button>
        </div>
      </header>

      <div className="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)]">
        <table className="w-full border-collapse text-[var(--font-size-sm)]">
          <thead>
            <tr className="border-b border-[var(--color-border)]">
              <th className="sticky left-0 z-10 bg-[var(--color-surface)] px-[var(--spacing-3)] py-[var(--spacing-2)] text-left text-[var(--font-size-xs)] font-medium uppercase tracking-wide text-[var(--color-text-tertiary)]">
                {t("notify:matrix.col_event")}
              </th>
              {channels.map((c) => (
                <th
                  key={c.id}
                  className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-center text-[var(--font-size-xs)] font-medium text-[var(--color-text-secondary)]"
                  title={c.name}
                >
                  <div className="flex flex-col items-center gap-[var(--spacing-1)]">
                    <ChannelIcon
                      kind={c.kind}
                      className="text-[var(--color-text-secondary)]"
                    />
                    <span className="max-w-[8ch] truncate">{c.name}</span>
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {EVENT_TYPES.map((evt) => (
              <tr
                key={evt}
                className="border-b border-[var(--color-border)] last:border-0 hover:bg-[var(--color-surface-hover)]"
              >
                <th
                  scope="row"
                  className="sticky left-0 z-10 bg-[var(--color-surface)] px-[var(--spacing-3)] py-[var(--spacing-2)] text-left font-normal"
                >
                  <span className="font-medium text-[var(--color-text-primary)]">
                    {t(`notify:events.${evt}.name`)}
                  </span>
                  <span className="ml-2 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                    {t(`notify:events.${evt}.description`, "")}
                  </span>
                </th>
                {channels.map((c) => {
                  const checked = matrix[c.id]?.has(evt) ?? false;
                  const wasChecked = c.event_types.includes(evt);
                  const dirty = checked !== wasChecked;
                  return (
                    <td
                      key={c.id}
                      className="px-[var(--spacing-3)] py-[var(--spacing-2)] text-center"
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggle(c.id, evt)}
                        disabled={!c.enabled}
                        aria-label={`${t(`notify:events.${evt}.name`)} → ${c.name}`}
                        className={cn(
                          "h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]",
                          dirty &&
                            "outline outline-1 outline-[var(--color-primary)]",
                        )}
                        data-testid={`notify-matrix-cell-${evt}-${c.id}`}
                      />
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ─── Helpers ────────────────────────────────────────────────────────────────

function seedMatrix(
  channels: NotificationChannel[],
): Record<string, Set<EventType>> {
  const out: Record<string, Set<EventType>> = {};
  for (const c of channels) {
    out[c.id] = new Set(c.event_types);
  }
  return out;
}

function setEquals<T>(a: Set<T>, b: Set<T>): boolean {
  if (a.size !== b.size) return false;
  for (const x of a) if (!b.has(x)) return false;
  return true;
}
