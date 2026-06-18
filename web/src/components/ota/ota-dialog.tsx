import * as React from "react";
import { useTranslation } from "react-i18next";
import { AlertTriangle, CheckCircle2, XCircle } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { prefixedPath } from "@/lib/silent-prefix";
import { useApplyOta } from "@/api/ota";
import type { OTAReleaseInfo } from "@/types/api";

/**
 * SSE `ota_progress` payload as emitted by `internal/ota/service.go`.
 * Stage transitions: downloading → verifying → restarting → done | error.
 */
interface OtaProgressPayload {
  stage: "downloading" | "verifying" | "restarting" | "done" | "error";
  downloaded: number;
  total: number;
  version: string;
  error?: string;
  ts: number;
}

type DialogStep = "confirm" | "progress" | "restarting";

interface OtaDialogProps {
  open: boolean;
  release: OTAReleaseInfo | null;
  onClose: () => void;
}

/**
 * Three-step OTA upgrade flow:
 *
 *  1. confirm  — diff (current vs latest) + changelog + warning banner.
 *  2. progress — live progress bar driven by SSE `ota_progress` events.
 *  3. restarting — 10s countdown, then auto-reload to pick up the new build.
 *
 * The SSE subscription is opened lazily (only when the dialog is open) so the
 * EventSource cost is paid only by the admin running the upgrade.
 */
export function OtaDialog({ open, release, onClose }: OtaDialogProps) {
  const [step, setStep] = React.useState<DialogStep>("confirm");
  const [progress, setProgress] = React.useState<OtaProgressPayload | null>(
    null,
  );
  const [countdown, setCountdown] = React.useState(10);

  const applyMutation = useApplyOta();

  // Reset internal state when the dialog opens; otherwise the second upgrade
  // attempt would inherit stale progress from the previous run.
  React.useEffect(() => {
    if (!open) return;
    setStep("confirm");
    setProgress(null);
    setCountdown(10);
  }, [open]);

  // SSE subscription. Disabled when the dialog is closed so we don't keep
  // the EventSource alive in the background.
  React.useEffect(() => {
    if (!open || step === "confirm") return;
    const url = prefixedPath(
      `/api/notify/stream`,
    );
    const es = new EventSource(url, { withCredentials: true });
    // Per backend contract, system events for OTA are emitted under either
    // "ota_progress" (typed) or "system" (broadcast). We listen for both so a
    // future contract refactor doesn't silently break the UI.
    const onProgress = (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data) as OtaProgressPayload;
        setProgress(data);
        if (data.stage === "restarting" || data.stage === "done") {
          setStep("restarting");
        }
      } catch {
        // Malformed payload — leave the UI as-is; the daily checker will
        // self-correct on the next event.
      }
    };
    es.addEventListener("ota_progress", onProgress);
    return () => {
      es.removeEventListener("ota_progress", onProgress);
      es.close();
    };
  }, [open, step]);

  // Countdown timer that fires reload() once it hits zero. The reload is
  // wrapped in a try/catch so test environments without window.location
  // (jsdom edge cases) don't crash.
  React.useEffect(() => {
    if (step !== "restarting") return;
    if (countdown <= 0) {
      try {
        window.location.reload();
      } catch {
        // ignore
      }
      return;
    }
    const t = setTimeout(() => setCountdown((c) => c - 1), 1_000);
    return () => clearTimeout(t);
  }, [step, countdown]);

  const handleStart = async () => {
    setStep("progress");
    try {
      await applyMutation.mutateAsync();
    } catch {
      // Mutation failure is surfaced through the SSE error event; the
      // progress view handles it.
    }
  };

  if (!release) return null;

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        // Don't allow dismissing the dialog mid-upgrade — that would leave
        // the admin with no UI to inspect the SSE event stream.
        if (!o && step === "confirm") onClose();
      }}
    >
      <DialogContent
        className="max-w-lg"
        onEscapeKeyDown={(e) => {
          if (step !== "confirm") e.preventDefault();
        }}
        onInteractOutside={(e) => {
          if (step !== "confirm") e.preventDefault();
        }}
      >
        {step === "confirm" && (
          <ConfirmStep
            release={release}
            onConfirm={handleStart}
            onCancel={onClose}
            pending={applyMutation.isPending}
          />
        )}
        {step === "progress" && (
          <ProgressStep progress={progress} version={release.latest_version} />
        )}
        {step === "restarting" && (
          <RestartingStep
            countdown={countdown}
            error={
              progress?.stage === "error" ? (progress.error ?? null) : null
            }
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

// ─── Step 1: confirmation ─────────────────────────────────────────────────────

function ConfirmStep({
  release,
  onConfirm,
  onCancel,
  pending,
}: {
  release: OTAReleaseInfo;
  onConfirm: () => void;
  onCancel: () => void;
  pending: boolean;
}) {
  const { t } = useTranslation(["auth", "common"]);
  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("auth:ota.confirm.title")}</DialogTitle>
        <DialogDescription>
          {t("auth:ota.confirm.description")}
        </DialogDescription>
      </DialogHeader>
      <div className="my-4 flex flex-col gap-3">
        <div className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-[var(--font-size-sm)]">
          <span className="text-[var(--color-text-tertiary)]">
            {t("auth:ota.confirm.current_version")}
          </span>
          <code className="mono text-[var(--color-text-primary)]">
            {release.current_version || t("common:unknown")}
          </code>
        </div>
        <div className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-[var(--font-size-sm)]">
          <span className="text-[var(--color-text-tertiary)]">
            {t("auth:ota.confirm.latest_version")}
          </span>
          <code className="mono text-[var(--color-primary)]">
            {release.latest_version}
          </code>
        </div>
        {release.changelog && (
          <div className="max-h-48 overflow-y-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3 text-[var(--font-size-sm)] text-[var(--color-text-secondary)] whitespace-pre-wrap">
            {release.changelog}
          </div>
        )}
        <div className="flex items-start gap-2 rounded-[var(--radius-md)] bg-[var(--color-warning-bg,var(--color-surface-hover))] p-3 text-[var(--font-size-sm)] text-[var(--color-warning,var(--color-text-primary))]">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{t("auth:ota.confirm.warning")}</span>
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onCancel} disabled={pending}>
          {t("common:actions.cancel")}
        </Button>
        <Button onClick={onConfirm} disabled={pending}>
          {pending
            ? t("auth:ota.confirm.starting")
            : t("auth:ota.confirm.submit")}
        </Button>
      </DialogFooter>
    </>
  );
}

