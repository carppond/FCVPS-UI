import * as React from "react";
import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  ExternalLink,
  RefreshCw,
  Rocket,
  Package,
  GitBranch,
  Clock,
  CheckCircle2,
  AlertTriangle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useOtaCheck, useOtaHistory, useOtaStatus } from "@/api/ota";
import { useMeQuery } from "@/api/user";
import { OtaDialog } from "@/components/ota/ota-dialog";
import { cn } from "@/lib/cn";

export const Route = createFileRoute("/_authed/admin/ota")({
  component: AdminOtaPage,
});

function AdminOtaPage() {
  const { t } = useTranslation(["auth", "common", "errors"]);
  const { handle: handleError } = useApiError();

  const { data: me } = useMeQuery();
  const {
    data: status,
    isLoading: statusLoading,
    isError: statusError,
    error: statusErr,
    refetch: refetchStatus,
  } = useOtaStatus();
  const { data: history } = useOtaHistory();
  const checkMutation = useOtaCheck();

  const [dialogOpen, setDialogOpen] = React.useState(false);

  if (me && me.role !== "admin") {
    return <Navigate to="/dashboard" />;
  }

  const handleCheck = async () => {
    try {
      const fresh = await checkMutation.mutateAsync();
      toast.success(
        fresh.has_update
          ? t("auth:ota.check.found_new", { version: fresh.latest_version })
          : t("auth:ota.check.up_to_date"),
      );
    } catch (err) {
      handleError(err);
    }
  };

  if (statusLoading) {
    return (
      <div className="mx-auto flex max-w-[720px] flex-col gap-6">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-[300px] w-full rounded-2xl" />
      </div>
    );
  }
  if (statusError || !status) {
    return (
      <ErrorState
        message={(statusErr as Error)?.message ?? t("errors:INTERNAL_UNKNOWN")}
        onRetry={() => refetchStatus()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  return (
    <div className="mx-auto flex max-w-[720px] flex-col gap-6">
      <header>
        <h1 className="text-[26px] font-extrabold tracking-tight text-[var(--color-text-primary)]">
          {t("auth:ota.title")}
        </h1>
        <p className="mt-1 text-[13px] text-[var(--color-text-tertiary)]">
          {t("auth:ota.description")}
        </p>
      </header>

      {/* Version card */}
      <div
        className={cn(
          "overflow-hidden rounded-2xl border bg-[var(--color-surface)]",
          "shadow-[0_16px_48px_rgba(0,0,0,0.3)]",
          status.has_update
            ? "border-[var(--color-primary)]/20"
            : "border-[var(--color-border)]",
        )}
      >
        {/* Header */}
        <div className="flex items-center gap-3 px-7 pt-6 pb-4">
          <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-[var(--color-primary-soft)] text-[var(--color-primary)]">
            <Package className="h-5 w-5" />
          </span>
          <div>
            <h2 className="text-[16px] font-bold text-[var(--color-text-primary)]">
              {t("auth:ota.current.title")}
            </h2>
            <p className="text-[11px] text-[var(--color-text-tertiary)]">
              {t("auth:ota.current.description")}
            </p>
          </div>
        </div>

        {/* Version rows */}
        <div className="mx-7 mb-4 overflow-hidden rounded-xl border border-[var(--color-border)]">
          <div className="flex items-center justify-between bg-[var(--color-bg-elevated)] px-5 py-3.5 border-b border-[var(--color-border)]">
            <div className="flex items-center gap-2.5 text-[13px] text-[var(--color-text-tertiary)]">
              <GitBranch className="h-3.5 w-3.5" />
              {t("auth:ota.current.running")}
            </div>
            <code className="font-mono text-[14px] font-semibold text-[var(--color-text-primary)]">
              {status.current_version || t("common:unknown")}
            </code>
          </div>
          <div className="flex items-center justify-between bg-[var(--color-bg-elevated)] px-5 py-3.5">
            <div className="flex items-center gap-2.5 text-[13px] text-[var(--color-text-tertiary)]">
              <Rocket className="h-3.5 w-3.5" />
              {t("auth:ota.current.latest")}
            </div>
            <div className="flex items-center gap-2.5">
              <code className="font-mono text-[14px] font-semibold text-[var(--color-text-primary)]">
                {status.latest_version || t("common:unknown")}
              </code>
              {status.has_update && (
                <span className="rounded-md bg-[var(--color-primary-soft)] px-2 py-0.5 text-[10px] font-bold text-[var(--color-primary)]">
                  NEW
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Update available banner */}
        {status.has_update && (
          <div className="mx-7 mb-4 flex items-center gap-3 rounded-xl bg-[var(--color-primary-soft)] px-5 py-3.5">
            <AlertTriangle className="h-4 w-4 shrink-0 text-[var(--color-primary)]" />
            <span className="flex-1 text-[12px] text-[var(--color-primary)]">
              {t("auth:ota.check.found_new", { version: status.latest_version })}
            </span>
            <Button
              size="sm"
              onClick={() => setDialogOpen(true)}
              className="h-8 px-4 text-[12px]"
            >
              <Rocket className="mr-1.5 h-3.5 w-3.5" />
              {t("auth:ota.current.upgrade_now")}
            </Button>
          </div>
        )}

        {/* Changelog */}
        {status.changelog && (
          <div className="mx-7 mb-5">
            <span className="mb-2 block text-[10px] font-bold uppercase tracking-wider text-[var(--color-text-disabled)]">
              Changelog
            </span>
            <div
              className={cn(
                "max-h-48 overflow-y-auto whitespace-pre-wrap rounded-xl",
                "border border-[var(--color-border)] bg-[var(--color-bg-elevated)]",
                "p-4 text-[12px] leading-relaxed text-[var(--color-text-secondary)]",
                "[scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.1)_transparent]",
              )}
            >
              {status.changelog}
            </div>
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center gap-2.5 border-t border-[var(--color-border)] px-7 py-4">
          <Button
            variant="outline"
            size="sm"
            onClick={handleCheck}
            disabled={checkMutation.isPending}
            className="h-9"
          >
            <RefreshCw className={cn("mr-2 h-3.5 w-3.5", checkMutation.isPending && "animate-spin")} />
            {checkMutation.isPending
              ? t("auth:ota.check.checking")
              : t("auth:ota.check.submit")}
          </Button>
          {status.release_url && (
            <Button variant="ghost" size="sm" asChild className="h-9">
              <a
                href={status.release_url}
                target="_blank"
                rel="noopener noreferrer"
              >
                <ExternalLink className="mr-2 h-3.5 w-3.5" />
                {t("auth:ota.current.view_release")}
              </a>
            </Button>
          )}
        </div>
      </div>

      {/* History */}
      {history && history.length > 0 && (
        <div className="overflow-hidden rounded-2xl border border-[var(--color-border)] bg-[var(--color-surface)] shadow-[0_12px_36px_rgba(0,0,0,0.2)]">
          <div className="flex items-center gap-3 px-7 pt-5 pb-3">
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-white/[.04] text-[var(--color-text-secondary)]">
              <Clock className="h-4 w-4" />
            </span>
            <h3 className="text-[14px] font-bold text-[var(--color-text-primary)]">
              {t("auth:ota.history.title")}
            </h3>
          </div>
          <div className="px-7 pb-5">
            <div className="overflow-hidden rounded-xl border border-[var(--color-border)]">
              {history.map((h, idx) => (
                <div
                  key={`${h.applied_at}-${idx}`}
                  className={cn(
                    "flex items-center justify-between px-5 py-3 text-[13px]",
                    "bg-[var(--color-bg-elevated)] transition-colors hover:bg-white/[.02]",
                    idx < history.length - 1 && "border-b border-[var(--color-border)]",
                  )}
                >
                  <div className="flex items-center gap-3">
                    {h.status === "success" ? (
                      <CheckCircle2 className="h-4 w-4 text-[var(--color-success)]" />
                    ) : (
                      <AlertTriangle className="h-4 w-4 text-[var(--color-error)]" />
                    )}
                    <code className="font-mono font-semibold text-[var(--color-text-primary)]">
                      {h.version}
                    </code>
                    <span
                      className={cn(
                        "rounded-md px-2 py-0.5 text-[9px] font-bold uppercase",
                        h.status === "success"
                          ? "bg-[rgba(52,211,153,.1)] text-[var(--color-success)]"
                          : "bg-[rgba(248,113,113,.1)] text-[var(--color-error)]",
                      )}
                    >
                      {t(`auth:ota.history.status_${h.status}`)}
                    </span>
                  </div>
                  <time
                    className="text-[12px] tabular-nums text-[var(--color-text-tertiary)]"
                    dateTime={new Date(h.applied_at).toISOString()}
                  >
                    {new Date(h.applied_at).toLocaleString()}
                  </time>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      <OtaDialog
        open={dialogOpen}
        release={status}
        onClose={() => setDialogOpen(false)}
      />
    </div>
  );
}
