import * as React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Boxes, MoreHorizontal } from "lucide-react";
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
import { useNodesQuery, type ListNodesParams } from "@/api/node";
import { ProtocolBadge } from "./protocol-badge";
import { LatencyBadge } from "./latency-badge";
import { TCPingButton } from "./tcping-button";
import type { NodeWithLatency } from "@/types/api";

const PAGE_SIZE = 25;

interface NodeTableProps {
  params: ListNodesParams;
  selected: string[];
  onToggleSelect: (id: string, next: boolean) => void;
  onToggleSelectAll: (ids: string[], next: boolean) => void;
  onCopyURI: (node: NodeWithLatency) => void;
  onDelete: (node: NodeWithLatency) => void;
}

/**
 * Cross-subscription node table.
 *
 * Renders four states explicitly (loading / error / empty / data) per
 * _dev-cheatsheet.md §编码硬约束. The header checkbox toggles every visible
 * row at once; per-row checkboxes drive the parent's batch-tcping selection.
 */
export function NodeTable({
  params,
  selected,
  onToggleSelect,
  onToggleSelectAll,
  onCopyURI,
  onDelete,
}: NodeTableProps) {
  const { t } = useTranslation(["node", "common"]);
  const [page, setPage] = React.useState(1);

  const { data, isLoading, isError, error, refetch } = useNodesQuery({
    ...params,
    page,
    pageSize: PAGE_SIZE,
  });

  if (isLoading) return <NodeTableSkeleton />;
  if (isError) {
    const errMsg = error instanceof Error ? error.message : String(error ?? "");
    return (
      <ErrorState
        message={t("node:error.load_failed") + (errMsg ? ` (${errMsg})` : "")}
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }
  const items = data?.items ?? [];
  if (items.length === 0) {
    return (
      <EmptyState
        icon={<Boxes />}
        title={t("node:empty.title")}
        description={t("node:empty.description")}
      />
    );
  }

  const allSelected = items.length > 0 && items.every((n) => selected.includes(n.id));
  const totalPages = data ? Math.max(1, Math.ceil(data.total / PAGE_SIZE)) : 1;

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
        <table className="w-full text-[var(--font-size-sm)]">
          <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
            <tr>
              <Th className="w-10">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={(e) =>
                    onToggleSelectAll(items.map((n) => n.id), e.target.checked)
                  }
                  aria-label={t("node:actions.select_all")}
                />
              </Th>
              <Th>{t("node:columns.name")}</Th>
              <Th>{t("node:columns.protocol")}</Th>
              <Th>{t("node:columns.server")}</Th>
              <Th className="text-right">{t("node:columns.port")}</Th>
              <Th>{t("node:columns.latency")}</Th>
              <Th>{t("node:columns.tags")}</Th>
              <Th className="w-12 text-right">
                {t("node:columns.actions")}
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((node) => (
              <NodeRow
                key={node.id}
                node={node}
                selected={selected.includes(node.id)}
                onToggleSelect={onToggleSelect}
                onCopyURI={onCopyURI}
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

interface NodeRowProps {
  node: NodeWithLatency;
  selected: boolean;
  onToggleSelect: (id: string, next: boolean) => void;
  onCopyURI: (node: NodeWithLatency) => void;
  onDelete: (node: NodeWithLatency) => void;
}

function NodeRow({
  node,
  selected,
  onToggleSelect,
  onCopyURI,
  onDelete,
}: NodeRowProps) {
  const { t } = useTranslation("node");
  return (
    <tr
      className={cn(
        "border-b border-[var(--color-border)] last:border-0",
        "hover:bg-[var(--color-surface-hover)] transition-colors duration-[var(--duration-fast)]",
      )}
    >
      <Td>
        <input
          type="checkbox"
          checked={selected}
          onChange={(e) => onToggleSelect(node.id, e.target.checked)}
          aria-label={node.tag || node.id}
        />
      </Td>
      <Td>
        <Link
          to="/nodes/$nodeId"
          params={{ nodeId: node.id }}
          className="font-medium text-[var(--color-text-primary)] hover:text-[var(--color-primary)]"
        >
          {node.tag || node.id.slice(0, 8)}
        </Link>
      </Td>
      <Td>
        <ProtocolBadge protocol={node.protocol} />
      </Td>
      <Td className="font-mono text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
        {node.server}
      </Td>
      <Td className="text-right tabular-nums">{node.port}</Td>
      <Td>
        <LatencyBadge latencyMs={node.reachable ? node.latency_ms : -1} />
      </Td>
      <Td>
        <div className="flex flex-wrap gap-1">
          {node.tags.slice(0, 3).map((tag) => (
            <Badge key={tag} variant="outline" className="text-[var(--font-size-xs)]">
              {tag}
            </Badge>
          ))}
        </div>
      </Td>
      <Td className="text-right">
        <div className="flex items-center justify-end gap-2">
          <TCPingButton nodeId={node.id} />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={t("actions.view_detail")}>
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => onCopyURI(node)}>
                {t("actions.copy_uri")}
              </DropdownMenuItem>
              <DropdownMenuItem asChild>
                <Link to="/nodes/$nodeId" params={{ nodeId: node.id }}>
                  {t("actions.view_detail")}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={() => onDelete(node)}
                className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
              >
                {t("actions.delete_node")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </Td>
    </tr>
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
  const { t } = useTranslation("common");
  return (
    <div className="flex items-center justify-between gap-3 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
      <span className="tabular-nums">
        {t("common:pagination.total", { count: total })}
      </span>
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

function NodeTableSkeleton() {
  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
        <div className="flex flex-col gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex items-center gap-3">
              <Skeleton className="h-4 w-4" />
              <Skeleton className="h-4 w-32" />
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-4 w-12" />
              <Skeleton className="h-4 w-20" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function Th({ children, className }: { children: React.ReactNode; className?: string }) {
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

function Td({ children, className }: { children: React.ReactNode; className?: string }) {
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
