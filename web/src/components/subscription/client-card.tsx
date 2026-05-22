import * as React from "react";
import { useTranslation } from "react-i18next";
import { ClipboardCopy, ExternalLink, Link2, QrCode } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useCreateShortLink } from "@/api/shortlink";
import { cn } from "@/lib/cn";
import {
  buildClientShareUrl,
  resolveDeeplink,
  type ClientDef,
} from "./client-catalog";

interface ClientCardProps {
  client: ClientDef;
  /** Share URL without target param; the card appends `&target=...` per client. */
  baseUrl: string;
  subscriptionName: string;
  /** Disabled card when the subscription has no share token yet. */
  disabled?: boolean;
}

/**
 * Single client subscription card.
 *
 *  - Header: client name + format badge + platform badge.
 *  - URL (monospace, truncated with ellipsis).
 *  - Actions: copy / QR / import (deeplink) — import only when supported.
 */
export function ClientCard({
  client,
  baseUrl,
  subscriptionName,
  disabled = false,
}: ClientCardProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const createShortLink = useCreateShortLink();
  const [qrOpen, setQrOpen] = React.useState(false);
  const [shortUrl, setShortUrl] = React.useState<string | null>(null);
  const shareUrl = React.useMemo(
    () => buildClientShareUrl(baseUrl, client.target),
    [baseUrl, client.target],
  );
  const deeplink = React.useMemo(
    () => resolveDeeplink(client, shareUrl, subscriptionName),
    [client, shareUrl, subscriptionName],
  );

  // Per-client display URL: the short URL once generated, otherwise the
  // full target-specific share URL. We keep both in state so the user can
  // see what was just generated without losing the underlying long URL.
  const displayUrl = shortUrl ?? shareUrl;

  const copy = async () => {
    if (!displayUrl || disabled) return;
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      await navigator.clipboard.writeText(displayUrl);
    }
    toast.success(t("subscription:detail.share.copy_success"));
  };

  const importNow = () => {
    if (!deeplink || disabled) return;
    // Use window.location instead of an anchor to maintain the user gesture
    // for OS-level deeplink handlers (Surge / Stash / etc.).
    window.location.href = deeplink;
  };

  const makeShortLink = async () => {
    if (!shareUrl || disabled) return;
    try {
      const link = await createShortLink.mutateAsync({ target_url: shareUrl });
      const origin =
        typeof window !== "undefined" ? window.location.origin : "";
      const code = `${link.file_code}${link.user_code}`;
      const short = `${origin}/s/${code}`;
      setShortUrl(short);
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(short);
      }
      toast.success(t("subscription:detail.share.shortlink_success"));
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-[var(--radius-lg)] border bg-[var(--color-surface)] p-4",
        "border-[var(--color-border)]",
        disabled && "opacity-50",
      )}
    >
      <header className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <h4 className="truncate text-[var(--font-size-sm)] font-semibold text-[var(--color-text-primary)]">
            {client.name}
          </h4>
          <div className="mt-1 flex items-center gap-1.5">
            <Badge variant="secondary" className="text-[10px]">
              {t(`subscription:detail.share.format.${client.format}`)}
            </Badge>
            <Badge variant="outline" className="text-[10px]">
              {t(`subscription:detail.share.platforms.${client.platform}`)}
            </Badge>
          </div>
        </div>
      </header>

      <div
        className={cn(
          "rounded-[var(--radius-md)] border px-2 py-1.5",
          shortUrl
            ? "border-[var(--color-primary)] bg-[var(--color-primary)]/10"
            : "border-[var(--color-border)] bg-[var(--color-bg-elevated)]",
        )}
      >
        <p
          className={cn(
            "truncate font-mono text-[10px]",
            shortUrl
              ? "text-[var(--color-text-primary)]"
              : "text-[var(--color-text-tertiary)]",
          )}
        >
          {displayUrl || "—"}
        </p>
      </div>

      <TooltipProvider delayDuration={150}>
        <div className="flex flex-wrap items-center gap-1.5">
          <Button
            variant="outline"
            size="sm"
            onClick={copy}
            disabled={disabled || !displayUrl}
            className="flex-1 min-w-[80px]"
          >
            <ClipboardCopy className="mr-1.5 h-3.5 w-3.5" />
            {t("common:actions.copy")}
          </Button>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                onClick={makeShortLink}
                disabled={disabled || !shareUrl || createShortLink.isPending}
                aria-label={t("subscription:detail.share.shortlink_button")}
              >
                <Link2 className="h-3.5 w-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              {t("subscription:detail.share.shortlink_tooltip")}
            </TooltipContent>
          </Tooltip>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setQrOpen(true)}
            disabled={disabled || !displayUrl}
            aria-label={t("subscription:detail.share.qr_alt")}
          >
            <QrCode className="h-3.5 w-3.5" />
          </Button>
          {deeplink && (
            <Button
              variant="default"
              size="sm"
              onClick={importNow}
              disabled={disabled}
              className="flex-1 min-w-[80px]"
            >
              <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
              {t("subscription:detail.share.deeplink")}
            </Button>
          )}
        </div>
      </TooltipProvider>

      <Dialog open={qrOpen} onOpenChange={setQrOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:detail.share.qr_title", { client: client.name })}
            </DialogTitle>
            <DialogDescription className="break-all font-mono text-[10px]">
              {displayUrl}
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center justify-center p-2">
            {displayUrl && (
              <QRCodeSVG
                value={displayUrl}
                size={224}
                level="M"
                bgColor="transparent"
              />
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
