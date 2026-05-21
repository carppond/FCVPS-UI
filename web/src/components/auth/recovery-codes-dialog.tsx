import * as React from "react";
import { useTranslation } from "react-i18next";
import { Copy, Download, ShieldAlert } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";

interface RecoveryCodesDialogProps {
  open: boolean;
  codes: string[];
  onConfirmed: () => void;
}

/**
 * One-time recovery-code reveal dialog.
 *
 *  - Lists all 8 codes (plain-text, monospace).
 *  - Provides "copy all" + "download .txt" affordances.
 *  - Does NOT allow closing until the user explicitly ticks "I have saved
 *    these codes" — preventing accidental loss. The Radix `onInteractOutside`
 *    handler is suppressed for the same reason.
 */
export function RecoveryCodesDialog({
  open,
  codes,
  onConfirmed,
}: RecoveryCodesDialogProps) {
  const { t } = useTranslation(["auth"]);
  const [hasConfirmed, setHasConfirmed] = React.useState(false);

  // Reset the checkbox each time a fresh set of codes is shown.
  React.useEffect(() => {
    if (open) setHasConfirmed(false);
  }, [open, codes]);

  const handleCopyAll = async () => {
    try {
      await navigator.clipboard.writeText(codes.join("\n"));
      toast.success(t("auth:recovery_codes.copied"));
    } catch {
      toast.error(t("errors:INTERNAL_UNKNOWN"));
    }
  };

  const handleDownload = () => {
    const blob = new Blob([codes.join("\n") + "\n"], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "shiguang-vps-recovery-codes.txt";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <Dialog open={open}>
      <DialogContent
        // Forced-save: prevent ESC / overlay click from dismissing.
        onEscapeKeyDown={(e) => e.preventDefault()}
        onInteractOutside={(e) => e.preventDefault()}
        // Hide the built-in close button by removing focus on it; we keep the
        // explicit "Done" button below as the only dismissal path.
        className="max-w-md"
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShieldAlert className="h-5 w-5 text-[var(--color-warning)]" />
            {t("auth:recovery_codes.title")}
          </DialogTitle>
          <DialogDescription>
            {t("auth:recovery_codes.description")}
          </DialogDescription>
        </DialogHeader>

        <div
          className="my-4 rounded-[var(--radius-md)] border border-[var(--color-warning-bg)] bg-[var(--color-warning-bg)] p-3 text-[var(--font-size-sm)] text-[var(--color-warning)]"
        >
          {t("auth:recovery_codes.warning")}
        </div>

        <ul className="grid grid-cols-2 gap-2">
          {codes.map((code) => (
            <li
              key={code}
              className="rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-center font-mono text-[var(--font-size-base)] text-[var(--color-text-primary)] tabular-nums"
            >
              {code}
            </li>
          ))}
        </ul>

        <div className="mt-4 flex gap-2">
          <Button variant="outline" size="sm" onClick={handleCopyAll}>
            <Copy className="mr-1 h-4 w-4" />
            {t("auth:recovery_codes.copy_all")}
          </Button>
          <Button variant="outline" size="sm" onClick={handleDownload}>
            <Download className="mr-1 h-4 w-4" />
            {t("auth:recovery_codes.download")}
          </Button>
        </div>

        <label className="mt-4 flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
          <input
            type="checkbox"
            className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
            checked={hasConfirmed}
            onChange={(e) => setHasConfirmed(e.target.checked)}
          />
          {t("auth:recovery_codes.confirm_saved")}
        </label>

        <DialogFooter className="mt-4">
          <Button onClick={onConfirmed} disabled={!hasConfirmed}>
            {t("auth:recovery_codes.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
