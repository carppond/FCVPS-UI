import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  MoreHorizontal,
  RefreshCw,
  Share2,
  Pencil,
  Trash2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/cn";
import { formatBytes, formatRelativeTime } from "@/lib/format";
import type { Subscription, SyncStatus, SubType } from "@/types/api";

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface SubCardProps {
  subscription: Subscription;
  onEdit: (sub: Subscription) => void;
  onSync: (sub: Subscription) => void;
  onShare: (sub: Subscription) => void;
  onDelete: (sub: Subscription) => void;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Map sync status to the left status-bar colour. */
function statusColor(status?: SyncStatus): string {
  switch (status) {
    case "ok":
      return "bg-[var(--color-success)]";
    case "error":
      return "bg-[var(--color-error)]";
    case "pending":
      return "bg-[var(--color-warning)]";
    default:
      return "bg-[var(--color-text-disabled)]";
  }
}

/** Map sync status to the dot colour. */
function dotColor(status?: SyncStatus): string {
  switch (status) {
    case "ok":
      return "bg-[var(--color-success)]";
    case "error":
      return "bg-[var(--color-error)]";
    case "pending":
      return "bg-[var(--color-warning)]";
    default:
      return "bg-[var(--color-text-disabled)]";
  }
}

/** Badge variant for the source type. */
function sourceVariant(type: SubType): "info" | "warning" | "secondary" {
  switch (type) {
    case "url":
      return "info";
    case "upload":
      return "warning";
    case "manual":
      return "secondary";
  }
}

/** Badge variant for the sync status. */
function statusVariant(
  status?: SyncStatus,
): "success" | "destructive" | "warning" | "secondary" {
  switch (status) {
    case "ok":
      return "success";
    case "error":
      return "destructive";
    case "pending":
      return "warning";
    default:
      return "secondary";
  }
}

/** Traffic progress bar colour based on usage percentage. */
function progressColor(pct: number): string {
  if (pct >= 90) return "bg-[var(--color-error)]";
  if (pct >= 70) return "bg-[var(--color-warning)]";
  return "bg-[var(--color-info)]";
}

/** Format traffic as "X / Y GB" or "X / unlimited". */
function fmtTraffic(
  used: number | undefined,
  total: number | undefined,
  unlimitedLabel: string,
): string {
  const u = used ?? 0;
  if (!total || total <= 0) return `${formatBytes(u)} / ${unlimitedLabel}`;
  return `${formatBytes(u)} / ${formatBytes(total)}`;
}

/** Compute usage ratio 0-100. */
function usagePercent(used?: number, total?: number): number {
  const u = used ?? 0;
  if (!total || total <= 0) return 0;
  return Math.min(100, (u / total) * 100);
}

/** Format sync interval to readable period string. */
function fmtInterval(
  seconds: number,
  t: (key: string) => string,
): string {
  if (!seconds || seconds <= 0) return t("subscription:sync_interval_manual");
  const h = seconds / 3600;
  if (h >= 24) return `${Math.round(h / 24)}d`;
  if (h >= 1) return `${Math.round(h)}h`;
  return `${Math.round(seconds / 60)}m`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function SubCard({
  subscription: sub,
  onEdit,
  onSync,
  onShare,
  onDelete,
}: SubCardProps) {
  const { t } = useTranslation(["subscription", "common"]);

  const pct = usagePercent(sub.traffic_used, sub.traffic_total);
  const hasTraffic = sub.traffic_total && sub.traffic_total > 0;
  const isUrlType = sub.type === "url";

  return (
    <Link
      to="/subscriptions/$id"
      params={{ id: sub.id }}
      className={cn(
        // Card base — glass morphism
        "group relative flex cursor-pointer flex-col gap-3 overflow-hidden",
        "rounded-[var(--radius-lg)] border border-[var(--color-border)]",
        "bg-[var(--color-surface)] backdrop-blur-xl",
        "px-5 py-[18px]",
        // Hover
        "transition-all duration-150",
        "hover:border-[var(--color-border-strong)] hover:-translate-y-0.5 hover:shadow-lg",
      )}
      onClick={(e) => {
        // Prevent navigation when clicking action buttons
        if ((e.target as HTMLElement).closest("[data-action]")) {
          e.preventDefault();
        }
      }}
    >
      {/* Left 3px status bar (::before equivalent) */}
      <span
        className={cn(
          "absolute left-0 top-3 bottom-3 w-[3px] rounded-r-sm",
          statusColor(sub.last_sync_status),
        )}
        aria-hidden
      />

      {/* ── Row 1: Name + badge group ── */}
      <div className="flex items-start justify-between gap-2.5">
        <span className="truncate text-[15px] font-semibold text-[var(--color-text-primary)]">
          {sub.name}
        </span>
        <div className="flex shrink-0 items-center gap-1">
          <Badge variant={sourceVariant(sub.type)}>
            {t(`subscription:source_type.${sub.type}`)}
          </Badge>
          <Badge variant={statusVariant(sub.last_sync_status)}>
            {t(
              `subscription:status.${sub.last_sync_status ?? "never"}`,
            )}
          </Badge>
        </div>
      </div>

      {/* ── Row 2: Metrics ── */}
      <div className="flex flex-wrap gap-4 text-xs text-[var(--color-text-secondary)]">
        <span>
          <span className="mr-1">🌐</span>
          <strong className="text-[var(--color-text-primary)]">
            {sub.node_count}
          </strong>{" "}
          {t("subscription:list.card.nodes")}
        </span>
        {hasTraffic && (
          <span>
            <span className="mr-1">📊</span>
            <strong className="text-[var(--color-text-primary)]">
              {fmtTraffic(
                sub.traffic_used,
                sub.traffic_total,
                t("subscription:detail.metadata.traffic_unlimited"),
              )}
            </strong>
          </span>
        )}
      </div>

      {/* ── Row 3: Traffic progress bar ── */}
      {hasTraffic && (
        <div className="h-1 w-full overflow-hidden rounded-sm bg-[var(--color-border)]">
          <div
            className={cn("h-full rounded-sm transition-all", progressColor(pct))}
            style={{ width: `${pct}%` }}
          />
        </div>
      )}

      {/* ── Row 4: Tags ── */}
      {sub.tags && sub.tags.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {sub.tags.slice(0, 6).map((tag) => (
            <span
              key={tag}
              className="rounded-[4px] bg-[var(--color-surface-hover)] px-[7px] py-[2px] text-[10px] font-medium text-[var(--color-text-tertiary)]"
            >
              {tag}
            </span>
          ))}
        </div>
      )}

      {/* ── Row 5: Bottom — sync time + actions ── */}
      <div className="flex items-center justify-between text-[11px] text-[var(--color-text-tertiary)]">
        <span className="flex items-center gap-1.5">
          <span
            className={cn(
              "inline-block h-1.5 w-1.5 rounded-full",
              dotColor(sub.last_sync_status),
            )}
          />
          {sub.last_synced_at
            ? formatRelativeTime(sub.last_synced_at)
            : sub.type === "manual"
              ? t("subscription:list.card.manual_managed")
              : t("subscription:status.never")}
          {isUrlType && sub.sync_interval > 0 && (
            <span className="ml-0.5">
              · {t("subscription:list.card.every")}{" "}
              {fmtInterval(sub.sync_interval, t)}
            </span>
          )}
        </span>

        {/* Action buttons */}
        <div
          className="flex items-center gap-1"
          data-action
          onClick={(e) => e.stopPropagation()}
        >
          {isUrlType && (
            <button
              type="button"
              className={cn(
                "rounded-[5px] border border-[var(--color-border)] bg-transparent px-2 py-[3px]",
                "text-[11px] text-[var(--color-text-tertiary)]",
                "transition-all duration-100",
                "hover:border-[var(--color-border-strong)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)]",
              )}
              onClick={(e) => {
                e.preventDefault();
                onSync(sub);
              }}
              title={t("subscription:actions.sync")}
            >
              <RefreshCw className="h-3 w-3" />
            </button>
          )}
          <button
            type="button"
            className={cn(
              "rounded-[5px] border border-[var(--color-border)] bg-transparent px-2 py-[3px]",
              "text-[11px] text-[var(--color-text-tertiary)]",
              "transition-all duration-100",
              "hover:border-[var(--color-border-strong)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)]",
            )}
            onClick={(e) => {
              e.preventDefault();
              onShare(sub);
            }}
            title={t("subscription:detail.tabs.share")}
          >
            <Share2 className="h-3 w-3" />
          </button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={cn(
                  "rounded-[5px] border border-[var(--color-border)] bg-transparent px-2 py-[3px]",
                  "text-[11px] text-[var(--color-text-tertiary)]",
                  "transition-all duration-100",
                  "hover:border-[var(--color-border-strong)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)]",
                )}
                onClick={(e) => e.preventDefault()}
              >
                <MoreHorizontal className="h-3 w-3" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                onSelect={() => onEdit(sub)}
              >
                <Pencil className="mr-2 h-3.5 w-3.5" />
                {t("subscription:actions.edit")}
              </DropdownMenuItem>
              <DropdownMenuItem asChild>
                <Link to="/subscriptions/$id" params={{ id: sub.id }}>
                  <Share2 className="mr-2 h-3.5 w-3.5" />
                  {t("subscription:detail.tabs.share")}
                </Link>
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onSelect={() => onDelete(sub)}
                className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
              >
                <Trash2 className="mr-2 h-3.5 w-3.5" />
                {t("subscription:actions.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </Link>
  );
}
