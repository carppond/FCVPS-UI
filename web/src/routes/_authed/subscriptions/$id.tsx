import * as React from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  Activity,
  ClipboardCopy,
  Loader2,
  Pencil,
  RefreshCw,
  RotateCw,
  Trash2,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { EmptyState } from "@/components/ui/empty-state";
import { toast } from "@/components/ui/toast";
import { cn } from "@/lib/cn";
import { useApiError } from "@/hooks/use-api-error";
import {
  useDeleteSubscriptionMutation,
  useSubscriptionQuery,
  useSyncSubscriptionMutation,
  useRotateShareTokenMutation,
} from "@/api/subscription";
import { useNodesQuery, useTCPingBatchMutation } from "@/api/node";
import { SubEditForm } from "@/components/subscription/sub-edit-form";
import { SyncHistory } from "@/components/subscription/sync-history";
import { NodeMiniCard } from "@/components/subscription/node-mini-card";
import { ClientCard } from "@/components/subscription/client-card";
import { CLIENT_CATALOG } from "@/components/subscription/client-catalog";
import { prefixedPath } from "@/lib/silent-prefix";
import { formatBytes, formatDate, formatRelativeTime } from "@/lib/format";
import i18n from "@/lib/i18n";
import subZh from "@/locales/zh-CN/subscription.json";
import subEn from "@/locales/en/subscription.json";
import subJa from "@/locales/ja/subscription.json";
import subKo from "@/locales/ko/subscription.json";
import nodeZh from "@/locales/zh-CN/node.json";
import nodeEn from "@/locales/en/node.json";
import nodeJa from "@/locales/ja/node.json";
import nodeKo from "@/locales/ko/node.json";
import type { SyncStatus, SubType, NodeWithLatency } from "@/types/api";

