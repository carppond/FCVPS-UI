import * as React from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  BookOpen,
  Plus,
  Search,
  Globe,
  BarChart3,
  RefreshCw,
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
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import { SubCard } from "@/components/subscription/sub-card";
import { SubCreateWizard } from "@/components/subscription/sub-create-wizard";
import { SubEditForm } from "@/components/subscription/sub-edit-form";
import { useAuthStore } from "@/stores/auth-store";
import {
  useSubscriptionsQuery,
  useDeleteSubscriptionMutation,
  useSyncSubscriptionMutation,
} from "@/api/subscription";
import { cn } from "@/lib/cn";
import { formatBytes } from "@/lib/format";
import i18n from "@/lib/i18n";
import subZh from "@/locales/zh-CN/subscription.json";
import subEn from "@/locales/en/subscription.json";
import subJa from "@/locales/ja/subscription.json";
import subKo from "@/locales/ko/subscription.json";
import type { Subscription, SubType, SyncStatus } from "@/types/api";

// ---------------------------------------------------------------------------
// i18n bootstrap
// ---------------------------------------------------------------------------

function ensureSubNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "subscription")) {
    i18n.addResourceBundle("zh-CN", "subscription", subZh, true, true);
    i18n.addResourceBundle("en", "subscription", subEn, true, true);
    i18n.addResourceBundle("ja", "subscription", subJa, true, true);
    i18n.addResourceBundle("ko", "subscription", subKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/subscriptions/")({
  beforeLoad: () => {
    ensureSubNamespace();
  },
  component: SubscriptionsPage,
});

// ---------------------------------------------------------------------------
// Filter types
// ---------------------------------------------------------------------------

type SourceFilter = "all" | SubType;
type StatusFilter = "all" | SyncStatus;

// ---------------------------------------------------------------------------
// Page component
// ---------------------------------------------------------------------------

