import * as React from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { GitBranch } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import {
  useDeleteSubscriptionMutation,
  useSubscriptionQuery,
  useSyncSubscriptionMutation,
} from "@/api/subscription";
import {
  useCopyNodeURIMutation,
  useDeleteNodeMutation,
} from "@/api/node";
import { SubDetailHeader } from "@/components/subscription/sub-detail-header";
import { SubEditForm } from "@/components/subscription/sub-edit-form";
import { SubSyncHistory } from "@/components/subscription/sub-sync-history";
import { ShareUrlCard } from "@/components/subscription/share-url-card";
import { NodeTable } from "@/components/nodes/node-table";
import { prefixedPath } from "@/lib/silent-prefix";
import { formatBytes, formatDate } from "@/lib/format";
import i18n from "@/lib/i18n";
import subZh from "@/locales/zh-CN/subscription.json";
import subEn from "@/locales/en/subscription.json";
import subJa from "@/locales/ja/subscription.json";
import subKo from "@/locales/ko/subscription.json";
import nodeZh from "@/locales/zh-CN/node.json";
import nodeEn from "@/locales/en/node.json";
import nodeJa from "@/locales/ja/node.json";
import nodeKo from "@/locales/ko/node.json";
import type { NodeWithLatency } from "@/types/api";

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

type DetailTab = "nodes" | "metadata" | "sync_history" | "pipeline" | "share";