function ensureNamespaces() {
  if (!i18n.hasResourceBundle("zh-CN", "subscription")) {
    i18n.addResourceBundle("zh-CN", "subscription", subZh, true, true);
    i18n.addResourceBundle("en", "subscription", subEn, true, true);
    i18n.addResourceBundle("ja", "subscription", subJa, true, true);
    i18n.addResourceBundle("ko", "subscription", subKo, true, true);
  }
  if (!i18n.hasResourceBundle("zh-CN", "node")) {
    i18n.addResourceBundle("zh-CN", "node", nodeZh, true, true);
    i18n.addResourceBundle("en", "node", nodeEn, true, true);
    i18n.addResourceBundle("ja", "node", nodeJa, true, true);
    i18n.addResourceBundle("ko", "node", nodeKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/subscriptions/$id")({
  beforeLoad: () => {
    ensureNamespaces();
  },
  component: SubscriptionDetailPage,
});

// ── Helpers ─────────────────────────────────────────────────────────────────

function buildShareUrl(name: string, token?: string): string {
  if (!token) return "";
  const origin = typeof window !== "undefined" ? window.location.origin : "";
  const path = prefixedPath(`/download/${encodeURIComponent(name)}`);
  return `${origin}${path}?token=${encodeURIComponent(token)}`;
}

function statusDotColor(status?: SyncStatus): string {
  switch (status) {
    case "ok":
      return "bg-[var(--color-success)] shadow-[0_0_6px_var(--color-success)]";
    case "error":
      return "bg-[var(--color-error)]";
    case "pending":
      return "bg-[var(--color-warning)]";
    default:
      return "bg-[var(--color-text-disabled)]";
  }
}

function statusVariant(status?: SyncStatus): "success" | "destructive" | "warning" | "secondary" {
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

function trafficPercent(used?: number, total?: number): number {
  const u = used ?? 0;
  if (!total || total <= 0) return 0;
  return Math.min(100, (u / total) * 100);
}

function progressColor(pct: number): string {
  if (pct >= 90) return "bg-[var(--color-error)]";
  if (pct >= 70) return "bg-[var(--color-warning)]";
  return "bg-[var(--color-info)]";
}

function fmtInterval(seconds: number): string {
  if (!seconds || seconds <= 0) return "--";
  const h = seconds / 3600;
  if (h >= 24) return `${Math.round(h / 24)}d`;
  if (h >= 1) return `${Math.round(h)}h`;
  return `${Math.round(seconds / 60)}m`;
}

function daysUntil(epochMs: number): number {
  return Math.max(0, Math.ceil((epochMs - Date.now()) / 86_400_000));
}

// ═══════════════════════════════════════════════════════════════════════════
// Main page component
// ═══════════════════════════════════════════════════════════════════════════

function SubscriptionDetailPage() {
  const { id } = Route.useParams();
  const { t } = useTranslation(["subscription", "common", "node"]);
  const { handle: handleError } = useApiError();
  const navigate = useNavigate();

  // ── Data queries ──
  const { data, isLoading, isError, error, refetch } = useSubscriptionQuery(id);
  const {
    data: nodesPage,
    isLoading: nodesLoading,
  } = useNodesQuery({ subscriptionId: id, pageSize: 200 });

  // ── Mutations ──
  const syncMutation = useSyncSubscriptionMutation();
  const deleteMutation = useDeleteSubscriptionMutation();
  const rotateMutation = useRotateShareTokenMutation();
  const tcpingBatch = useTCPingBatchMutation();

  // ── Local state ──
  const [editOpen, setEditOpen] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [rotateOpen, setRotateOpen] = React.useState(false);

  // ── Loading / error states ──
  if (isLoading) return <DetailSkeleton />;
  if (isError || !data) {
    const errMsg = error instanceof Error ? error.message : String(error ?? "");
    return (
      <div className="p-2">
        <ErrorState
          message={
            t("subscription:error.load_detail_failed") +
            (errMsg ? ` (${errMsg})` : "")
          }
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      </div>
    );
  }

  const shareUrl = buildShareUrl(data.name, data.share_token);
  const nodes: NodeWithLatency[] = nodesPage?.items ?? [];
  const pct = trafficPercent(data.traffic_used, data.traffic_total);
  const hasTraffic = data.traffic_total && data.traffic_total > 0;

  // ── Handlers ──
  const onSync = async () => {
    try {
      const res = await syncMutation.mutateAsync(id);
      toast.success(
        t("subscription:detail.sync_success", {
          added: res.added_count,
          removed: res.removed_count,
        }),
      );
    } catch (err) {
      handleError(err);
    }
  };

  const onCopyShareUrl = async () => {
    if (!shareUrl) return;
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      await navigator.clipboard.writeText(shareUrl);
    }
    toast.success(t("subscription:detail.share.copy_success"));
  };

  const onCopyToken = async () => {
    if (!data.share_token) return;
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      await navigator.clipboard.writeText(data.share_token);
    }
    toast.success(t("subscription:detail.share.copy_success"));
  };

  const confirmDelete = async () => {
    try {
      await deleteMutation.mutateAsync(id);
      toast.success(t("subscription:detail.delete_confirm.success"));
      setDeleteOpen(false);
      navigate({ to: "/subscriptions" });
    } catch (err) {
      handleError(err);
    }
  };

  const confirmRotate = async () => {
    try {
      await rotateMutation.mutateAsync(id);
      toast.success(t("subscription:detail.share.rotate_success"));
      setRotateOpen(false);
    } catch (err) {
      handleError(err);
    }
  };

  const onBatchTCPing = async () => {
    if (nodes.length === 0) return;
    try {
      const ids = nodes.map((n) => n.id);
      await tcpingBatch.mutateAsync({ node_ids: ids.slice(0, 200) });
      toast.success(t("node:batch.completed", { total: ids.length }));
    } catch (err) {
      handleError(err);
    }
  };

  // ── Token display (masked) ──
  const maskedToken = data.share_token
    ? `${data.share_token.slice(0, 6)}...${data.share_token.slice(-6)}`
    : "--";

  // ═════════════════════════════════════════════════════════════════════════
  // RENDER
  // ═════════════════════════════════════════════════════════════════════════

  return (
    <div className="flex flex-col gap-4">
      {/* Breadcrumb */}
      <nav className="text-[12px] text-[var(--color-text-tertiary)]">
        <Link
          to="/subscriptions"
          className="hover:text-[var(--color-text-secondary)]"
        >
          {t("subscription:title")}
        </Link>
        <span className="mx-1 opacity-40">/</span>
        <span className="text-[var(--color-text-primary)]">{data.name}</span>
      </nav>

      {/* V4 two-column layout */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[300px_1fr]">
        {/* ───────────── LEFT SIDEBAR ───────────── */}
        <aside className="self-start lg:sticky lg:top-6">
          <div
            className={cn(
              "flex flex-col overflow-hidden rounded-[var(--radius-lg)]",
              "border border-[var(--color-border)]",
              "bg-[var(--color-surface)] backdrop-blur-xl",
            )}
          >
            {/* Header: name + dot */}
            <div className="px-5 pt-5 pb-0">
              <h2 className="flex items-center gap-2 text-xl font-bold text-[var(--color-text-primary)]">
                <span
                  className={cn(
                    "inline-block h-[9px] w-[9px] flex-shrink-0 rounded-full",
                    statusDotColor(data.last_sync_status),
                  )}
                />
                <span className="truncate">{data.name}</span>
              </h2>
              <div className="mt-2 flex flex-wrap gap-1">
                <Badge variant={sourceVariant(data.type)}>
                  {t(`subscription:source_type.${data.type}`)}
                </Badge>
                <Badge variant={statusVariant(data.last_sync_status)}>
                  {t(`subscription:status.${data.last_sync_status ?? "never"}`)}
                </Badge>
              </div>
            </div>

            {/* Field list */}
            <div className="px-5 pt-3 pb-1">
              {/* Nodes */}
              <FieldRow label={t("subscription:columns.nodes")}>
                <span className="text-[13px] tabular-nums">
                  <strong>{data.node_count}</strong>
                </span>
              </FieldRow>

              {/* Traffic + progress bar */}
              <FieldRow label={t("subscription:detail.metadata.traffic")}>
                <span className="text-[13px] tabular-nums">
                  {formatBytes(data.traffic_used ?? 0)}
                  {" / "}
                  {hasTraffic
                    ? formatBytes(data.traffic_total!)
                    : t("subscription:detail.metadata.traffic_unlimited")}
                </span>
                <div className="mt-1 h-[3px] w-full overflow-hidden rounded-sm bg-[var(--color-border)]">
                  <div
                    className={cn("h-full rounded-sm transition-all", progressColor(pct))}
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </FieldRow>

              {/* Expiry */}
              <FieldRow label={t("subscription:detail.metadata.expire_at")}>
                <span className="text-[13px] tabular-nums">
                  {data.expire_at
                    ? `${daysUntil(data.expire_at)} ${t("common:units.days", { defaultValue: "d" })} · ${formatDate(data.expire_at, { year: "numeric", month: "2-digit", day: "2-digit" })}`
                    : t("subscription:detail.metadata.none")}
                </span>
              </FieldRow>

              {/* Sync interval */}
              <FieldRow label={t("subscription:detail.metadata.sync_interval")}>
                <span className="text-[13px]">
                  {data.sync_interval > 0
                    ? `${fmtInterval(data.sync_interval)}`
                    : t("subscription:wizard.sync_interval.manual")}
                  {data.last_synced_at && (
                    <span className="text-[var(--color-text-tertiary)]">
                      {" · "}
                      {formatRelativeTime(data.last_synced_at)}
                    </span>
                  )}
                </span>
              </FieldRow>

              {/* Tags */}
              {data.tags && data.tags.length > 0 && (
                <FieldRow label={t("subscription:detail.metadata.tags")}>
                  <div className="flex flex-wrap gap-1 pt-0.5">
                    {data.tags.map((tag) => (
                      <span
                        key={tag}
                        className="rounded bg-[var(--color-surface-hover)] px-1.5 py-px text-[10px] font-medium text-[var(--color-text-tertiary)]"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                </FieldRow>
              )}

              {/* Token */}
              <FieldRow label="Token">
                <button
                  className="cursor-pointer font-mono text-[11px] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                  onClick={onCopyToken}
                  title={t("common:actions.copy")}
                >
                  {maskedToken}
                </button>
              </FieldRow>
            </div>

            {/* Action buttons */}
            <div className="flex flex-col gap-1.5 border-t border-[var(--color-border)] px-5 py-4">
              <Button
                className="w-full justify-center"
                onClick={onSync}
                disabled={syncMutation.isPending}
              >
                <RefreshCw
                  className={cn(
                    "mr-2 h-4 w-4",
                    syncMutation.isPending && "animate-spin",
                  )}
                />
                {t("subscription:actions.sync")}
              </Button>
              <Button
                variant="outline"
                className="w-full justify-center"
                onClick={onCopyShareUrl}
                disabled={!shareUrl}
              >
                <ClipboardCopy className="mr-2 h-4 w-4" />
                {t("subscription:actions.copy_url")}
              </Button>
              <div className="flex gap-1.5">
                <Button
                  variant="outline"
                  className="flex-1 justify-center"
                  onClick={() => setEditOpen(true)}
                >
                  <Pencil className="mr-1.5 h-4 w-4" />
                  {t("subscription:actions.edit")}
                </Button>
                <Button
                  variant="outline"
                  className="flex-1 justify-center text-[var(--color-error)]"
                  onClick={() => setDeleteOpen(true)}
                >
                  <Trash2 className="mr-1.5 h-4 w-4" />
                </Button>
              </div>
            </div>
          </div>
        </aside>

        {/* ───────────── RIGHT MAIN AREA ───────────── */}
        <main className="flex flex-col gap-5">
          {/* ── Section: Nodes ── */}
          <section>
            <SectionHeader
              emoji="🌐"
              title={t("subscription:detail.tabs.nodes", { count: data.node_count })}
            >
              <Button
                variant="outline"
                size="sm"
                onClick={onBatchTCPing}
                disabled={tcpingBatch.isPending || nodes.length === 0}
                className="text-[11px]"
              >
                {tcpingBatch.isPending ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Activity className="mr-1.5 h-3.5 w-3.5" />
                )}
                TCPing
              </Button>
            </SectionHeader>

            {nodesLoading ? (
              <div className="grid grid-cols-[repeat(auto-fill,minmax(190px,1fr))] gap-2">
                {Array.from({ length: 6 }).map((_, i) => (
                  <Skeleton key={i} className="h-[76px] rounded-lg" />
                ))}
              </div>
            ) : nodes.length > 0 ? (
              <div className="grid grid-cols-[repeat(auto-fill,minmax(190px,1fr))] gap-2">
                {nodes.map((node) => (
                  <NodeMiniCard key={node.id} node={node} />
                ))}
              </div>
            ) : (
              <EmptyState
                title={t("node:empty.title")}
                description={t("node:empty.description")}
              />
            )}
          </section>

          {/* ── Section: Share ── */}
          <section>
            <SectionHeader
              emoji="📋"
              title={t("subscription:detail.share.title")}
            >
              <Button
                variant="outline"
                size="sm"
                onClick={() => setRotateOpen(true)}
                disabled={rotateMutation.isPending}
                className="text-[11px] text-[var(--color-warning)] border-[rgba(251,191,36,.2)]"
              >
                <RotateCw className="mr-1.5 h-3.5 w-3.5" />
                {t("subscription:actions.rotate_token")}
              </Button>
            </SectionHeader>

            <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2">
              {CLIENT_CATALOG.map((client) => (
                <ClientCard
                  key={client.id}
                  client={client}
                  baseUrl={shareUrl}
                  subscriptionName={data.name}
                  disabled={!data.share_token}
                />
              ))}
            </div>
          </section>

          {/* ── Section: Sync History ── */}
          <section>
            <SectionHeader emoji="📜" title={t("subscription:detail.sync_history.title")} />
            <div
              className={cn(
                "overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)]",
                "bg-[var(--color-surface)] backdrop-blur-xl",
                "px-4 py-3",
              )}
            >
              <SyncHistory subscriptionId={data.id} />
            </div>
          </section>
        </main>
      </div>

      {/* ── Modals ── */}
      <SubEditForm
        open={editOpen}
        subscription={data}
        onClose={() => setEditOpen(false)}
      />

      <Dialog open={deleteOpen} onOpenChange={(o) => !o && setDeleteOpen(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:detail.delete_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("subscription:detail.delete_confirm.description", {
                name: data.name,
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteOpen(false)}
              disabled={deleteMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmDelete}
              disabled={deleteMutation.isPending}
            >
              {t("subscription:detail.delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={rotateOpen} onOpenChange={(o) => !o && !rotateMutation.isPending && setRotateOpen(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:detail.share.rotate_title")}
            </DialogTitle>
            <DialogDescription>
              {t("subscription:detail.share.rotate_confirm")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setRotateOpen(false)}
              disabled={rotateMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={confirmRotate}
              disabled={rotateMutation.isPending}
            >
              {t("subscription:actions.rotate_token")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// ── Reusable sub-components ────────────────────────────────────────────────

/** A labelled field row in the left sidebar. */
function FieldRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-0.5 border-b border-[var(--color-border)] py-2 last:border-b-0">
      <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-text-tertiary)]">
        {label}
      </span>
      <div className="text-[var(--color-text-primary)]">{children}</div>
    </div>
  );
}

/** Section header with emoji, title, and optional trailing actions. */
function SectionHeader({
  emoji,
  title,
  children,
}: {
  emoji: string;
  title: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="mb-2 flex items-center gap-1.5">
      <span className="text-[13px]">{emoji}</span>
      <h3 className="text-[13px] font-semibold text-[var(--color-text-primary)]">
        {title}
      </h3>
      {children && <div className="ml-auto">{children}</div>}
    </div>
  );
}

/** A single history timeline row. */
// ── Skeleton ────────────────────────────────────────────────────────────────

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <Skeleton className="h-3 w-32" />
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[300px_1fr]">
        <Skeleton className="h-[400px] rounded-[var(--radius-lg)]" />
        <div className="flex flex-col gap-5">
          <Skeleton className="h-8 w-48" />
          <div className="grid grid-cols-[repeat(auto-fill,minmax(190px,1fr))] gap-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-[76px] rounded-lg" />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
