import * as React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { BookOpen, MoreHorizontal, RefreshCw } from "lucide-react";
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
import { cn } from "@/lib/cn";
import { formatBytes, formatRelativeTime } from "@/lib/format";
import {
  useSubscriptionsQuery,
  type ListSubscriptionsParams,
} from "@/api/subscription";
import type { Subscription, SyncStatus } from "@/types/api";

const PAGE_SIZE = 20;

interface SubListProps {
  params: ListSubscriptionsParams;
  onSync: (sub: Subscription) => void;
  onEdit: (sub: Subscription) => void;
  onDelete: (sub: Subscription) => void;
  onCreate?: () => void;
}

/**
 * Cross-user subscription DataTable. Renders the canonical four states
 * (loading / error / empty / data) and delegates row actions to the parent.
 */
export function SubList({
  params,
  onSync,
  onEdit,
  onDelete,
  onCreate,
}: SubListProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const [page, setPage] = React.useState(1);

  const { data, isLoading, isError, error, refetch } = useSubscriptionsQuery({
    ...params,
    page,
    pageSize: PAGE_SIZE,
  });

  if (isLoading) return <SubListSkeleton />;
  if (isError) {
    const errMsg = error instanceof Error ? error.message : String(error ?? "");
    return (
      <ErrorState
        message={t("subscription:error.load_failed") + (errMsg ? ` (${errMsg})` : "")}
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }

  const items = data?.items ?? [];
  if (items.length === 0) {
    return (
      <EmptyState
        icon={<BookOpen />}
        title={t("subscription:empty.title")}
        description={t("subscription:empty.description")}
        ctaLabel={onCreate ? t("subscription:empty.cta") : undefined}
        onCta={onCreate}
      />
    );
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1;

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
        <table className="w-full text-[var(--font-size-sm)]">
          <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
            <tr>
              <Th>{t("subscription:columns.name")}</Th>
              {params.allUsers && <Th>{t("subscription:columns.owner")}</Th>}
              <Th>{t("subscription:columns.type")}</Th>
              <Th className="text-right">{t("subscription:columns.nodes")}</Th>
              <Th>{t("subscription:columns.status")}</Th>
              <Th>{t("subscription:columns.last_synced_at")}</Th>
              <Th>{t("subscription:columns.traffic")}</Th>
              <Th className="w-12 text-right">
                {t("subscription:columns.actions")}
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((sub) => (
              <SubRow
                key={sub.id}
                sub={sub}
                showOwner={!!params.allUsers}
                onSync={onSync}
                onEdit={onEdit}
                onDelete={onDelete}
              />
            ))}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
          total={data?.total ?? 0}
        />
      )}
    </div>
  );
}

interface SubRowProps {
  sub: Subscription;
  showOwner: boolean;
  onSync: (sub: Subscription) => void;
  onEdit: (sub: Subscription) => void;
  onDelete: (sub: Subscription) => void;
}

function SubRow({ sub, showOwner, onSync, onEdit, onDelete }: SubRowProps) {
  const { t } = useTranslation(["subscription", "common"]);
  return (
    <tr
      className={cn(
        "border-b border-[var(--color-border)] last:border-0",
        "hover:bg-[var(--color-surface-hover)] transition-colors duration-[var(--duration-fast)]",
      )}
    >
      <Td>
        <Link
          to="/subscriptions/$id"
          params={{ id: sub.id }}
          className="font-medium text-[var(--color-text-primary)] hover:text-[var(--color-primary)]"
        >
          {sub.name}
        </Link>
        {sub.tags && sub.tags.length > 0 && (
          <div className="mt-1 flex flex-wrap gap-1">
            {sub.tags.slice(0, 4).map((tag) => (
              <Badge
                key={tag}
                variant="outline"
                className="text-[var(--font-size-xs)]"
              >
                {tag}
              </Badge>
            ))}
          </div>
        )}
      </Td>
      {showOwner && (
        <Td className="font-mono text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {sub.user_id.slice(0, 8)}
        </Td>
      )}
      <Td>
        <Badge variant="outline">
          {t(`subscription:source_type.${sub.type}`)}
        </Badge>
      </Td>
      <Td className="text-right tabular-nums">{sub.node_count}</Td>
      <Td>
        <SyncStatusBadge status={sub.last_sync_status} />
      </Td>
      <Td className="text-[var(--color-text-secondary)] tabular-nums">
        {sub.last_synced_at
          ? formatRelativeTime(sub.last_synced_at)
          : t("subscription:status.never")}
      </Td>
      <Td className="text-[var(--color-text-secondary)] tabular-nums">
        {formatTraffic(sub.traffic_used, sub.traffic_total, t)}
      </Td>
      <Td className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={t("common:aria.actions")}>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => onSync(sub)}>
              <RefreshCw className="mr-2 h-4 w-4" />
              {t("subscription:actions.sync")}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => onEdit(sub)}>
              {t("subscription:actions.edit")}
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link to="/subscriptions/$id" params={{ id: sub.id }}>
                {t("subscription:detail.tabs.share")}
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => onDelete(sub)}
              className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
            >
              {t("subscription:actions.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </Td>
    </tr>
  );
}

function formatTraffic(
  used: number | undefined,
  total: number | undefined,
  t: (key: string) => string,
): string {
  const u = used ?? 0;
  if (!total || total <= 0) {
    return `${formatBytes(u)} / ${t("subscription:detail.metadata.traffic_unlimited")}`;
  }
  return `${formatBytes(u)} / ${formatBytes(total)}`;
}

export function SyncStatusBadge({ status }: { status?: SyncStatus }) {
  const { t } = useTranslation("subscription");
  if (!status) {
    return (
      <Badge variant="secondary">{t("subscription:status.never")}</Badge>
    );
  }
  const variant: Record<SyncStatus, "default" | "secondary" | "destructive" | "outline"> = {
    ok: "outline",
    pending: "secondary",
    error: "destructive",
  };
  const cls: Record<SyncStatus, string> = {
    ok: "bg-[var(--color-success-bg)] text-[var(--color-success)]",
    pending: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
    error: "",
  };
  return (
    <Badge variant={variant[status]} className={cls[status]}>
      {t(`subscription:status.${status}`)}
    </Badge>
  );
}

function Pagination({
  page,
  totalPages,
  onPageChange,
  total,
}: {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  total: number;
}) {
  return (
    <div className="flex items-center justify-between gap-3 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
      <span className="tabular-nums">total {total}</span>
      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="sm"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          ‹
        </Button>
        <span className="px-2 tabular-nums text-[var(--color-text-primary)]">
          {page} / {totalPages}
        </span>
        <Button
          variant="ghost"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          ›
        </Button>
      </div>
    </div>
  );
}

function SubListSkeleton() {
  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
      <div className="flex flex-col gap-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3">
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-12" />
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-4 w-24" />
            <Skeleton className="h-4 w-24" />
          </div>
        ))}
      </div>
    </div>
  );
}

function Th({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <th
      className={cn(
        "px-4 py-2.5 text-center text-[var(--font-size-xs)] font-medium uppercase tracking-wide",
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
        "px-4 py-3 text-center align-middle text-[var(--color-text-primary)]",
        className,
      )}
    >
      {children}
    </td>
  );
}
