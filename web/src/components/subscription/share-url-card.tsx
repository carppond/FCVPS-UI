import * as React from "react";
import { useTranslation } from "react-i18next";
import { ClipboardCopy, QrCode, RotateCw } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { cn } from "@/lib/cn";
import { useApiError } from "@/hooks/use-api-error";
import { useRotateShareTokenMutation } from "@/api/subscription";

interface ShareUrlCardProps {
  subscriptionId: string;
  shareUrl: string;
  /** Whether the URL is empty (e.g. share_token missing from server). */
  available: boolean;
}

/**
 * sub-store compatible URL panel.
 *
 *  - Read-only URL field + copy button.
 *  - QR placeholder (lucide icon over the rendered URL — full QR rendering
 *    can land later via a small SVG generator; the icon clearly communicates
 *    the action).
 *  - Rotate token button gated behind a confirm dialog (irreversible).
 */
export function ShareUrlCard({
  subscriptionId,
  shareUrl,
  available,
}: ShareUrlCardProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const rotate = useRotateShareTokenMutation();
  const [confirmOpen, setConfirmOpen] = React.useState(false);
  const [showQr, setShowQr] = React.useState(false);

  const copy = async () => {
    if (!shareUrl) return;
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      await navigator.clipboard.writeText(shareUrl);
    }
    toast.success(t("subscription:detail.share.copy_success"));
  };

  const confirmRotate = async () => {
    try {
      await rotate.mutateAsync(subscriptionId);
      toast.success(t("subscription:detail.share.rotate_success"));
      setConfirmOpen(false);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
        <header className="flex items-center justify-between">
          <h3 className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
            {t("subscription:detail.share.title")}
          </h3>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowQr((v) => !v)}
            aria-label={t("subscription:detail.share.qr_alt")}
          >
            <QrCode className="h-4 w-4" />
          </Button>
        </header>
        <p className="mt-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("subscription:detail.share.url_hint")}
        </p>

        <div className="mt-3 flex items-center gap-2">
          <Input
            value={available ? shareUrl : ""}
            readOnly
            placeholder={available ? "" : "—"}
            className="font-mono text-[var(--font-size-xs)]"
          />
          <Button
            variant="outline"
            onClick={copy}
            disabled={!available}
          >
            <ClipboardCopy className="mr-2 h-4 w-4" />
            {t("common:actions.copy")}
          </Button>
        </div>

        {showQr && available && <QrPlaceholder url={shareUrl} />}
      </div>

      <div className="rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
        <h3 className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
          {t("subscription:detail.share.rotate_title")}
        </h3>
        <p className="mt-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("subscription:detail.share.rotate_confirm")}
        </p>
        <Button
          variant="outline"
          className="mt-3"
          onClick={() => setConfirmOpen(true)}
          disabled={rotate.isPending}
        >
          <RotateCw className="mr-2 h-4 w-4" />
          {t("subscription:actions.rotate_token")}
        </Button>
      </div>

      <Dialog
        open={confirmOpen}
        onOpenChange={(o) => !o && !rotate.isPending && setConfirmOpen(false)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:detail.share.rotate_title")}
            </DialogTitle>
            <DialogDescription>
              {t("subscription:detail.share.rotate_confirm")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setConfirmOpen(false)}
              disabled={rotate.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmRotate}
              disabled={rotate.isPending}
            >
              {t("subscription:actions.rotate_token")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function QrPlaceholder({ url }: { url: string }) {
  // Lightweight visual surrogate — the production app can swap this for a
  // proper QR renderer (e.g. qrcode.react) without changing the public API
  // of this component.
  return (
    <div
      className={cn(
        "mt-3 flex flex-col items-center gap-2 rounded-[var(--radius-md)]",
        "border border-dashed border-[var(--color-border-strong)]",
        "bg-[var(--color-bg-elevated)] p-4",
      )}
    >
      <QrCode className="h-32 w-32 text-[var(--color-text-secondary)]" />
      <p className="break-all text-center font-mono text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        {url}
      </p>
    </div>
  );
}
