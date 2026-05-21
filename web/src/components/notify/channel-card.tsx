import { useTranslation } from "react-i18next";
import { Edit, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ChannelIcon } from "@/components/notify/channel-icon";
import { ChannelTestButton } from "@/components/notify/channel-test-button";
import { formatDate } from "@/lib/format";
import type { NotificationChannel } from "@/types/api";

interface ChannelCardProps {
  channel: NotificationChannel;
  onEdit: (channel: NotificationChannel) => void;
  onDelete: (channel: NotificationChannel) => void;
  onToggleEnabled: (channel: NotificationChannel, next: boolean) => void;
  /**
   * Reflects whether the most recent test send succeeded.
   *   - "untested" : never tested in this session
   *   - "ok"       : last test responded ok
   *   - "failed"   : last test failed (error tooltip lives on the button)
   */
  status: "untested" | "ok" | "failed";
}

/**
 * Detailed card for a single configured channel — shown in the middle column
 * of the notifications page. Surfaces:
 *   - kind icon + name + status badge
 *   - enable/disable toggle
 *   - test / edit / delete actions
 *   - last update timestamp
 *
 * Layout is intentionally non-clickable as a whole — the row buttons are the
 * only click targets so we don't accidentally swallow text-selection.
 */
export function ChannelCard({
  channel,
  onEdit,
  onDelete,
  onToggleEnabled,
  status,
}: ChannelCardProps) {
  const { t } = useTranslation(["notify", "common"]);

  const badge =
    status === "ok"
      ? { variant: "default" as const, label: t("notify:card.status.ok") }
      : status === "failed"
        ? {
            variant: "destructive" as const,
            label: t("notify:card.status.failed"),
          }
        : {
            variant: "secondary" as const,
            label: t("notify:card.status.untested"),
          };

  return (
    <article
      className="flex flex-col gap-[var(--spacing-4)] rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]"
      data-testid={`notify-channel-card-${channel.id}`}
    >
      <header className="flex items-start justify-between gap-[var(--spacing-3)]">
        <div className="flex items-start gap-[var(--spacing-3)]">
          <div className="rounded-[var(--radius-md)] bg-[var(--color-bg-elevated)] p-[var(--spacing-2)]">
            <ChannelIcon
              kind={channel.kind}
              className="text-[var(--color-primary)]"
            />
          </div>
          <div>
            <h3 className="text-[var(--font-size-base)] font-semibold text-[var(--color-text-primary)]">
              {channel.name}
            </h3>
            <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t(`notify:kinds.${channel.kind}.name`)} ·{" "}
              {t("notify:card.updated_at", {
                time: formatDate(channel.updated_at),
              })}
            </p>
          </div>
        </div>
        <Badge variant={badge.variant}>{badge.label}</Badge>
      </header>

      <div className="flex items-center justify-between gap-[var(--spacing-3)]">
        <label className="flex items-center gap-[var(--spacing-2)] text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
          <input
            type="checkbox"
            checked={channel.enabled}
            onChange={(e) => onToggleEnabled(channel, e.target.checked)}
            className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
            data-testid={`notify-channel-toggle-${channel.id}`}
          />
          {channel.enabled
            ? t("notify:card.enabled")
            : t("notify:card.disabled")}
        </label>

        <div className="flex items-center gap-[var(--spacing-2)]">
          <ChannelTestButton channelId={channel.id} />
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => onEdit(channel)}
            aria-label={t("common:actions.edit")}
          >
            <Edit className="h-4 w-4" />
            {t("common:actions.edit")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onDelete(channel)}
            aria-label={t("common:actions.delete")}
            className="text-[var(--color-error)] hover:bg-[var(--color-error-bg)]"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {channel.event_types.length > 0 && (
        <div className="flex flex-wrap gap-[var(--spacing-1)] border-t border-[var(--color-border)] pt-[var(--spacing-3)]">
          {channel.event_types.map((evt) => (
            <Badge key={evt} variant="outline">
              {t(`notify:events.${evt}.name`)}
            </Badge>
          ))}
        </div>
      )}
    </article>
  );
}
