import * as React from "react";
import { useTranslation } from "react-i18next";
import { Copy, RefreshCw, ShieldAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import {
  useDisableSilentMode,
  useEnableSilentMode,
  useRotateSilentMode,
  useSilentModeStatus,
} from "@/api/settings";

/**
 * Silent-mode admin tile. Drives the new opt-in flow (T-26 follow-up):
 * - A top-level switch toggles enabled. Enabling pops a one-time dialog
 *   surfacing the full entry URL the admin must save; disabling pops a
 *   confirm because the admin is removing a security control.
 * - When enabled, a sub-section shows the (full, this admin already has
 *   the URL) entry URL with a copy button and a destructive "rotate"
 *   action that mints a fresh prefix + force-logs every user.
 *
 * The settings-handler endpoints are the single source of truth; the
 * localStorage update inside the API hooks keeps this browser in sync so
 * the admin can keep clicking around without manually copying the URL into
 * the address bar.
 */
export function SilentModeSection() {
  const { t } = useTranslation(["settings", "common", "errors"]);
  const { handle: handleError } = useApiError();

  const status = useSilentModeStatus();
  const enable = useEnableSilentMode();
  const disable = useDisableSilentMode();
  const rotate = useRotateSilentMode();

  // The "enable just succeeded — copy the URL now" dialog. Distinct from
  // the rotate dialog because the messaging is different ("first-time
  // setup" vs "you already had a URL, here's the new one").
  const [postEnableUrl, setPostEnableUrl] = React.useState<string | null>(null);
  const [postEnableSaved, setPostEnableSaved] = React.useState(false);

  // The "rotate just succeeded" dialog.
  const [rotatedUrl, setRotatedUrl] = React.useState<string | null>(null);
  const [rotatedSaved, setRotatedSaved] = React.useState(false);

  // Confirm dialogs.
  const [confirmDisableOpen, setConfirmDisableOpen] = React.useState(false);
  const [confirmRotateOpen, setConfirmRotateOpen] = React.useState(false);

  const enabled = status.data?.enabled ?? false;
  const currentUrl = status.data?.login_url ?? "";
  const currentPrefix = status.data?.prefix ?? "";
  // Surface the trailing 8 hex chars so the admin can sanity-check which
  // URL the hub thinks is live without exposing the whole secret.
  const maskedPrefix = currentPrefix
    ? `…${currentPrefix.slice(-8)}`
    : "—";

  const onToggle = async (next: boolean) => {
    if (next) {
      try {
        const res = await enable.mutateAsync();
        setPostEnableUrl(res.login_url);
        setPostEnableSaved(false);
        toast.success(t("settings:silent_mode.enable_success"));
      } catch (err) {
        handleError(err);
      }
    } else {
      setConfirmDisableOpen(true);
    }
  };

  const confirmDisable = async () => {
    setConfirmDisableOpen(false);
    try {
      await disable.mutateAsync();
      toast.success(t("settings:silent_mode.disable_success"));
    } catch (err) {
      handleError(err);
    }
  };

  const confirmRotate = async () => {
    setConfirmRotateOpen(false);
    try {
      const res = await rotate.mutateAsync();
      setRotatedUrl(res.login_url);
      setRotatedSaved(false);
      toast.success(t("settings:silent_mode.rotate_success"));
    } catch (err) {
      handleError(err);
    }
  };

  const copyUrl = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      toast.success(t("settings:silent_mode.copy_success"));
    } catch {
      toast.error(t("errors:INTERNAL_UNKNOWN"));
    }
  };

  return (
    <section
      aria-labelledby="silent-mode-heading"
      className="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6"
    >
      <header className="flex flex-col gap-1">
        <h2
          id="silent-mode-heading"
          className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]"
        >
          {t("settings:silent_mode.title")}
        </h2>
        <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("settings:silent_mode.desc")}
        </p>
      </header>

      {/* Master switch row. */}
      <div className="flex items-center justify-between gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-4 py-3">
        <div className="flex flex-col gap-1">
          <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
            {enabled
              ? t("settings:silent_mode.state_enabled", { suffix: maskedPrefix })
              : t("settings:silent_mode.state_disabled")}
          </span>
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {enabled
              ? t("settings:silent_mode.state_enabled_hint")
              : t("settings:silent_mode.state_disabled_hint")}
          </span>
        </div>
        <label className="inline-flex cursor-pointer items-center gap-2">
          <input
            type="checkbox"
            className="h-5 w-5 cursor-pointer rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
            checked={enabled}
            disabled={
              status.isLoading || enable.isPending || disable.isPending
            }
            onChange={(e) => onToggle(e.target.checked)}
            aria-label={
              enabled
                ? t("settings:silent_mode.disable")
                : t("settings:silent_mode.enable")
            }
          />
          <span className="text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
            {enabled
              ? t("settings:silent_mode.disable")
              : t("settings:silent_mode.enable")}
          </span>
        </label>
      </div>

      {/* Live entry URL + rotate, only when enabled. */}
      {enabled && currentUrl && (
        <div className="flex flex-col gap-2">
          <span className="text-[var(--font-size-xs)] uppercase tracking-wider text-[var(--color-text-tertiary)]">
            {t("settings:silent_mode.entry_url_label")}
          </span>
          <div className="flex items-center gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-3 py-2">
            <code className="flex-1 break-all font-mono text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
              {currentUrl}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => copyUrl(currentUrl)}
            >
              <Copy className="mr-1 h-4 w-4" />
              {t("settings:silent_mode.copy_login_url")}
            </Button>
          </div>
          <div className="flex justify-end">
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setConfirmRotateOpen(true)}
              disabled={rotate.isPending}
            >
              <RefreshCw className="mr-2 h-4 w-4" />
              {rotate.isPending
                ? t("settings:silent_mode.rotate_pending")
                : t("settings:silent_mode.rotate")}
            </Button>
          </div>
        </div>
      )}

      {/* Confirm disable. */}
      <Dialog open={confirmDisableOpen} onOpenChange={setConfirmDisableOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-[var(--color-warning)]">
              <ShieldAlert className="h-5 w-5" />
              {t("settings:silent_mode.disable")}
            </DialogTitle>
            <DialogDescription>
              {t("settings:silent_mode.disable_confirm")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setConfirmDisableOpen(false)}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button variant="destructive" onClick={confirmDisable}>
              {t("settings:silent_mode.disable")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Confirm rotate. */}
      <Dialog open={confirmRotateOpen} onOpenChange={setConfirmRotateOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-[var(--color-error)]">
              <ShieldAlert className="h-5 w-5" />
              {t("settings:silent_mode.rotate_confirm_title")}
            </DialogTitle>
            <DialogDescription>
              {t("settings:silent_mode.rotate_confirm_description")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setConfirmRotateOpen(false)}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button variant="destructive" onClick={confirmRotate}>
              {t("settings:silent_mode.rotate_confirm_submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Post-enable: surface the new URL ONCE. */}
      <Dialog open={postEnableUrl !== null}>
        <DialogContent
          className="max-w-lg"
          onEscapeKeyDown={(e) => e.preventDefault()}
          onInteractOutside={(e) => e.preventDefault()}
        >
          <DialogHeader>
            <DialogTitle>
              {t("settings:silent_mode.enable_dialog.title")}
            </DialogTitle>
            <DialogDescription>
              {t("settings:silent_mode.enable_dialog.body")}
            </DialogDescription>
          </DialogHeader>

          <div className="my-4 flex items-center gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-3">
            <code className="flex-1 break-all font-mono text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
              {postEnableUrl ?? ""}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => postEnableUrl && copyUrl(postEnableUrl)}
            >
              <Copy className="mr-1 h-4 w-4" />
              {t("settings:silent_mode.copy_login_url")}
            </Button>
          </div>

          <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
            <input
              type="checkbox"
              className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
              checked={postEnableSaved}
              onChange={(e) => setPostEnableSaved(e.target.checked)}
            />
            {t("common:actions.confirm")}
          </label>

          <DialogFooter className="mt-4">
            <Button
              onClick={() => setPostEnableUrl(null)}
              disabled={!postEnableSaved}
            >
              {t("common:actions.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Post-rotate: surface the rotated URL ONCE. */}
      <Dialog open={rotatedUrl !== null}>
        <DialogContent
          className="max-w-lg"
          onEscapeKeyDown={(e) => e.preventDefault()}
          onInteractOutside={(e) => e.preventDefault()}
        >
          <DialogHeader>
            <DialogTitle>
              {t("settings:silent_mode.login_url_label")}
            </DialogTitle>
            <DialogDescription>
              {t("settings:silent_mode.url_only_shown_once")}
            </DialogDescription>
          </DialogHeader>

          <div className="my-4 flex items-center gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-3">
            <code className="flex-1 break-all font-mono text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
              {rotatedUrl ?? ""}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => rotatedUrl && copyUrl(rotatedUrl)}
            >
              <Copy className="mr-1 h-4 w-4" />
              {t("settings:silent_mode.copy_login_url")}
            </Button>
          </div>

          <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
            <input
              type="checkbox"
              className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
              checked={rotatedSaved}
              onChange={(e) => setRotatedSaved(e.target.checked)}
            />
            {t("common:actions.confirm")}
          </label>

          <DialogFooter className="mt-4">
            <Button
              onClick={() => setRotatedUrl(null)}
              disabled={!rotatedSaved}
            >
              {t("common:actions.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