function SubscriptionDetailPage() {
  const { id } = Route.useParams();
  const { t } = useTranslation(["subscription", "common", "node"]);
  const { handle: handleError } = useApiError();
  const navigate = useNavigate();

  const { data, isLoading, isError, error, refetch } = useSubscriptionQuery(id);
  const syncMutation = useSyncSubscriptionMutation();
  const deleteMutation = useDeleteSubscriptionMutation();
  const copyNodeUri = useCopyNodeURIMutation();
  const deleteNode = useDeleteNodeMutation();

  const [tab, setTab] = React.useState<DetailTab>("nodes");
  const [editOpen, setEditOpen] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [selected, setSelected] = React.useState<string[]>([]);

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

  const toggleSelect = (nid: string, next: boolean) =>
    setSelected((prev) =>
      next ? [...new Set([...prev, nid])] : prev.filter((v) => v !== nid),
    );
  const toggleSelectAll = (ids: string[], next: boolean) =>
    setSelected((prev) =>
      next ? [...new Set([...prev, ...ids])] : prev.filter((v) => !ids.includes(v)),
    );

  const onCopyNodeUri = async (node: NodeWithLatency) => {
    try {
      const res = await copyNodeUri.mutateAsync(node.id);
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(res.raw_uri);
      }
      toast.success(t("node:detail.copied"));
    } catch (err) {
      handleError(err);
    }
  };

  const onDeleteNode = async (node: NodeWithLatency) => {
    try {
      await deleteNode.mutateAsync(node.id);
      setSelected((prev) => prev.filter((v) => v !== node.id));
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <SubDetailHeader
        subscription={data}
        shareUrl={shareUrl}
        syncing={syncMutation.isPending}
        onSync={onSync}
        onEdit={() => setEditOpen(true)}
        onDelete={() => setDeleteOpen(true)}
        onCopyShareUrl={onCopyShareUrl}
      />

      <Tabs value={tab} onValueChange={(v) => setTab(v as DetailTab)}>
        <TabsList className="flex w-full justify-start gap-1 bg-[var(--color-surface)] p-1">
          <TabsTrigger value="nodes">
            {t("subscription:detail.tabs.nodes", { count: data.node_count })}
          </TabsTrigger>
          <TabsTrigger value="metadata">
            {t("subscription:detail.tabs.metadata")}
          </TabsTrigger>
          <TabsTrigger value="sync_history">
            {t("subscription:detail.tabs.sync_history")}
          </TabsTrigger>
          <TabsTrigger value="pipeline">
            {t("subscription:detail.tabs.pipeline")}
          </TabsTrigger>
          <TabsTrigger value="share">
            {t("subscription:detail.tabs.share")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="nodes" className="mt-6">
          <NodeTable
            params={{ subscriptionId: id }}
            selected={selected}
            onToggleSelect={toggleSelect}
            onToggleSelectAll={toggleSelectAll}
            onCopyURI={onCopyNodeUri}
            onDelete={onDeleteNode}
          />
        </TabsContent>

        <TabsContent value="metadata" className="mt-6">
          <MetadataPanel subscription={data} />
        </TabsContent>

        <TabsContent value="sync_history" className="mt-6">
          <SubSyncHistory subscription={data} />
        </TabsContent>

        <TabsContent value="pipeline" className="mt-6">
          <PipelinePlaceholder />
        </TabsContent>

        <TabsContent value="share" className="mt-6">
          <ShareUrlCard
            subscriptionId={id}
            subscriptionName={data.name}
            shareUrl={shareUrl}
            available={!!data.share_token}
          />
        </TabsContent>
      </Tabs>

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
    </div>
  );
}

// ── Metadata panel ──────────────────────────────────────────────────────────

function MetadataPanel({
  subscription,
}: {
  subscription: import("@/types/api").Subscription;
}) {
  const { t } = useTranslation("subscription");
  const rows: Array<[string, React.ReactNode]> = [
    [
      t("subscription:detail.metadata.type"),
      t(`subscription:source_type.${subscription.type}`),
    ],
    [
      t("subscription:detail.metadata.source_url"),
      subscription.source_url ?? t("subscription:detail.metadata.none"),
    ],
    [
      t("subscription:detail.metadata.ua"),
      subscription.ua ?? t("subscription:detail.metadata.none"),
    ],
    [
      t("subscription:detail.metadata.sync_interval"),
      subscription.sync_interval > 0
        ? `${subscription.sync_interval}s`
        : t("subscription:wizard.sync_interval.manual"),
    ],
    [
      t("subscription:detail.metadata.expire_at"),
      subscription.expire_at
        ? formatDate(subscription.expire_at)
        : t("subscription:detail.metadata.none"),
    ],
    [
      t("subscription:detail.metadata.traffic"),
      subscription.traffic_total && subscription.traffic_total > 0
        ? `${formatBytes(subscription.traffic_used ?? 0)} / ${formatBytes(subscription.traffic_total)}`
        : `${formatBytes(subscription.traffic_used ?? 0)} / ${t("subscription:detail.metadata.traffic_unlimited")}`,
    ],
    [
      t("subscription:detail.metadata.tags"),
      subscription.tags && subscription.tags.length > 0
        ? subscription.tags.join(", ")
        : t("subscription:detail.metadata.none"),
    ],
    [
      t("subscription:detail.metadata.remark"),
      subscription.remark ?? t("subscription:detail.metadata.none"),
    ],
    [
      t("subscription:detail.metadata.created_at"),
      formatDate(subscription.created_at),
    ],
    [
      t("subscription:detail.metadata.updated_at"),
      formatDate(subscription.updated_at),
    ],
  ];

  return (
    <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]">
      <dl className="divide-y divide-[var(--color-border)]">
        {rows.map(([label, value]) => (
          <div
            key={String(label)}
            className="grid grid-cols-3 gap-3 px-4 py-3"
          >
            <dt className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
              {label}
            </dt>
            <dd className="col-span-2 break-all text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
              {value}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

// ── Pipeline placeholder ────────────────────────────────────────────────────

function PipelinePlaceholder() {
  const { t } = useTranslation("subscription");
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-[var(--radius-lg)] border border-dashed border-[var(--color-border-strong)] bg-[var(--color-surface)] py-16 text-center">
      <GitBranch className="h-10 w-10 text-[var(--color-text-disabled)]" />
      <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
        {t("subscription:detail.pipeline.placeholder_title")}
      </h3>
      <p className="max-w-md text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
        {t("subscription:detail.pipeline.placeholder_desc")}
      </p>
      <p className="rounded-[var(--radius-sm)] bg-[var(--color-surface-hover)] px-3 py-1 text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
        {t("subscription:detail.pipeline.drop_hint")}
      </p>
    </div>
  );
}

// ── Skeleton ────────────────────────────────────────────────────────────────

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-3 w-32" />
        <div className="flex items-center justify-between">
          <Skeleton className="h-8 w-48" />
          <div className="flex gap-2">
            <Skeleton className="h-9 w-24" />
            <Skeleton className="h-9 w-24" />
          </div>
        </div>
      </div>
      <Skeleton className="h-10 w-full" />
      <div className="flex flex-col gap-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    </div>
  );
}

// ── Helpers ─────────────────────────────────────────────────────────────────

/**
 * Build the sub-store compatible URL.
 *  - Uses the current window origin so the link is consumable from any browser.
 *  - Includes the silent-mode prefix when configured (per docs §6.3).
 *  - URL-encodes the name (allows spaces / CJK in subscription names).
 */
function buildShareUrl(name: string, token?: string): string {
  if (!token) return "";
  const origin = typeof window !== "undefined" ? window.location.origin : "";
  const path = prefixedPath(`/download/${encodeURIComponent(name)}`);
  return `${origin}${path}?token=${encodeURIComponent(token)}`;
}