// ─── Step 2: progress ────────────────────────────────────────────────────────

function ProgressStep({
  progress,
  version,
}: {
  progress: OtaProgressPayload | null;
  version: string;
}) {
  const { t } = useTranslation(["auth"]);
  const stage = progress?.stage ?? "downloading";
  const total = progress?.total ?? 0;
  const downloaded = progress?.downloaded ?? 0;
  const pctRaw =
    total > 0 ? Math.floor((downloaded / total) * 100) : undefined;
  const pct = stage === "downloading" ? pctRaw : 100;
  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {t("auth:ota.progress.title", { version })}
        </DialogTitle>
        <DialogDescription>
          {t(`auth:ota.progress.stage_${stage}`)}
        </DialogDescription>
      </DialogHeader>
      <div className="my-6 flex flex-col gap-3">
        <Progress
          value={stage === "downloading" ? pct : undefined}
          label={t(`auth:ota.progress.stage_${stage}`)}
        />
        <div className="flex justify-between text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          <span>{t(`auth:ota.progress.stage_${stage}`)}</span>
          {stage === "downloading" && total > 0 && (
            <span className="mono tabular-nums">
              {formatBytes(downloaded)} / {formatBytes(total)}
              {typeof pct === "number" ? ` (${pct}%)` : ""}
            </span>
          )}
        </div>
        {progress?.error && (
          <div className="flex items-start gap-2 rounded-[var(--radius-md)] bg-[var(--color-error-bg)] p-3 text-[var(--font-size-sm)] text-[var(--color-error)]">
            <XCircle className="mt-0.5 h-4 w-4 shrink-0" />
            <span>{progress.error}</span>
          </div>
        )}
      </div>
    </>
  );
}

// ─── Step 3: restarting countdown ────────────────────────────────────────────

function RestartingStep({
  countdown,
  error,
}: {
  countdown: number;
  error: string | null;
}) {
  const { t } = useTranslation(["auth", "common"]);
  if (error) {
    return (
      <>
        <DialogHeader>
          <DialogTitle>{t("auth:ota.error.title")}</DialogTitle>
          <DialogDescription>
            {t("auth:ota.error.description")}
          </DialogDescription>
        </DialogHeader>
        <div className="my-6 flex items-center gap-2 rounded-[var(--radius-md)] bg-[var(--color-error-bg)] p-3 text-[var(--font-size-sm)] text-[var(--color-error)]">
          <XCircle className="h-5 w-5 shrink-0" />
          <span>{error}</span>
        </div>
        <DialogFooter>
          <Button onClick={() => window.location.reload()}>
            {t("common:actions.refresh")}
          </Button>
        </DialogFooter>
      </>
    );
  }
  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("auth:ota.restarting.title")}</DialogTitle>
        <DialogDescription>
          {t("auth:ota.restarting.description", { seconds: countdown })}
        </DialogDescription>
      </DialogHeader>
      <div className="my-6 flex flex-col items-center gap-3 text-center">
        <CheckCircle2 className="h-10 w-10 text-[var(--color-success,var(--color-primary))]" />
        <div className="text-[var(--font-size-2xl)] font-semibold tabular-nums mono text-[var(--color-text-primary)]">
          {countdown}s
        </div>
      </div>
    </>
  );
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

/**
 * Compact byte formatter. We deliberately avoid `Intl.NumberFormat` here
 * because the SSE stream may emit several updates per second; a manual
 * formatter is allocation-light and produces the same MB / GB output.
 */
function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}