function SubscriptionsPage() {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";

  // Search
  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);

  // Filters
  const [sourceFilter, setSourceFilter] = React.useState<SourceFilter>("all");
  const [statusFilter, setStatusFilter] = React.useState<StatusFilter>("all");
  const [allUsers, setAllUsers] = React.useState(false);

  // Dialogs
  const [wizardOpen, setWizardOpen] = React.useState(false);
  const [editTarget, setEditTarget] = React.useState<Subscription | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<Subscription | null>(
    null,
  );

  // Mutations
  const syncMutation = useSyncSubscriptionMutation();
  const deleteMutation = useDeleteSubscriptionMutation();

  // Data — fetch ALL subscriptions (large page_size) for client-side filtering
  const { data, isLoading, isError, error, refetch } = useSubscriptionsQuery({
    keyword: "",
    allUsers: isAdmin && allUsers,
    page: 1,
    pageSize: 500,
  });

  const allItems = React.useMemo(() => data?.items ?? [], [data]);

  // Client-side filtering
  const filtered = React.useMemo(() => {
    let items = allItems;

    // Keyword search (name + tags)
    if (keyword) {
      const kw = keyword.toLowerCase();
      items = items.filter(
        (s) =>
          s.name.toLowerCase().includes(kw) ||
          s.tags.some((tag) => tag.toLowerCase().includes(kw)),
      );
    }

    // Source type filter
    if (sourceFilter !== "all") {
      items = items.filter((s) => s.type === sourceFilter);
    }

    // Status filter
    if (statusFilter !== "all") {
      items = items.filter((s) => {
        if (statusFilter === "ok") return s.last_sync_status === "ok";
        if (statusFilter === "error")
          return s.last_sync_status === "error";
        if (statusFilter === "pending")
          return s.last_sync_status === "pending";
        return true;
      });
    }

    return items;
  }, [allItems, keyword, sourceFilter, statusFilter]);

  // Summary computations
  const summary = React.useMemo(() => {
    const total = allItems.length;
    const totalNodes = allItems.reduce((acc, s) => acc + s.node_count, 0);
    const trafficUsed = allItems.reduce(
      (acc, s) => acc + (s.traffic_used ?? 0),
      0,
    );
    const trafficTotal = allItems.reduce(
      (acc, s) => acc + (s.traffic_total ?? 0),
      0,
    );
    const syncOk = allItems.filter(
      (s) => s.last_sync_status === "ok",
    ).length;
    const syncError = allItems.filter(
      (s) => s.last_sync_status === "error",
    ).length;
    return { total, totalNodes, trafficUsed, trafficTotal, syncOk, syncError };
  }, [allItems]);

  // Handlers
  const onSync = async (sub: Subscription) => {
    try {
      const res = await syncMutation.mutateAsync(sub.id);
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

  const onShare = (sub: Subscription) => {
    void navigate({
      to: "/subscriptions/$id" as never,
      params: { id: sub.id } as never,
    });
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast.success(t("subscription:detail.delete_confirm.success"));
      setDeleteTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-5">
      {/* ── Page header ── */}
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
            {t("subscription:title")}
          </h1>
          <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("subscription:subtitle")}
          </p>
        </div>
        <Button onClick={() => setWizardOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("subscription:actions.create")}
        </Button>
      </header>

      {/* ── Summary strip ── */}
      {!isLoading && !isError && allItems.length > 0 && (
        <div
          className={cn(
            "grid grid-cols-2 gap-px overflow-hidden rounded-[var(--radius-lg)]",
            "border border-[var(--color-border)] bg-[var(--color-border)]",
            "sm:grid-cols-4",
          )}
        >
          <SummaryCell
            icon={<BookOpen className="h-4 w-4 text-[var(--color-primary)]" />}
            label={t("subscription:list.summary.total")}
            value={String(summary.total)}
          />
          <SummaryCell
            icon={<Globe className="h-4 w-4 text-[var(--color-info)]" />}
            label={t("subscription:list.summary.nodes")}
            value={String(summary.totalNodes)}
          />
          <SummaryCell
            icon={<BarChart3 className="h-4 w-4 text-[var(--color-success)]" />}
            label={t("subscription:list.summary.traffic")}
            value={
              summary.trafficTotal > 0
                ? `${formatBytes(summary.trafficUsed)} / ${formatBytes(summary.trafficTotal)}`
                : formatBytes(summary.trafficUsed)
            }
          />
          <SummaryCell
            icon={<RefreshCw className="h-4 w-4 text-[var(--color-warning)]" />}
            label={t("subscription:list.summary.sync_status")}
            value={`${summary.syncOk} ${t("subscription:list.summary.ok_short")} / ${summary.syncError} ${t("subscription:list.summary.err_short")}`}
          />
        </div>
      )}

      {/* ── Toolbar: search + filters ── */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("subscription:filters.search_placeholder")}
            className="pl-9"
          />
        </div>

        {/* Source filter chips */}
        <div className="flex flex-wrap items-center gap-1.5">
          {(["all", "url", "upload", "manual"] as const).map((v) => (
            <FilterChip
              key={v}
              active={sourceFilter === v}
              onClick={() => setSourceFilter(v)}
            >
              {t(`subscription:list.filter.${v}`)}
            </FilterChip>
          ))}
          <span className="mx-1 h-4 w-px bg-[var(--color-border)]" />
          {(["all", "ok", "error"] as const).map((v) => (
            <FilterChip
              key={`st-${v}`}
              active={statusFilter === (v as StatusFilter)}
              onClick={() =>
                setStatusFilter(v === "all" ? "all" : (v as SyncStatus))
              }
            >
              {t(
                `subscription:list.filter.${v === "all" ? "status_all" : v}`,
              )}
            </FilterChip>
          ))}
        </div>

        {isAdmin && (
          <label className="ml-auto flex cursor-pointer items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            <input
              type="checkbox"
              className="h-4 w-4 rounded-[var(--radius-sm)] border-[var(--color-border-strong)]"
              checked={allUsers}
              onChange={(e) => setAllUsers(e.target.checked)}
            />
            {t("subscription:filters.show_all_users")}
          </label>
        )}
      </div>

      {/* ── Content area ── */}
      {isLoading && <CardGridSkeleton />}
      {isError && (
        <ErrorState
          message={
            t("subscription:error.load_failed") +
            (error instanceof Error ? ` (${error.message})` : "")
          }
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      )}
      {!isLoading && !isError && filtered.length === 0 && allItems.length > 0 && (
        <EmptyState
          icon={<Search />}
          title={t("subscription:list.empty.no_match_title")}
          description={t("subscription:list.empty.no_match_desc")}
        />
      )}
      {!isLoading && !isError && allItems.length === 0 && (
        <EmptyState
          icon={<BookOpen />}
          title={t("subscription:list.empty.title")}
          description={t("subscription:list.empty.desc")}
          ctaLabel={t("subscription:list.empty.cta")}
          onCta={() => setWizardOpen(true)}
        />
      )}
      {!isLoading && !isError && filtered.length > 0 && (
        <div className="grid gap-3.5" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(340px, 1fr))" }}>
          {filtered.map((sub) => (
            <SubCard
              key={sub.id}
              subscription={sub}
              onEdit={(s) => setEditTarget(s)}
              onSync={onSync}
              onShare={onShare}
              onDelete={(s) => setDeleteTarget(s)}
            />
          ))}
        </div>
      )}

      {/* ── Dialogs ── */}
      <SubCreateWizard
        open={wizardOpen}
        onClose={() => setWizardOpen(false)}
      />

      {editTarget && (
        <SubEditForm
          open={!!editTarget}
          subscription={editTarget}
          onClose={() => setEditTarget(null)}
        />
      )}

      <Dialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:detail.delete_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("subscription:detail.delete_confirm.description", {
                name: deleteTarget?.name ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteTarget(null)}
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

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function SummaryCell({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-3 bg-[var(--color-surface)] px-4 py-3">
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-md)] bg-[var(--color-surface-hover)]">
        {icon}
      </span>
      <div className="min-w-0">
        <div className="truncate text-xs text-[var(--color-text-tertiary)]">
          {label}
        </div>
        <div className="truncate text-sm font-semibold tabular-nums text-[var(--color-text-primary)]">
          {value}
        </div>
      </div>
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-[6px] px-3 py-1 text-[11px] font-medium transition-all duration-100",
        active
          ? "bg-[var(--color-primary)] text-[var(--color-primary-foreground)]"
          : "bg-transparent text-[var(--color-text-tertiary)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)]",
      )}
    >
      {children}
    </button>
  );
}

function CardGridSkeleton() {
  return (
    <div
      className="grid gap-3.5"
      style={{ gridTemplateColumns: "repeat(auto-fill, minmax(340px, 1fr))" }}
    >
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="flex flex-col gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5"
        >
          <div className="flex items-center justify-between">
            <Skeleton className="h-5 w-32" />
            <div className="flex gap-1">
              <Skeleton className="h-5 w-12" />
              <Skeleton className="h-5 w-12" />
            </div>
          </div>
          <Skeleton className="h-4 w-48" />
          <Skeleton className="h-1 w-full" />
          <div className="flex gap-1">
            <Skeleton className="h-4 w-10" />
            <Skeleton className="h-4 w-10" />
          </div>
          <div className="flex items-center justify-between">
            <Skeleton className="h-3 w-24" />
            <div className="flex gap-1">
              <Skeleton className="h-5 w-6" />
              <Skeleton className="h-5 w-6" />
              <Skeleton className="h-5 w-6" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
