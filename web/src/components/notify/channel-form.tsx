import * as React from "react";
import { useTranslation } from "react-i18next";
import { Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { Skeleton } from "@/components/ui/skeleton";
import { CHANNEL_KIND_ORDER } from "@/components/notify/channel-list";
import { FieldRow } from "@/components/notify/channel-form-field";
import {
  useChannelKinds,
  useCreateChannel,
  useUpdateChannel,
  type ChannelKindDescriptor,
} from "@/api/notify";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import type {
  ChannelConfig,
  ChannelKind,
  EventType,
  NotificationChannel,
} from "@/types/api";

interface ChannelFormProps {
  /**
   * `null` ⇒ "new channel" mode (uses `initialKind` for the kind picker).
   * An existing channel ⇒ "edit" mode (kind is locked).
   */
  channel: NotificationChannel | null;
  /** The kind to default the picker to when creating a new channel. */
  initialKind?: ChannelKind;
  /** Fires when the channel is created / updated. */
  onSaved?: (channel: NotificationChannel) => void;
  /** Optional cancel handler — renders a "cancel" button when supplied. */
  onCancel?: () => void;
  className?: string;
}

/**
 * Generic channel editor.
 *
 *  - Top:   kind picker (locked when editing an existing channel)
 *  - Mid:   name + enabled + event_types[] (common fields)
 *  - Below: dynamic per-kind fields rendered from /api/notify/channel-kinds
 *
 * The form intentionally avoids react-hook-form here because the dynamic
 * field schema would force us to rebuild `useForm` on every kind switch,
 * which loses focus state. Plain controlled state covers our needs and
 * keeps the type story straightforward (ConfigBag = Record<string, any>).
 */
export function ChannelForm({
  channel,
  initialKind,
  onSaved,
  onCancel,
  className,
}: ChannelFormProps) {
  const { t } = useTranslation(["notify", "common"]);
  const { handle: handleError } = useApiError();
  const kindsQ = useChannelKinds();
  const createMutation = useCreateChannel();
  const updateMutation = useUpdateChannel();

  const isEdit = channel !== null;

  const [kind, setKind] = React.useState<ChannelKind>(
    channel?.kind ?? initialKind ?? "telegram",
  );
  const [name, setName] = React.useState<string>(channel?.name ?? "");
  const [enabled, setEnabled] = React.useState<boolean>(channel?.enabled ?? true);
  const [eventTypes, setEventTypes] = React.useState<EventType[]>(
    channel?.event_types ?? [],
  );
  const [config, setConfig] = React.useState<Record<string, unknown>>(
    (channel?.config as Record<string, unknown> | undefined) ?? {},
  );
  const [errors, setErrors] = React.useState<Record<string, string>>({});

  // Look up the descriptor for the current kind. The list query is cached
  // for 10 minutes so this rarely refetches.
  const descriptor: ChannelKindDescriptor | undefined = React.useMemo(() => {
    return kindsQ.data?.find((d) => d.kind === kind);
  }, [kindsQ.data, kind]);

  // When the kind changes (only valid in "new" mode), reset the config to
  // the per-field defaults declared by the descriptor.
  React.useEffect(() => {
    if (isEdit) return;
    if (!descriptor) return;
    const next: Record<string, unknown> = {};
    for (const f of descriptor.fields) {
      if (f.default !== undefined) next[f.name] = f.default;
    }
    setConfig(next);
    setErrors({});
  }, [descriptor, isEdit]);

  const toggleEventType = React.useCallback((evt: EventType) => {
    setEventTypes((prev) =>
      prev.includes(evt) ? prev.filter((x) => x !== evt) : [...prev, evt],
    );
  }, []);

  const setField = React.useCallback((key: string, value: unknown) => {
    setConfig((prev) => ({ ...prev, [key]: value }));
    setErrors((prev) => {
      if (!(key in prev)) return prev;
      const { [key]: _omit, ...rest } = prev;
      return rest;
    });
  }, []);

  const validate = React.useCallback((): boolean => {
    if (!descriptor) return false;
    const next: Record<string, string> = {};
    if (!name.trim()) next.__name = t("notify:form.error.name_required");
    for (const f of descriptor.fields) {
      if (!f.required) continue;
      const raw = config[f.name];
      const empty =
        raw === undefined ||
        raw === null ||
        raw === "" ||
        (Array.isArray(raw) && raw.length === 0);
      if (empty) next[f.name] = t("notify:form.error.field_required");
    }
    setErrors(next);
    return Object.keys(next).length === 0;
  }, [config, descriptor, name, t]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;

    try {
      if (isEdit && channel) {
        const updated = await updateMutation.mutateAsync({
          id: channel.id,
          payload: {
            name,
            config: config as unknown as ChannelConfig,
            event_types: eventTypes,
            enabled,
          },
        });
        toast.success(t("notify:form.saved"));
        onSaved?.(updated);
      } else {
        const created = await createMutation.mutateAsync({
          kind,
          name,
          config: config as unknown as ChannelConfig,
          event_types: eventTypes,
          enabled,
        });
        toast.success(t("notify:form.created"));
        onSaved?.(created);
      }
    } catch (err) {
      handleError(err);
    }
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  if (kindsQ.isLoading) {
    return (
      <div className={cn("flex flex-col gap-[var(--spacing-3)]", className)}>
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
        <Skeleton className="h-9 w-full" />
      </div>
    );
  }

  return (
    <form
      onSubmit={handleSubmit}
      className={cn("flex flex-col gap-[var(--spacing-4)]", className)}
      noValidate
      data-testid="notify-channel-form"
    >
      {/* Kind picker */}
      <div className="flex flex-col gap-[var(--spacing-2)]">
        <Label htmlFor="ch-kind">{t("notify:form.kind_label")}</Label>
        <select
          id="ch-kind"
          value={kind}
          disabled={isEdit}
          onChange={(e) => setKind(e.target.value as ChannelKind)}
          className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)] disabled:opacity-60"
          data-testid="notify-form-kind"
        >
          {CHANNEL_KIND_ORDER.map((k) => (
            <option key={k} value={k}>
              {t(`notify:kinds.${k}.name`)}
            </option>
          ))}
        </select>
        {!isEdit && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t(`notify:kinds.${kind}.description`, "")}
          </p>
        )}
      </div>

      {/* Common fields: name + enabled */}
      <div className="flex flex-col gap-[var(--spacing-2)]">
        <Label htmlFor="ch-name">{t("notify:form.name_label")}</Label>
        <Input
          id="ch-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("notify:form.name_placeholder")}
          data-testid="notify-form-name"
        />
        {errors.__name && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
            {errors.__name}
          </p>
        )}
      </div>

      <label className="flex items-center gap-[var(--spacing-2)] text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => setEnabled(e.target.checked)}
          className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
          data-testid="notify-form-enabled"
        />
        {t("notify:form.enabled_label")}
      </label>

      {/* Dynamic per-kind fields */}
      <fieldset
        className="flex flex-col gap-[var(--spacing-3)] rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg)] p-[var(--spacing-3)]"
        data-testid="notify-form-config-fields"
      >
        <legend className="px-1 text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("notify:form.config_section")}
        </legend>
        {descriptor ? (
          descriptor.fields.length === 0 ? (
            <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("notify:form.no_config_needed")}
            </p>
          ) : (
            descriptor.fields.map((field) => (
              <FieldRow
                key={field.name}
                kind={kind}
                field={field}
                value={config[field.name]}
                error={errors[field.name]}
                onChange={(v) => setField(field.name, v)}
              />
            ))
          )
        ) : (
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("notify:form.schema_missing")}
          </p>
        )}
      </fieldset>

      {/* Event subscription quick-select */}
      <fieldset className="flex flex-col gap-[var(--spacing-2)]">
        <legend className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-secondary)]">
          {t("notify:form.events_label")}
        </legend>
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("notify:form.events_hint")}
        </p>
        <div className="flex flex-wrap gap-[var(--spacing-2)]">
          {EVENT_TYPES.map((evt) => {
            const checked = eventTypes.includes(evt);
            return (
              <label
                key={evt}
                className={cn(
                  "inline-flex items-center gap-[var(--spacing-1)] rounded-[var(--radius-sm)] border px-2 py-1 text-[var(--font-size-xs)]",
                  checked
                    ? "border-[var(--color-primary)] bg-[var(--color-surface-hover)] text-[var(--color-text-primary)]"
                    : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-hover)]",
                )}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => toggleEventType(evt)}
                  className="h-3 w-3 rounded-[var(--radius-sm)]"
                  data-testid={`notify-form-event-${evt}`}
                />
                {t(`notify:events.${evt}.name`)}
              </label>
            );
          })}
        </div>
      </fieldset>

      <div className="flex items-center justify-end gap-[var(--spacing-2)] border-t border-[var(--color-border)] pt-[var(--spacing-3)]">
        {onCancel && (
          <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
            {t("common:actions.cancel")}
          </Button>
        )}
        <Button type="submit" size="sm" disabled={isPending}>
          <Save className="h-4 w-4" />
          {isPending
            ? t("common:loading")
            : isEdit
              ? t("common:actions.save")
              : t("common:actions.create")}
        </Button>
      </div>
    </form>
  );
}

// ─── Canonical event-type list ──────────────────────────────────────────────

/**
 * The set of EventType literals the form exposes. Mirrors the union in
 * `web/src/types/api.ts` so adding a new event type fails at compile time
 * if we forget to update either side.
 */
export const EVENT_TYPES = [
  "node_offline",
  "traffic_threshold",
  "subscription_sync_failed",
  "backup_completed",
  "login_anomaly",
  "ota_available",
  "script_alert",
] as const satisfies readonly EventType[];
