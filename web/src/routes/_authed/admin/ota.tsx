import * as React from "react";
import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ExternalLink, RefreshCw, Rocket } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useOtaCheck, useOtaHistory, useOtaStatus } from "@/api/ota";
import { useMeQuery } from "@/api/user";
import { OtaDialog } from "@/components/ota/ota-dialog";

export const Route = createFileRoute("/_authed/admin/ota")({
  component: AdminOtaPage,
});

/**
 * /admin/ota — single-page panel that surfaces the running binary's version,
 * the latest GitHub release and offers a one-click upgrade flow. Only
 * `role=admin` users see it; non-admins get bounced back to the dashboard.
 */
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

  // Authorisation guard. The backend already enforces admin-only on the
  // endpoints, but bouncing here avoids a confusing 403 flash on load.
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
      <div className="mx-auto flex max-w-3xl flex-col gap-6">
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }
  if (statusError || !status) {
    return (
      <ErrorState
        message={
          (statusErr as Error)?.message ?? t("errors:INTERNAL_UNKNOWN")
        }
        onRetry={() => refetchStatus()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <header>
        <h1 className="text-[var(--font-size-2xl)] font-semibold text-[var(--color-text-primary)]">
          {t("auth:ota.title")}
        </h1>
        <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("auth:ota.description")}
        </p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>{t("auth:ota.current.title")}</CardTitle>
          <CardDescription>{t("auth:ota.current.description")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3">
            <span className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("auth:ota.current.running")}
            </span>
            <code className="mono text-[var(--font-size-base)] text-[var(--color-text-primary)]">
              {status.current_version || t("common:unknown")}
            </code>
          </div>
          <div className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3">
            <span className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {t("auth:ota.current.latest")}
            </span>
            <div className="flex items-center gap-2">
              <code className="mono text-[var(--font-size-base)] text-[var(--color-text-primary)]">
                {status.latest_version || t("common:unknown")}
              </code>
              {status.has_update && (
                <Badge variant="default">
                  {t("auth:ota.current.has_update")}
                </Badge>
              )}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              onClick={handleCheck}
              disabled={checkMutation.isPending}
            >
              <RefreshCw className="mr-2 h-4 w-4" />
              {checkMutation.isPending
                ? t("auth:ota.check.checking")
                : t("auth:ota.check.submit")}
            </Button>
            {status.release_url && (
              <Button variant="ghost" asChild>
                <a
                  href={status.release_url}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <ExternalLink className="mr-2 h-4 w-4" />
                  {t("auth:ota.current.view_release")}
                </a>
              </Button>
            )}
            {status.has_update && (
              <Button onClick={() => setDialogOpen(true)} className="ml-auto">
                <Rocket className="mr-2 h-4 w-4" />
                {t("auth:ota.current.upgrade_now")}
              </Button>
            )}
          </div>
          {status.changelog && (
            <div className="mt-2 max-h-64 overflow-y-auto whitespace-pre-wrap rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
              {status.changelog}
            </div>
          )}
        </CardContent>
      </Card>

      {history && history.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t("auth:ota.history.title")}</CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="flex flex-col gap-2">
              {history.map((h, idx) => (
                <li
                  key={`${h.applied_at}-${idx}`}
                  className="flex items-center justify-between rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-[var(--font-size-sm)]"
                >
                  <div className="flex items-center gap-2">
                    <code className="mono text-[var(--color-text-primary)]">
                      {h.version}
                    </code>
                    <Badge
                      variant={
                        h.status === "success" ? "default" : "destructive"
                      }
                    >
                      {t(`auth:ota.history.status_${h.status}`)}
                    </Badge>
                  </div>
                  <time
                    className="text-[var(--color-text-tertiary)] tabular-nums"
                    dateTime={new Date(h.applied_at).toISOString()}
                  >
                    {new Date(h.applied_at).toLocaleString()}
                  </time>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}

      <OtaDialog
        open={dialogOpen}
        release={status}
        onClose={() => setDialogOpen(false)}
      />
    </div>
  );
}
