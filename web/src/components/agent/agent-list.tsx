import * as React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Radio, MoreHorizontal } from "lucide-react";
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
import { formatRelativeTime } from "@/lib/format";
import { useAgentsQuery, type ListAgentsParams } from "@/api/agent";
import type { AgentListItem, AgentStatus } from "@/types/api";
import { AgentStatusDot } from "./agent-status-dot";
import { AgentKindBadge } from "./agent-kind-badge";

const PAGE_SIZE = 25;

interface AgentListProps {
  params: ListAgentsParams;
  /**
   * Live status overrides keyed by agent id, fed from the SSE stream so the
   * list reflects status transitions without waiting for a TanStack refetch.
   */
  statusOverrides?: Record<string, AgentStatus>;
  onDelete: (agent: AgentListItem) => void;
  onRotateToken: (agent: AgentListItem) => void;
  onSendCommand: (agent: AgentListItem) => void;
}

/**
 * Cross-account agent table with four explicit states (loading / error /
 * empty / data) per the editorial cheatsheet. Each row exposes the same
 * action set as the detail page (rotate / command / delete) so admins can
 * operate without leaving the index.
 */
export function AgentList({
  params,
  statusOverrides = {},
  onDelete,
  onRotateToken,
  onSendCommand,
}: AgentListProps) {
  const { t } = useTranslation(["agent", "common"]);
  const [page, setPage] = React.useState(1);

  const { data, isLoading, isError, error, refetch } = useAgentsQuery({
    ...params,
    page,
    pageSize: PAGE_SIZE,
  });

  if (isLoading) return <AgentTableSkeleton />;
  if (isError) {
    const errMsg = error instanceof Error ? error.message : String(error ?? "");
    return (
      <ErrorState
        message={t("agent:error.load_failed") + (errMsg ? ` (${errMsg})` : "")}
        onRetry={() => refetch()}
        retryLabel={t("common:actions.retry")}
      />
    );
  }
  // Sort online → degraded → offline so the operator sees live agents first.
  // Within each status bucket the API ordering (last_seen desc) is preserved.
  const STATUS_RANK: Record<AgentStatus, number> = {
    online: 0,
    degraded: 1,
    offline: 2,
  };
  const items = [...(data?.items ?? [])].sort((a, b) => {
    const sa = statusOverrides[a.id] ?? a.status;
    const sb = statusOverrides[b.id] ?? b.status;
    return (STATUS_RANK[sa] ?? 99) - (STATUS_RANK[sb] ?? 99);
  });
  if (items.length === 0) {
    return (
      <EmptyState
        icon={<Radio />}
        title={t("agent:empty.title")}
        description={t("agent:empty.description")}
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
              <Th>{t("agent:columns.name")}</Th>
              <Th>{t("agent:columns.kind")}</Th>
              <Th>{t("agent:columns.status")}</Th>
              <Th>{t("agent:columns.version")}</Th>
              <Th>{t("agent:columns.os_arch")}</Th>
              <Th>{t("agent:columns.last_seen")}</Th>
              <Th className="w-12 text-right">{t("agent:columns.actions")}</Th>
            </tr>
          </thead>
          <tbody>
            {items.map((agent) => (
              <AgentRow
                key={agent.id}
                agent={{
                  ...agent,
                  status: statusOverrides[agent.id] ?? agent.status,
                }}
                onDelete={onDelete}
                onRotateToken={onRotateToken}
                onSendCommand={onSendCommand}
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

interface AgentRowProps {
  agent: AgentListItem;
  onDelete: (agent: AgentListItem) => void;
  onRotateToken: (agent: AgentListItem) => void;
  onSendCommand: (agent: AgentListItem) => void;
}

function AgentRow({
  agent,
  onDelete,
  onRotateToken,
  onSendCommand,
}: AgentRowProps) {
  const { t } = useTranslation("agent");
  const osArch =
    agent.os && agent.arch
      ? `${agent.os} / ${agent.arch}`
      : (agent.os ?? agent.arch ?? "—");
  return (
    <tr
      className={cn(
        "border-b border-[var(--color-border)] last:border-0",
        "hover:bg-[var(--color-surface-hover)] transition-colors duration-[var(--duration-fast)]",
      )}
    >
      <Td>
        <Link
          to="/agents/$agentId"
          params={{ agentId: agent.id }}
          className="font-medium text-[var(--color-text-primary)] hover:text-[var(--color-primary)]"
        >
          {agent.name}
        </Link>
      </Td>
      <Td>
        <AgentKindBadge kind={agent.kind} />
      </Td>
      <Td>
        <AgentStatusDot status={agent.status} withLabel />
      </Td>
      <Td className="font-mono text-[var(--font-size-xs)] text-[var(--color-text-secondary)] tabular-nums">
        {agent.version || "—"}
      </Td>
      <Td className="text-[var(--color-text-secondary)]">{osArch}</Td>
      <Td className="text-[var(--color-text-secondary)] tabular-nums">
        {agent.last_seen_at ? formatRelativeTime(agent.last_seen_at) : "—"}
      </Td>
      <Td className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("actions.view_detail")}
            >
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem asChild>
              <Link to="/agents/$agentId" params={{ agentId: agent.id }}>
                {t("actions.view_detail")}
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => onRotateToken(agent)}>
              {t("actions.rotate_token")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() => onSendCommand(agent)}
              disabled={agent.status !== "online"}
            >
              {t("actions.send_command")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => onDelete(agent)}
              className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
            >
              {t("actions.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
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
  return (
    <div className="flex items-center justify-between gap-3 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
      <span className="tabular-nums">{total}</span>
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

function AgentTableSkeleton() {
  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
      <div className="flex flex-col gap-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-16" />
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
