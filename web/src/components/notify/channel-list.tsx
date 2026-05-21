import * as React from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ChannelIcon } from "@/components/notify/channel-icon";
import type { ChannelKind, NotificationChannel } from "@/types/api";
import { cn } from "@/lib/cn";

/**
 * The canonical kind order shown across the channel-list and dropdowns.
 * Mirrors the order from §2.18 of docs/04-api-contract.md so the UX is
 * stable across browsers.
 */
export const CHANNEL_KIND_ORDER: ChannelKind[] = [
  "telegram",
  "discord",
  "slack",
  "email",
  "bark",
  "gotify",
  "webhook",
  "serverchan",
  "pushdeer",
  "ifttt",
];

interface ChannelListProps {
  /** All channels owned by the current user (any kind, enabled or not). */
  channels: NotificationChannel[];
  /** Optional kind filter — when set, only matching kind cards are shown. */
  filterKind?: ChannelKind | "";
  /** Fires when the user clicks "new" on a kind card. */
  onCreate: (kind: ChannelKind) => void;
  /** Fires when the user clicks an already-configured channel row. */
  onSelect: (channel: NotificationChannel) => void;
  /** Channel currently selected in the parent — used for highlight only. */
  selectedId?: string;
}

/**
 * Two-section list:
 *   1. "Available kinds" — 10 cards, each with logo / name / "+ new"
 *   2. "Your channels"   — flat list of every configured channel
 *
 * Designed for the left rail of the notifications page; selecting a channel
 * pushes the detail pane to the middle column.
 */
export function ChannelList({
  channels,
  filterKind,
  onCreate,
  onSelect,
  selectedId,
}: ChannelListProps) {
  const { t } = useTranslation(["notify", "common"]);

  const visibleKinds = React.useMemo(() => {
    if (!filterKind) return CHANNEL_KIND_ORDER;
    return CHANNEL_KIND_ORDER.filter((k) => k === filterKind);
  }, [filterKind]);

  const channelsByKind = React.useMemo(() => {
    const map = new Map<ChannelKind, NotificationChannel[]>();
    for (const c of channels) {
      const arr = map.get(c.kind) ?? [];
      arr.push(c);
      map.set(c.kind, arr);
    }
    return map;
  }, [channels]);

  const filteredChannels = React.useMemo(
    () =>
      filterKind ? channels.filter((c) => c.kind === filterKind) : channels,
    [channels, filterKind],
  );

  return (
    <div className="flex flex-col gap-[var(--spacing-6)]">
      <section>
        <h3 className="mb-[var(--spacing-3)] text-[var(--font-size-xs)] font-medium uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("notify:list.kinds_header")}
        </h3>
        <ul
          className="grid grid-cols-2 gap-[var(--spacing-2)]"
          data-testid="notify-kind-grid"
        >
          {visibleKinds.map((kind) => {
            const count = channelsByKind.get(kind)?.length ?? 0;
            return (
              <li key={kind}>
                <button
                  type="button"
                  onClick={() => onCreate(kind)}
                  className={cn(
                    "group flex w-full flex-col items-start gap-[var(--spacing-2)]",
                    "rounded-[var(--radius-md)] border border-[var(--color-border)]",
                    "bg-[var(--color-surface)] p-[var(--spacing-3)] text-left",
                    "transition-colors duration-[var(--duration-fast)]",
                    "hover:bg-[var(--color-surface-hover)] hover:border-[var(--color-border-strong)]",
                    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
                  )}
                  data-testid={`notify-kind-${kind}`}
                  aria-label={t(`notify:kinds.${kind}.name`)}
                >
                  <div className="flex w-full items-center justify-between">
                    <ChannelIcon
                      kind={kind}
                      className="text-[var(--color-text-secondary)]"
                    />
                    <span className="text-[var(--font-size-xs)] tabular-nums text-[var(--color-text-tertiary)]">
                      {count}
                    </span>
                  </div>
                  <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                    {t(`notify:kinds.${kind}.name`)}
                  </span>
                  <span className="inline-flex items-center gap-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                    <Plus className="h-3 w-3" />
                    {t("notify:list.new")}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      </section>

      <section>
        <h3 className="mb-[var(--spacing-3)] text-[var(--font-size-xs)] font-medium uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("notify:list.configured_header", { count: filteredChannels.length })}
        </h3>
        {filteredChannels.length === 0 ? (
          <p className="rounded-[var(--radius-md)] border border-dashed border-[var(--color-border)] p-[var(--spacing-4)] text-center text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("notify:list.empty")}
          </p>
        ) : (
          <ul className="flex flex-col gap-[var(--spacing-1)]">
            {filteredChannels.map((c) => (
              <li key={c.id}>
                <button
                  type="button"
                  onClick={() => onSelect(c)}
                  className={cn(
                    "flex w-full items-center gap-[var(--spacing-2)]",
                    "rounded-[var(--radius-md)] border p-[var(--spacing-2)] text-left",
                    "transition-colors duration-[var(--duration-fast)]",
                    selectedId === c.id
                      ? "border-[var(--color-primary)] bg-[var(--color-surface-hover)]"
                      : "border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-hover)]",
                  )}
                  data-testid={`notify-channel-row-${c.id}`}
                >
                  <ChannelIcon
                    kind={c.kind}
                    className="text-[var(--color-text-secondary)]"
                  />
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                      {c.name}
                    </div>
                    <div className="truncate text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                      {t(`notify:kinds.${c.kind}.name`)}
                    </div>
                  </div>
                  <span
                    className={cn(
                      "h-2 w-2 shrink-0 rounded-full",
                      c.enabled
                        ? "bg-[var(--color-success)]"
                        : "bg-[var(--color-text-disabled)]",
                    )}
                    aria-hidden="true"
                  />
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <Button
        type="button"
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() => onCreate("telegram")}
        data-testid="notify-quick-create"
      >
        <Plus className="h-4 w-4" />
        {t("notify:list.add_channel")}
      </Button>
    </div>
  );
}
