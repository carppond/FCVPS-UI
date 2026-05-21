import * as React from "react";
import { useTranslation } from "react-i18next";
import { Activity, ChevronLeft, ChevronRight, Eye } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useAdminAuditLogs, type AuditListParams } from "@/api/audit";
import { formatDate } from "@/lib/format";
import type { AuditLog } from "@/types/api";

const PAGE_SIZE = 50;

export interface AuditTableProps {
  filter: Omit<AuditListParams, "page" | "pageSize">;
}

/**
 * AuditTable — admin view of audit_logs with pagination + payload preview.
 * Filter inputs live in the parent route; this component only consumes the
 * resolved filter values.
 */
export function AuditTable({ filter }: AuditTableProps) {
  const { t } = useTranslation(["audit", "common"]);
  const [page, setPage] = React.useState(1);
  const [previewRow, setPreviewRow] = React.useState<AuditLog | null>(null);

  // Reset to page 1 whenever the filter changes — otherwise switching from a
  // narrow filter (page 5) to a wider one shows the wrong slice.
  const filterKey = JSON.stringify(filter);
  React.useEffect(() => {
    setPage(1);
  }, [filterKey]);

  const { data, isLoading, isError, error, refetch } = useAdminAuditLogs({
    ...filter,
    page,
    pageSize: PAGE_SIZE,
  });

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
      </div>
    );
  }

  if (isError) {
    const msg = error instanceof Error ? error.message : String(error ?? "");
    return (
      <ErrorState
        message={t("audit:table.error_load") + (msg ? ` (${msg})` : "")}
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  if (items.length === 0) {
    return (
      <EmptyState
        icon={<Activity />}
        title={t("audit:table.empty_title")}
        description={t("audit:table.empty_description")}
      />
    );
  }

  return (
    <>
      <div className="overflow-hidden rounded-md border border-[var(--color-border-subtle)]">
        <table className="w-full table-auto text-left text-[var(--font-size-sm)]">
          <thead className="bg-[var(--color-bg-subtle)] text-[var(--color-text-tertiary)]">
            <tr>
              <th className="px-4 py-2 font-medium">{t("audit:table.col_time")}</th>
              <th className="px-4 py-2 font-medium">{t("audit:table.col_user")}</th>
              <th className="px-4 py-2 font-medium">{t("audit:table.col_action")}</th>
              <th className="px-4 py-2 font-medium">{t("audit:table.col_resource")}</th>
              <th className="px-4 py-2 font-medium">{t("audit:table.col_ip")}</th>
              <th className="px-4 py-2 font-medium">{t("audit:table.col_status")}</th>
              <th className="px-4 py-2 font-medium text-right">
                {t("audit:table.col_actions")}
              </th>
            </tr>
          </thead>
          <tbody>
            {items.map((row) => (
              <tr
                key={row.id}
                className="border-t border-[var(--color-border-subtle)] hover:bg-[var(--color-bg-subtle)]"
              >
                <td className="px-4 py-2 text-[var(--color-text-tertiary)]">
                  {formatDate(row.created_at)}
                </td>
                <td className="px-4 py-2 font-mono text-[var(--color-text-secondary)]">
                  {row.user_id || "-"}
                </td>
                <td className="px-4 py-2 font-medium text-[var(--color-text-primary)]">
                  {row.action}
                </td>
                <td className="px-4 py-2 text-[var(--color-text-secondary)]">
                  {row.resource_type
                    ? `${row.resource_type}${row.resource_id ? "/" + row.resource_id : ""}`
                    : "-"}
                </td>
                <td className="px-4 py-2 font-mono text-[var(--color-text-tertiary)]">
                  {row.ip || "-"}
                </td>
                <td className="px-4 py-2">
                  <Badge variant={row.success ? "default" : "destructive"}>
                    {row.success
                      ? t("audit:table.status_success")
                      : t("audit:table.status_failure")}
                  </Badge>
                </td>
                <td className="px-4 py-2 text-right">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setPreviewRow(row)}
                    title={t("audit:table.view_payload")}
                  >
                    <Eye className="h-4 w-4" />
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="mt-4 flex items-center justify-between text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        <span>
          {t("audit:table.pagination_summary", {
            page,
            totalPages,
            total,
          })}
        </span>
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            <ChevronLeft className="h-4 w-4" />
            {t("common:actions.back")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          >
            {t("audit:table.next")}
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      </div>

      <Dialog open={!!previewRow} onOpenChange={(o) => !o && setPreviewRow(null)}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("audit:table.payload_title")}</DialogTitle>
          </DialogHeader>
          <pre className="max-h-[60vh] overflow-auto rounded-md bg-[var(--color-bg-subtle)] p-4 font-mono text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
            {previewRow?.payload
              ? JSON.stringify(previewRow.payload, null, 2)
              : t("audit:table.payload_empty")}
          </pre>
        </DialogContent>
      </Dialog>
    </>
  );
}
