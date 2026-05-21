import * as React from "react";
import { useTranslation } from "react-i18next";
import { Copy, RefreshCw, Webhook } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import {
  useTelegramStatus,
  useRotateTelegramWebhook,
  useInstallTelegramWebhook,
} from "@/api/notify";

interface TGBotSettingsProps {
  /**
   * Base URL of the deployment used to render the public webhook URL.
   * Defaults to window.location.origin when not provided. The component
   * accepts it as a prop so unit tests can pin the value.
   */
  baseURL?: string;
}

/**
 * Telegram-bot configuration card. Surfaces the active webhook token, the
 * full webhook URL (with copy button), the per-channel chat-ID bindings, and
 * two admin actions: rotate the token + install the webhook on Telegram's
 * side.
 *
 * Authorization for the destructive actions is enforced server-side (the
 * routes are admin-gated); this component renders the buttons unconditionally
 * and lets the API surface a 403 — keeping the client code free of
 * role-coupled UI logic.
 */
export function TGBotSettings({ baseURL }: TGBotSettingsProps) {
  const { t } = useTranslation(["notify", "common"]);
  const { handle: handleError } = useApiError();
  const status = useTelegramStatus();
  const rotate = useRotateTelegramWebhook();
  const install = useInstallTelegramWebhook();
  const [installURL, setInstallURL] = React.useState("");

  const computedBase = baseURL ?? (typeof window !== "undefined" ? window.location.origin : "");
  const token = status.data?.webhook_token ?? "";
  const webhookURL = token
    ? `${computedBase}/api/notify/telegram/webhook/${token}`
    : "";

  // Seed the install input with the computed URL once the token loads.
  React.useEffect(() => {
    if (webhookURL && !installURL) {
      setInstallURL(webhookURL);
    }
  }, [webhookURL, installURL]);

  const handleCopy = React.useCallback(
    async (value: string, key: string) => {
      try {
        await navigator.clipboard.writeText(value);
        toast.success(t(`notify:telegram.actions.${key}_copied`));
      } catch {
        toast.error(t("notify:telegram.actions.copy_failed"));
      }
    },
    [t],
  );

  const handleRotate = async () => {
    try {
      await rotate.mutateAsync();
      toast.success(t("notify:telegram.actions.rotated"));
    } catch (err) {
      handleError(err);
    }
  };

  const handleInstall = async () => {
    if (!installURL) return;
    try {
      await install.mutateAsync(installURL);
      toast.success(t("notify:telegram.actions.installed"));
    } catch (err) {
      handleError(err);
    }
  };

  if (status.isLoading) {
    return (
      <div className="flex flex-col gap-[var(--spacing-4)]">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-32 w-full" />
      </div>
    );
  }
  if (status.isError) {
    return (
      <ErrorState
        message={t("notify:telegram.errors.load_failed")}
        onRetry={() => status.refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  return (
    <div className="flex flex-col gap-[var(--spacing-6)]">
      <header>
        <h2 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
          {t("notify:telegram.title")}
        </h2>
        <p className="mt-[var(--spacing-1)] text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("notify:telegram.description")}
        </p>
      </header>

      {/* Webhook URL */}
      <section className="flex flex-col gap-[var(--spacing-3)] rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
        <div className="flex items-center justify-between">
          <Label htmlFor="tg-webhook-url">
            {t("notify:telegram.webhook.label")}
          </Label>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleRotate}
            disabled={rotate.isPending}
            aria-label={t("notify:telegram.actions.rotate")}
          >
            <RefreshCw className="mr-[var(--spacing-2)] h-4 w-4" />
            {rotate.isPending
              ? t("notify:telegram.actions.rotating")
              : t("notify:telegram.actions.rotate")}
          </Button>
        </div>
        <div className="flex items-center gap-[var(--spacing-2)]">
          <Input
            id="tg-webhook-url"
            value={webhookURL}
            readOnly
            className="flex-1 font-mono text-[var(--font-size-xs)]"
          />
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => handleCopy(webhookURL, "url")}
            disabled={!webhookURL}
            aria-label={t("notify:telegram.actions.copy")}
          >
            <Copy className="h-4 w-4" />
          </Button>
        </div>
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("notify:telegram.webhook.hint")}
        </p>
      </section>

      {/* Install webhook */}
      <section className="flex flex-col gap-[var(--spacing-3)] rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
        <Label htmlFor="tg-install-url">
          {t("notify:telegram.install.label")}
        </Label>
        <div className="flex items-center gap-[var(--spacing-2)]">
          <Input
            id="tg-install-url"
            value={installURL}
            onChange={(e) => setInstallURL(e.target.value)}
            placeholder={t("notify:telegram.install.placeholder")}
            className="flex-1 font-mono text-[var(--font-size-xs)]"
          />
          <Button
            type="button"
            onClick={handleInstall}
            disabled={install.isPending || !installURL}
          >
            <Webhook className="mr-[var(--spacing-2)] h-4 w-4" />
            {install.isPending
              ? t("notify:telegram.actions.installing")
              : t("notify:telegram.actions.install")}
          </Button>
        </div>
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("notify:telegram.install.hint")}
        </p>
      </section>

      {/* Bindings */}
      <section className="flex flex-col gap-[var(--spacing-3)] rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-[var(--spacing-4)]">
        <h3 className="text-[var(--font-size-base)] font-semibold text-[var(--color-text-primary)]">
          {t("notify:telegram.bindings.title")}
        </h3>
        {status.data?.bindings.length === 0 ? (
          <EmptyState title={t("notify:telegram.bindings.empty")} />
        ) : (
          <ul className="flex flex-col gap-[var(--spacing-2)]">
            {status.data?.bindings.map((b) => (
              <li
                key={b.channel_id}
                className="flex flex-col gap-[var(--spacing-1)] rounded-[var(--radius-sm)] border border-[var(--color-border)] p-[var(--spacing-3)]"
              >
                <div className="flex items-center justify-between">
                  <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                    {b.channel_name}
                  </span>
                  <span
                    className={
                      b.enabled
                        ? "text-[var(--font-size-xs)] text-[var(--color-success)]"
                        : "text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]"
                    }
                  >
                    {b.enabled
                      ? t("notify:telegram.bindings.enabled")
                      : t("notify:telegram.bindings.disabled")}
                  </span>
                </div>
                {b.chat_ids.length === 0 ? (
                  <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                    {t("notify:telegram.bindings.no_chats")}
                  </p>
                ) : (
                  <p className="font-mono text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
                    {b.chat_ids.join(", ")}
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("notify:telegram.bindings.hint")}
        </p>
      </section>
    </div>
  );
}
