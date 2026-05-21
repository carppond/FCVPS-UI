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
import { useRotateSilentMode, useSettings } from "@/api/settings";

/**
 * Silent-mode prefix admin tile. Displays the current prefix (masked) and
 * exposes a destructive "rotate" action. After rotation succeeds the freshly
 * generated login URL is shown ONCE — closing the post-rotate dialog hides it
 * forever (the GET /settings endpoint never returns the raw prefix again).
 *
 * UX notes:
 *  - Confirm dialog explicitly calls out "all users will be force-logged-out"
 *    so admins do not click the button on a whim.
 *  - The URL panel is non-dismissible until the admin clicks "I have saved
 *    this" — mirroring the recovery-codes pattern used elsewhere.
 */
export function SilentModeSection() {
  const { t } = useTranslation(["settings", "common", "errors"]);
  const { handle: handleError } = useApiError();

  const settings = useSettings();
  const rotate = useRotateSilentMode();

  const [confirmOpen, setConfirmOpen] = React.useState(false);
  const [revealedUrl, setRevealedUrl] = React.useState<string | null>(null);
  const [hasSaved, setHasSaved] = React.useState(false);

  const masked = settings.data?.silent_mode_prefix ?? "—";

  const handleConfirmRotate = async () => {
    setConfirmOpen(false);
    try {
      const res = await rotate.mutateAsync();
      setRevealedUrl(res.login_url);
      setHasSaved(false);
      toast.success(t("settings:silent_mode.rotate_success"));
    } catch (err) {
      handleError(err);
    }
  };

  const copyUrl = async () => {
    if (!revealedUrl) return;
    try {
      await navigator.clipboard.writeText(revealedUrl);
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
          {t("settings:silent_mode.description")}
        </p>
      </header>

      <div className="flex flex-col gap-2">
        <span className="text-[var(--font-size-xs)] uppercase tracking-wider text-[var(--color-text-tertiary)]">
          {t("settings:silent_mode.current_prefix_label")}
        </span>
        <div className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-3 py-2">
          <code className="font-mono text-[var(--font-size-base)] text-[var(--color-text-primary)] tabular-nums">
            {masked}
          </code>
          <Button
            variant="destructive"
            onClick={() => setConfirmOpen(true)}
            disabled={rotate.isPending}
          >
            <RefreshCw className="mr-2 h-4 w-4" />
            {rotate.isPending
              ? t("settings:silent_mode.rotate_pending")
              : t("settings:silent_mode.rotate_button")}
          </Button>
        </div>
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("settings:silent_mode.current_prefix_hint")}
        </p>
      </div>

      {/* Confirm dialog — destructive copy + force re-login warning. */}
      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
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
              onClick={() => setConfirmOpen(false)}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button variant="destructive" onClick={handleConfirmRotate}>
              {t("settings:silent_mode.rotate_confirm_submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Post-rotate dialog — surfaces the new URL ONCE. */}
      <Dialog open={revealedUrl !== null}>
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
              {revealedUrl ?? ""}
            </code>
            <Button variant="outline" size="sm" onClick={copyUrl}>
              <Copy className="mr-1 h-4 w-4" />
              {t("settings:silent_mode.copy_login_url")}
            </Button>
          </div>

          <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
            <input
              type="checkbox"
              className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
              checked={hasSaved}
              onChange={(e) => setHasSaved(e.target.checked)}
            />
            {t("common:actions.confirm")}
          </label>

          <DialogFooter className="mt-4">
            <Button
              onClick={() => setRevealedUrl(null)}
              disabled={!hasSaved}
            >
              {t("common:actions.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
