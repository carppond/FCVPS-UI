import * as React from "react";
import { useTranslation } from "react-i18next";
import { Download, Upload, ShieldAlert } from "lucide-react";
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
import { downloadBackup, restoreBackup } from "@/api/settings";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Backup + restore tile. v1 supports only local downloads (no remote history
 * yet — that ships in T-32 as a P2 enhancement). Restore is gated behind a
 * destructive confirm dialog because it overwrites the live database.
 */
export function BackupSection() {
  const { t } = useTranslation(["settings", "common", "errors"]);
  const token = useAuthStore((s) => s.token);

  const [creating, setCreating] = React.useState(false);
  const [restoring, setRestoring] = React.useState(false);
  const [pendingFile, setPendingFile] = React.useState<File | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);

  const handleCreate = async () => {
    setCreating(true);
    try {
      const blob = await downloadBackup(token ?? undefined);
      // Synthesise a downloadable anchor — keeps the flow inside the SPA
      // without an extra round-trip to a /download endpoint.
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      const ts = new Date()
        .toISOString()
        .replace(/[-T:]/g, "")
        .slice(0, 14);
      a.download = `shiguang-backup-${ts}.tar.gz`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("errors:INTERNAL_UNKNOWN"),
      );
    } finally {
      setCreating(false);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) setPendingFile(file);
    // Reset the input so the same file can be picked twice if the first
    // restore attempt failed.
    e.target.value = "";
  };

  const confirmRestore = async () => {
    if (!pendingFile) return;
    setRestoring(true);
    try {
      await restoreBackup(pendingFile, token ?? undefined);
      toast.success(t("settings:backup.restore_success"));
      setPendingFile(null);
    } catch (err) {
      toast.error(
        t("settings:backup.restore_failed", {
          message: err instanceof Error ? err.message : "unknown",
        }),
      );
    } finally {
      setRestoring(false);
    }
  };

  return (
    <section
      aria-labelledby="backup-heading"
      className="flex flex-col gap-6 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6"
    >
      <header className="flex flex-col gap-1">
        <h2
          id="backup-heading"
          className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]"
        >
          {t("settings:backup.title")}
        </h2>
        <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("settings:backup.description")}
        </p>
      </header>

      {/* Create-backup panel. */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-3 py-2">
          <span className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            {t("settings:backup.create_hint")}
          </span>
          <Button onClick={handleCreate} disabled={creating}>
            <Download className="mr-2 h-4 w-4" />
            {creating
              ? t("settings:backup.create_pending")
              : t("settings:backup.create_button")}
          </Button>
        </div>
      </div>

      {/* Restore panel. */}
      <div className="flex flex-col gap-2">
        <h3 className="text-[var(--font-size-base)] font-medium text-[var(--color-text-primary)]">
          {t("settings:backup.restore_title")}
        </h3>
        <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("settings:backup.restore_description")}
        </p>
        <div className="flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-error)] bg-[var(--color-error-bg)] p-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
          <div className="flex items-center gap-2 text-[var(--color-error)]">
            <ShieldAlert className="h-4 w-4" />
            <span className="font-medium">
              {t("settings:backup.restore_warning")}
            </span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <input
              ref={fileInputRef}
              type="file"
              accept=".tar.gz,.gz,application/gzip"
              onChange={handleFileSelect}
              className="hidden"
            />
            <span className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
              {pendingFile?.name ?? "—"}
            </span>
            <Button
              variant="destructive"
              onClick={() => fileInputRef.current?.click()}
              disabled={restoring}
            >
              <Upload className="mr-2 h-4 w-4" />
              {t("settings:backup.restore_button")}
            </Button>
          </div>
        </div>
      </div>

      <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        {t("settings:backup.history_empty")}
      </p>

      {/* Confirm dialog */}
      <Dialog
        open={pendingFile !== null}
        onOpenChange={(o) => !o && !restoring && setPendingFile(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-[var(--color-error)]">
              <ShieldAlert className="h-5 w-5" />
              {t("settings:backup.restore_confirm_title")}
            </DialogTitle>
            <DialogDescription>
              {t("settings:backup.restore_confirm_description")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setPendingFile(null)}
              disabled={restoring}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmRestore}
              disabled={restoring}
            >
              {restoring
                ? t("settings:backup.restore_pending")
                : t("settings:backup.restore_confirm_submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
