import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import { formatRelativeTime } from "@/lib/format";
import {
  useDeleteRuleSet,
  useSyncRuleSet,
  useUpdateRuleSet,
} from "@/api/rule-set";
import type { RuleSetProvider } from "@/types/api";

interface RuleSetListProps {
  items: RuleSetProvider[] | undefined;
  isLoading: boolean;
  isError: boolean;
  errorMessage?: string;
  onRetry?: () => void;
  onEdit: (rs: RuleSetProvider) => void;
  onNew: () => void;
  className?: string;
}

/**
 * DataTable for rule providers. The behavior / format columns surface the
 * Clash semantics the user picked; the URL column keeps the host visible and
 * exposes the full string on hover (native title= for brevity). last_synced_at
 * is rendered via formatRelativeTime so it stays readable for both stale and
 * fresh entries.
 */
export function RuleSetList({
  items,
  isLoading,
  isError,
  errorMessage,
  onRetry,
  onEdit,
  onNew,
  className,
}: RuleSetListProps) {
  const { t } = useTranslation(["rule-set", "common"]);

  if (isLoading) return <RuleSetTableSkeleton className={className} />;

  if (isError) {
    return (
      <div className={cn(className)}>
        <ErrorState
          message={errorMessage ?? t("rule-set:error.load_failed")}
          onRetry={onRetry}
          retryLabel={t("common:actions.retry")}
        />
      </div>
    );
  }

  if (!items || items.length === 0) {
    return (
      <div className={cn("flex flex-1 items-center justify-center", className)}>
        <EmptyState
          title={t("rule-set:list.empty_title")}
          description={t("rule-set:list.empty_description")}
          ctaLabel={t("rule-set:list.add")}
          onCta={onNew}
        />
      </div>
    );
  }

  return (
    <div
      className={cn(
        "overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]",
        className,
      )}
    >
      <table className="w-full text-[var(--font-size-sm)]" data-testid="rule-set-list">
        <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
          <tr>
            <Th>{t("rule-set:columns.name")}</Th>
            <Th className="w-24">{t("rule-set:columns.behavior")}</Th>
            <Th className="w-20">{t("rule-set:columns.format")}</Th>
            <Th>{t("rule-set:columns.url")}</Th>
            <Th className="w-40">{t("rule-set:columns.synced")}</Th>
            <Th className="w-20 text-center">{t("rule-set:columns.enabled")}</Th>
            <Th className="w-12 text-right">
              <span className="sr-only">{t("rule-set:columns.actions")}</span>
            </Th>
          </tr>
        </thead>
        <tbody>
          {items.map((rs) => (
            <RuleSetRow key={rs.id} rs={rs} onEdit={onEdit} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

interface RuleSetRowProps {
  rs: RuleSetProvider;
  onEdit: (rs: RuleSetProvider) => void;
}

function RuleSetRow({ rs, onEdit }: RuleSetRowProps) {
  const { t } = useTranslation(["rule-set", "common"]);
  const { handle: handleError } = useApiError();
  const updateMutation = useUpdateRuleSet();
  const deleteMutation = useDeleteRuleSet();
  const syncMutation = useSyncRuleSet();

  const onToggle = async (e: React.ChangeEvent<HTMLInputElement>) => {
    try {
      await updateMutation.mutateAsync({
        id: rs.id,
        payload: { enabled: e.target.checked },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const onSync = async () => {
    try {
      await syncMutation.mutateAsync(rs.id);
      toast.success(t("rule-set:toast.sync_ok"));
    } catch (err) {
      handleError(err);
    }
  };

  const onDelete = async () => {
    if (!confirm(t("rule-set:delete.confirm", { name: rs.name }))) return;
    try {
      await deleteMutation.mutateAsync(rs.id);
      toast.success(t("rule-set:toast.delete_ok"));
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <tr
      data-testid={`rule-set-row-${rs.id}`}
      className={cn(
        "border-b border-[var(--color-border)] last:border-0",
        "hover:bg-[var(--color-surface-hover)] transition-colors duration-[var(--duration-fast)]",
      )}
    >
      <Td>
        <button
          type="button"
          onClick={() => onEdit(rs)}
          className={cn(
            "text-left font-medium hover:text-[var(--color-primary)]",
            "transition-colors duration-[var(--duration-fast)]",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] rounded-[var(--radius-sm)]",
            rs.enabled
              ? "text-[var(--color-text-primary)]"
              : "text-[var(--color-text-tertiary)]",
          )}
        >
          {rs.name}
        </button>
      </Td>
      <Td>
        <Badge variant="outline">{rs.behavior}</Badge>
      </Td>
      <Td>
        <Badge variant="secondary">{rs.format}</Badge>
      </Td>
      <Td>
        <span
          className="block max-w-[18rem] truncate text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]"
          title={rs.url}
        >
          {rs.url}
        </span>
      </Td>
      <Td>
        {rs.last_synced_at ? (
          <span className="flex items-center gap-2">
            <SyncStatusDot status={rs.last_sync_status} />
            <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {formatRelativeTime(rs.last_synced_at * 1000)}
            </span>
          </span>
        ) : (
          <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("rule-set:list.never_synced")}
          </span>
        )}
      </Td>
      <Td className="text-center">
        <input
          type="checkbox"
          checked={rs.enabled}
          onChange={onToggle}
          className="h-4 w-4 rounded border-[var(--color-border-strong)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]"
          aria-label={t("rule-set:columns.enabled")}
        />
      </Td>
      <Td className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("rule-set:columns.actions")}
            >
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => onEdit(rs)}>
              <Pencil className="mr-2 h-3.5 w-3.5" />
              {t("rule-set:actions.edit")}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => void onSync()}>
              <RefreshCw className="mr-2 h-3.5 w-3.5" />
              {t("rule-set:actions.sync_now")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => void onDelete()}
              className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
            >
              <Trash2 className="mr-2 h-3.5 w-3.5" />
              {t("rule-set:actions.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </Td>
    </tr>
  );
}

function SyncStatusDot({ status }: { status?: string }) {
  const color =
    status === "ok"
      ? "bg-[var(--color-success)]"
      : status === "error"
        ? "bg-[var(--color-error)]"
        : status === "pending"
          ? "bg-[var(--color-warning)]"
          : "bg-[var(--color-text-disabled)]";
  return <span className={cn("h-2 w-2 rounded-full", color)} aria-hidden />;
}

function Th({
  children,
  className,
}: {
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <th
      className={cn(
        "px-4 py-2.5 text-left text-[var(--font-size-xs)] font-medium uppercase tracking-wide",
        className,
      )}
    >
      {children}
    </th>
  );
}

function Td({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <td
      className={cn(
        "px-4 py-3 align-middle text-[var(--color-text-primary)]",
        className,
      )}
    >
      {children}
    </td>
  );
}

function RuleSetTableSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4",
        className,
      )}
      aria-hidden
    >
      <div className="flex flex-col gap-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-4 w-10" />
          </div>
        ))}
      </div>
    </div>
  );
}
