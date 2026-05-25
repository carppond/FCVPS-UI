import * as React from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { MoreHorizontal } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/cn";
import type { NodeWithLatency } from "@/types/api";

// ---------------------------------------------------------------------------
// Latency color helpers — matches .lf / .lm / .ls / .ln from preview
// ---------------------------------------------------------------------------

function latencyClass(node: NodeWithLatency): string {
  if (!node.reachable || node.latency_ms < 0)
    return "bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)]";
  if (node.latency_ms < 100)
    return "bg-[var(--color-success-bg)] text-[var(--color-success)]";
  if (node.latency_ms <= 300)
    return "bg-[var(--color-warning-bg)] text-[var(--color-warning)]";
  return "bg-[var(--color-error-bg)] text-[var(--color-error)]";
}

function latencyText(
  node: NodeWithLatency,
  t: (key: string) => string,
): string {
  if (!node.reachable || node.latency_ms < 0) return t("node:latency.unreachable");
  if (node.latency_ms === 0) return t("node:latency.untested");
  return `${node.latency_ms} ms`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface NodeCardProps {
  node: NodeWithLatency;
  onCopyURI: (node: NodeWithLatency) => void;
  onDelete: (node: NodeWithLatency) => void;
}

/**
 * Single node card — `.nc1` style from the V1 preview.
 *
 * Layout:
 *  - Left 3px status colour bar (green=online, grey=offline)
 *  - Top row: dot(7px) + name(13px bold) + protocol badge(9px mono)
 *  - Mid row: server:port (10.5px mono grey)
 *  - Bottom row: tag chips + latency badge
 *  - Hover: -2px translateY + shadow
 */
export const NodeCard = React.memo(function NodeCard({
  node,
  onCopyURI,
  onDelete,
}: NodeCardProps) {
  const { t } = useTranslation(["node", "common"]);
  const isOnline = node.reachable;
  const displayName = node.tag || node.id.slice(0, 8);

  return (
    <div
      className={cn(
        // .nc1 base
        "group relative flex cursor-pointer flex-col gap-2 overflow-hidden rounded-[10px] border px-4 py-3.5",
        "bg-[var(--color-surface-hover)] border-[var(--color-border)]",
        "transition-all duration-150",
        "hover:border-[var(--color-border-strong)] hover:bg-[var(--color-surface-active,var(--color-surface-hover))]",
        "hover:-translate-y-0.5 hover:shadow-[0_6px_20px_rgba(0,0,0,.2)]",
      )}
    >
      {/* Left status bar */}
      <span
        className={cn(
          "absolute left-0 top-2.5 bottom-2.5 w-[3px] rounded-r-sm",
          isOnline ? "bg-[var(--color-success)]" : "bg-[var(--color-text-tertiary)]",
        )}
      />

      {/* Top row: dot + name + protocol badge + actions */}
      <div className="flex items-center justify-between">
        <div className="flex min-w-0 items-center gap-1.5">
          {/* Status dot */}
          <span
            className={cn(
              "inline-block h-[7px] w-[7px] shrink-0 rounded-full",
              isOnline
                ? "bg-[var(--color-success)] shadow-[0_0_6px_var(--color-success)]"
                : "bg-[var(--color-text-tertiary)]",
            )}
          />
          {/* Name */}
          <span
            className={cn(
              "truncate text-[13px] font-semibold leading-tight",
              isOnline
                ? "text-[var(--color-text-primary)]"
                : "text-[var(--color-text-tertiary)]",
            )}
          >
            {displayName}
          </span>
        </div>

        <div className="flex shrink-0 items-center gap-1.5">
          {/* Protocol badge */}
          <span className="rounded-[4px] bg-[var(--color-surface-active,var(--color-surface-hover))] px-1.5 py-0.5 font-mono text-[9px] font-semibold uppercase text-[var(--color-text-secondary)]">
            {node.protocol}
          </span>

          {/* Context menu — visible on hover */}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-5 w-5 opacity-0 transition-opacity group-hover:opacity-100"
                aria-label={t("node:actions.view_detail")}
                onClick={(e) => e.stopPropagation()}
              >
                <MoreHorizontal className="h-3.5 w-3.5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
              <DropdownMenuItem onSelect={() => onCopyURI(node)}>
                {t("node:actions.copy_uri")}
              </DropdownMenuItem>
              <DropdownMenuItem asChild>
                <Link to="/nodes/$nodeId" params={{ nodeId: node.id }}>
                  {t("node:actions.view_detail")}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={() => onDelete(node)}
                className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
              >
                {t("node:actions.delete_node")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Mid row: server address */}
      <div className="truncate pl-4 font-mono text-[10.5px] leading-tight text-[var(--color-text-tertiary)]">
        {node.server}:{node.port}
      </div>

      {/* Bottom row: tags + latency */}
      <div className="flex items-center justify-between pl-4 text-[10px]">
        <div className="flex flex-wrap gap-1">
          {node.tags.slice(0, 3).map((tag) => (
            <Badge
              key={tag}
              variant="outline"
              className="rounded-[3px] border-0 bg-[var(--color-surface-hover)] px-1.5 py-0 text-[9px] font-medium text-[var(--color-text-tertiary)]"
            >
              {tag}
            </Badge>
          ))}
        </div>
        <span
          className={cn(
            "shrink-0 rounded-[3px] px-1.5 py-0.5 text-[10px] font-semibold tabular-nums",
            latencyClass(node),
          )}
        >
          {latencyText(node, t)}
        </span>
      </div>
    </div>
  );
});
