import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  HardDrive,
  Plus,
  Search,
  DollarSign,
  AlertTriangle,
  XCircle,
  Copy,
  Pencil,
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
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import { cn } from "@/lib/cn";
import {
  useVpsAssetsQuery,
  useVpsAssetSummaryQuery,
  useDeleteVpsAssetMutation,
} from "@/api/vps-asset";
import { VpsAssetFormDialog } from "@/components/vps-asset/vps-asset-form";
import { useAgentsQuery } from "@/api/agent";
import { useTrafficSummaryQuery } from "@/api/traffic";
import { formatBytes } from "@/lib/format";
import type {
  AgentListItem,
  AgentTrafficSummary,
  VpsAsset,
  VpsAssetStatus,
} from "@/types/api";

export const Route = createFileRoute("/_authed/vps-assets/")({
  component: VpsAssetsPage,
});

type StatusFilter = "all" | VpsAssetStatus;

function VpsAssetsPage() {
  const { t } = useTranslation(["vps-asset", "common"]);
  const { handle: handleError } = useApiError();

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);
  const [statusFilter, setStatusFilter] = React.useState<StatusFilter>("all");

  const [createOpen, setCreateOpen] = React.useState(false);
  const [editTarget, setEditTarget] = React.useState<VpsAsset | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<VpsAsset | null>(null);

  const deleteMutation = useDeleteVpsAssetMutation();

  const { data, isLoading, isError, error, refetch } = useVpsAssetsQuery({
    page: 1,
    pageSize: 500,
  });
  const { data: summary } = useVpsAssetSummaryQuery();
  // Linked-agent live state + monthly traffic for the per-card badge.
  const { data: agentsPage } = useAgentsQuery();
  const { data: trafficSummary } = useTrafficSummaryQuery();
  const agentById = React.useMemo(() => {
    const m = new Map<string, AgentListItem>();
    for (const a of agentsPage?.items ?? []) m.set(a.id, a);
    return m;
  }, [agentsPage]);
  const trafficByAgent = React.useMemo(() => {
    const m = new Map<string, AgentTrafficSummary>();
    for (const a of trafficSummary?.agents ?? []) m.set(a.agent_id, a);
    return m;
  }, [trafficSummary]);

  const allItems = data?.items ?? [];

  const filtered = React.useMemo(() => {
    let items = allItems;
    if (keyword) {
      const kw = keyword.toLowerCase();
      items = items.filter(
        (v) =>
          v.name.toLowerCase().includes(kw) ||
          v.provider.toLowerCase().includes(kw) ||
          (v.ip && v.ip.toLowerCase().includes(kw)),
      );
    }
    if (statusFilter !== "all") {
      items = items.filter((v) => v.status === statusFilter);
    }
    return items;
  }, [allItems, keyword, statusFilter]);

  const costLabel = React.useMemo(() => {
    if (!summary?.monthly_cost?.length) return "—";
    return summary.monthly_cost
      .map((c) => `${currencySymbol(c.currency)}${c.monthly_cost.toFixed(1)}`)
      .join(" + ");
  }, [summary]);

  const copyIp = (ip: string) => {
    navigator.clipboard.writeText(ip);
    toast.success(t("vps-asset:toast.ip_copied"));
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteMutation.mutateAsync(deleteTarget.id);
      toast.success(t("vps-asset:delete_confirm.success"));
      setDeleteTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-5">
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
            {t("vps-asset:title")}
          </h1>
          <p className="mt-1 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("vps-asset:subtitle")}
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("vps-asset:actions.create")}
        </Button>
      </header>

      {!isLoading && !isError && summary && (
        <div
          className={cn(
            "grid grid-cols-2 gap-px overflow-hidden rounded-[var(--radius-lg)]",
            "border border-[var(--color-border)] bg-[var(--color-border)]",
            "sm:grid-cols-4",
          )}
        >
          <SummaryCell
            icon={<HardDrive className="h-4 w-4 text-[var(--color-primary)]" />}
            label={t("vps-asset:summary.total")}
            value={String(summary.total)}
          />
          <SummaryCell
            icon={<DollarSign className="h-4 w-4 text-[var(--color-info)]" />}
            label={t("vps-asset:summary.monthly_cost")}
            value={costLabel}
          />
          <SummaryCell
            icon={<AlertTriangle className="h-4 w-4 text-[var(--color-warning)]" />}
            label={t("vps-asset:summary.expiring")}
            value={String(summary.expiring)}
            highlight={summary.expiring > 0 ? "warning" : undefined}
          />
          <SummaryCell
            icon={<XCircle className="h-4 w-4 text-[var(--color-error)]" />}
            label={t("vps-asset:summary.expired")}
            value={String(summary.expired)}
            highlight={summary.expired > 0 ? "error" : undefined}
          />
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("vps-asset:filter.search_placeholder")}
            className="pl-9"
          />
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          {(["all", "normal", "expiring", "expired"] as const).map((v) => (
            <FilterChip
              key={v}
              active={statusFilter === v}
              onClick={() => setStatusFilter(v)}
            >
              {t(`vps-asset:filter.${v}`)}
            </FilterChip>
          ))}
        </div>
      </div>

      {isLoading && <CardGridSkeleton />}
      {isError && (
        <ErrorState
          message={
            t("vps-asset:error.load_failed") +
            (error instanceof Error ? ` (${error.message})` : "")
          }
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      )}
      {!isLoading && !isError && filtered.length === 0 && allItems.length > 0 && (
        <EmptyState
          icon={<Search />}
          title={t("vps-asset:filter.all")}
          description={t("vps-asset:subtitle")}
        />
      )}
      {!isLoading && !isError && allItems.length === 0 && (
        <EmptyState
          icon={<HardDrive />}
          title={t("vps-asset:empty.title")}
          description={t("vps-asset:empty.desc")}
          ctaLabel={t("vps-asset:empty.cta")}
          onCta={() => setCreateOpen(true)}
        />
      )}
      {!isLoading && !isError && filtered.length > 0 && (
        <div
          className="grid gap-3"
          style={{ gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))" }}
        >
          {filtered.map((vps) => (
            <VpsCard
              key={vps.id}
              vps={vps}
              agent={vps.agent_id ? agentById.get(vps.agent_id) : undefined}
              traffic={
                vps.agent_id ? trafficByAgent.get(vps.agent_id) : undefined
              }
              onEdit={setEditTarget}
              onDelete={setDeleteTarget}
              onCopyIp={copyIp}
            />
          ))}
        </div>
      )}

      <VpsAssetFormDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
      />

      {editTarget && (
        <VpsAssetFormDialog
          open={!!editTarget}
          vps={editTarget}
          onClose={() => setEditTarget(null)}
        />
      )}

      <Dialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("vps-asset:delete_confirm.title")}</DialogTitle>
            <DialogDescription>
              {t("vps-asset:delete_confirm.description", {
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
              {t("vps-asset:delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// ---------------------------------------------------------------------------
// VPS Card (V7D design)
// ---------------------------------------------------------------------------

const LOCATION_FLAGS: Record<string, string> = {
  "hong kong": "\u{1F1ED}\u{1F1F0}",
  hk: "\u{1F1ED}\u{1F1F0}",
  japan: "\u{1F1EF}\u{1F1F5}",
  tokyo: "\u{1F1EF}\u{1F1F5}",
  jp: "\u{1F1EF}\u{1F1F5}",
  us: "\u{1F1FA}\u{1F1F8}",
  "united states": "\u{1F1FA}\u{1F1F8}",
  ashburn: "\u{1F1FA}\u{1F1F8}",
  "los angeles": "\u{1F1FA}\u{1F1F8}",
  singapore: "\u{1F1F8}\u{1F1EC}",
  sg: "\u{1F1F8}\u{1F1EC}",
  germany: "\u{1F1E9}\u{1F1EA}",
  de: "\u{1F1E9}\u{1F1EA}",
  falkenstein: "\u{1F1E9}\u{1F1EA}",
  uk: "\u{1F1EC}\u{1F1E7}",
  london: "\u{1F1EC}\u{1F1E7}",
  korea: "\u{1F1F0}\u{1F1F7}",
  kr: "\u{1F1F0}\u{1F1F7}",
  taiwan: "\u{1F1F9}\u{1F1FC}",
  tw: "\u{1F1F9}\u{1F1FC}",
  netherlands: "\u{1F1F3}\u{1F1F1}",
  nl: "\u{1F1F3}\u{1F1F1}",
  france: "\u{1F1EB}\u{1F1F7}",
  fr: "\u{1F1EB}\u{1F1F7}",
  canada: "\u{1F1E8}\u{1F1E6}",
  ca: "\u{1F1E8}\u{1F1E6}",
  australia: "\u{1F1E6}\u{1F1FA}",
  au: "\u{1F1E6}\u{1F1FA}",
  russia: "\u{1F1F7}\u{1F1FA}",
  ru: "\u{1F1F7}\u{1F1FA}",
};

function guessFlag(location?: string): string {
  if (!location) return "\u{1F310}";
  const lower = location.toLowerCase();
  for (const [key, flag] of Object.entries(LOCATION_FLAGS)) {
    if (lower.includes(key)) return flag;
  }
  return "\u{1F310}";
}

function currencySymbol(currency: string): string {
  switch (currency.toUpperCase()) {
    case "CNY": return "¥";
    case "USD": return "$";
    case "EUR": return "€";
    case "GBP": return "£";
    case "JPY": return "¥";
    case "KRW": return "₩";
    default: return currency + " ";
  }
}

function billingCycleSuffix(cycle: string, t: (key: string) => string): string {
  return t(`vps-asset:billing_cycle.${cycle}`);
}

function statusColor(status: VpsAssetStatus) {
  switch (status) {
    case "normal": return { text: "var(--color-success)", bg: "var(--color-success-bg)" };
    case "expiring": return { text: "var(--color-warning)", bg: "var(--color-warning-bg)" };
    case "expired": return { text: "var(--color-error)", bg: "var(--color-error-bg)" };
  }
}

function expiryBarPercent(days: number): number {
  if (days <= 0) return 0;
  if (days >= 365) return 95;
  return Math.max(3, Math.round((days / 365) * 100));
}

function VpsCard({
  vps,
  agent,
  traffic,
  onEdit,
  onDelete,
  onCopyIp,
}: {
  vps: VpsAsset;
  agent?: AgentListItem;
  traffic?: AgentTrafficSummary;
  onEdit: (v: VpsAsset) => void;
  onDelete: (v: VpsAsset) => void;
  onCopyIp: (ip: string) => void;
}) {
  const { t } = useTranslation(["vps-asset"]);
  const sc = statusColor(vps.status);
  const flag = guessFlag(vps.location);
  const sym = currencySymbol(vps.currency);
  const spec = [vps.cpu, vps.memory, vps.disk, vps.bandwidth].filter(Boolean).join(" · ");
  const days = vps.days_until_expiry;

  return (
    <div
      className={cn(
        "group flex flex-col overflow-hidden rounded-[var(--radius-lg)]",
        "border bg-[var(--color-surface)] backdrop-blur-lg",
        "transition-all duration-150 hover:-translate-y-0.5",
        "hover:shadow-[0_8px_24px_rgba(0,0,0,0.25)]",
        vps.status === "expiring" && "border-[color:rgba(251,191,36,0.2)]",
        vps.status === "expired" && "border-[color:rgba(248,113,113,0.15)]",
        vps.status === "normal" && "border-[var(--color-border)]",
      )}
      style={{
        borderColor:
          vps.status === "expiring"
            ? "rgba(251,191,36,0.2)"
            : vps.status === "expired"
              ? "rgba(248,113,113,0.15)"
              : undefined,
      }}
    >
      {/* Top band: flag + name + day chip */}
      <div className="flex items-center gap-3 px-4 py-3.5">
        <span className="text-[28px] leading-none">{flag}</span>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[15px] font-bold text-[var(--color-text-primary)]">
            {vps.name}
          </div>
          <div className="truncate text-[10px] text-[var(--color-text-tertiary)]">
            {vps.provider} &middot; {sym}{vps.price}/{billingCycleSuffix(vps.billing_cycle, t)}
          </div>
          {spec && (
            <div className="truncate font-mono text-[10px] text-[var(--color-text-secondary)]">
              {spec}
            </div>
          )}
        </div>
        <div className="flex flex-col items-center" style={{ minWidth: 56 }}>
          <span
            className="text-[32px] font-extrabold leading-none tabular-nums"
            style={{ color: sc.text }}
          >
            {days}
          </span>
          <span
            className="text-[8px] font-semibold uppercase tracking-wider"
            style={{ color: sc.text, marginTop: 2 }}
          >
            {days <= 0 ? t("vps-asset:card.expired_label") : t("vps-asset:card.days_left")}
          </span>
        </div>
      </div>

      {/* Expiry progress bar */}
      <div className="mx-4 h-1 overflow-hidden rounded-sm bg-[rgba(255,255,255,0.04)]">
        <div
          className="h-full rounded-sm transition-all"
          style={{
            width: `${expiryBarPercent(days)}%`,
            background: sc.text,
          }}
        />
      </div>

      {/* Info pills */}
      <div className="flex flex-wrap gap-1.5 px-4 py-2.5">
        {vps.ip && (
          <button
            type="button"
            onClick={() => onCopyIp(vps.ip!)}
            className="flex items-center gap-1 rounded-[5px] bg-[var(--color-surface-hover)] px-2 py-1 font-mono text-[10px] text-[var(--color-text-secondary)] transition hover:bg-[var(--color-border)]"
          >
            <Copy className="h-2.5 w-2.5" />
            {vps.ip}
          </button>
        )}
        {vps.agent_id && agent ? (
          <span className="flex items-center gap-1 rounded-[5px] bg-[var(--color-surface-hover)] px-2 py-1 text-[10px]">
            <span
              className={cn(
                "h-1.5 w-1.5 rounded-full",
                agent.online
                  ? "bg-[var(--color-success)]"
                  : "bg-[var(--color-text-disabled)]",
              )}
            />
            <span className="text-[var(--color-text-secondary)]">
              {agent.online
                ? t("vps-asset:card.agent_online")
                : t("vps-asset:card.agent_offline")}
              {agent.online && agent.latest_metrics
                ? ` · CPU ${Math.round(agent.latest_metrics.cpu_percent)}%`
                : ""}
            </span>
          </span>
        ) : (
          <span className="rounded-[5px] bg-[var(--color-surface-hover)] px-2 py-1 text-[10px] text-[var(--color-text-tertiary)]">
            {t("vps-asset:card.no_agent")}
          </span>
        )}
        {traffic && traffic.limit ? (
          <span className="rounded-[5px] bg-[var(--color-surface-hover)] px-2 py-1 text-[10px] tabular-nums text-[var(--color-text-secondary)]">
            {formatBytes(traffic.total_used)} / {formatBytes(traffic.limit)}
          </span>
        ) : null}
        {vps.tags.map((tag) => (
          <span
            key={tag}
            className="rounded-[5px] bg-[var(--color-surface-hover)] px-2 py-1 text-[9px] font-medium text-[var(--color-text-tertiary)]"
          >
            {tag}
          </span>
        ))}
      </div>

      {/* Actions row — shown on hover */}
      <div className="flex items-center justify-end gap-1 border-t border-[var(--color-border)] px-3 py-1.5 opacity-0 transition-opacity group-hover:opacity-100">
        <button
          type="button"
          onClick={() => onEdit(vps)}
          className="flex items-center gap-1 rounded-[var(--radius-sm)] px-2 py-1 text-[10px] text-[var(--color-text-tertiary)] transition hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)]"
        >
          <Pencil className="h-3 w-3" />
          {t("vps-asset:actions.edit")}
        </button>
        <button
          type="button"
          onClick={() => onDelete(vps)}
          className="flex items-center gap-1 rounded-[var(--radius-sm)] px-2 py-1 text-[10px] text-[var(--color-text-tertiary)] transition hover:bg-[var(--color-error-bg)] hover:text-[var(--color-error)]"
        >
          <Trash2 className="h-3 w-3" />
          {t("vps-asset:actions.delete")}
        </button>
      </div>
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
  highlight,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  highlight?: "warning" | "error";
}) {
  return (
    <div className="flex items-center gap-3 bg-[var(--color-surface)] px-4 py-3">
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-md)] bg-[var(--color-surface-hover)]">
        {icon}
      </span>
      <div className="min-w-0">
        <div className="truncate text-xs text-[var(--color-text-tertiary)]">{label}</div>
        <div
          className={cn(
            "truncate text-sm font-semibold tabular-nums",
            highlight === "warning" && "text-[var(--color-warning)]",
            highlight === "error" && "text-[var(--color-error)]",
            !highlight && "text-[var(--color-text-primary)]",
          )}
        >
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
      className="grid gap-3"
      style={{ gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))" }}
    >
      {Array.from({ length: 6 }).map((_, i) => (
        <div
          key={i}
          className="flex flex-col gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4"
        >
          <div className="flex items-center gap-3">
            <Skeleton className="h-7 w-7 rounded" />
            <div className="flex-1">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="mt-1 h-3 w-32" />
            </div>
            <Skeleton className="h-10 w-12" />
          </div>
          <Skeleton className="h-1 w-full" />
          <div className="flex gap-1">
            <Skeleton className="h-5 w-20" />
            <Skeleton className="h-5 w-14" />
          </div>
        </div>
      ))}
    </div>
  );
}
