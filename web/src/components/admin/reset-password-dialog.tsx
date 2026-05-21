import * as React from "react";
import { useTranslation } from "react-i18next";
import { Copy } from "lucide-react";
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

interface ResetPasswordDialogProps {
  open: boolean;
  username: string;
  /** New password text. Empty string while the reset request is in flight. */
  newPassword: string;
  onClose: () => void;
}

/**
 * Force-save dialog displayed after an admin resets a user's password.
 * Same pattern as recovery-codes: must tick the confirm checkbox to close.
 */
export function ResetPasswordDialog({
  open,
  username,
  newPassword,
  onClose,
}: ResetPasswordDialogProps) {
  const { t } = useTranslation(["auth"]);
  const [hasConfirmed, setHasConfirmed] = React.useState(false);

  React.useEffect(() => {
    if (open) setHasConfirmed(false);
  }, [open, newPassword]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(newPassword);
      toast.success(t("auth:recovery_codes.copied"));
    } catch {
      toast.error(t("errors:INTERNAL_UNKNOWN"));
    }
  };

  return (
    <Dialog open={open}>
      <DialogContent
        onEscapeKeyDown={(e) => e.preventDefault()}
        onInteractOutside={(e) => e.preventDefault()}
        className="max-w-md"
      >
        <DialogHeader>
          <DialogTitle>
            {t("auth:admin_users.reset_password.title")}
          </DialogTitle>
          <DialogDescription>
            {t("auth:admin_users.reset_password.description", { username })}
          </DialogDescription>
        </DialogHeader>

        <div className="my-4 flex items-center gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-3">
          <code className="flex-1 font-mono text-[var(--font-size-base)] text-[var(--color-text-primary)] tabular-nums break-all">
            {newPassword || "…"}
          </code>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleCopy}
            disabled={!newPassword}
          >
            <Copy className="mr-1 h-4 w-4" />
            {t("auth:admin_users.reset_password.copy")}
          </Button>
        </div>

        <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
          <input
            type="checkbox"
            className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
            checked={hasConfirmed}
            onChange={(e) => setHasConfirmed(e.target.checked)}
          />
          {t("auth:admin_users.reset_password.confirm_saved")}
        </label>

        <DialogFooter className="mt-4">
          <Button onClick={onClose} disabled={!hasConfirmed}>
            {t("auth:admin_users.reset_password.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
