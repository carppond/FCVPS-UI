import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Activity, Boxes, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import { NodeCard } from "@/components/nodes/node-card";
import { BatchTCPingDialog } from "@/components/nodes/batch-tcping-dialog";
import { cn } from "@/lib/cn";
import {
  useNodesQuery,
  useCopyNodeURIMutation,
  useDeleteNodeMutation,
} from "@/api/node";
import { useSubscriptionsQuery } from "@/api/subscription";
import i18n from "@/lib/i18n";
import nodeZh from "@/locales/zh-CN/node.json";
import nodeEn from "@/locales/en/node.json";
import nodeJa from "@/locales/ja/node.json";
import nodeKo from "@/locales/ko/node.json";
import type { NodeWithLatency, NodeProtocol } from "@/types/api";

// ---------------------------------------------------------------------------
// i18n bootstrap
// ---------------------------------------------------------------------------

function ensureNodeNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "node")) {
    i18n.addResourceBundle("zh-CN", "node", nodeZh, true, true);
    i18n.addResourceBundle("en", "node", nodeEn, true, true);
    i18n.addResourceBundle("ja", "node", nodeJa, true, true);
    i18n.addResourceBundle("ko", "node", nodeKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/nodes/")({
  beforeLoad: () => {
    ensureNodeNamespace();
  },
  component: NodesPage,
});

// ---------------------------------------------------------------------------
// Filter types
// ---------------------------------------------------------------------------

type StatusFilter = "all" | "online" | "offline";

const PROTOCOL_OPTIONS: NodeProtocol[] = [
  "vless",
  "trojan",
  "ss",
  "hysteria2",
  "vmess",
  "wireguard",
  "hysteria",
  "tuic",
  "ssr",
  "anytls",
  "socks5",
  "naive",
];

const TCPING_MAX = 200;

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

function NodesPage() {
  const { t } = useTranslation(["node", "common"]);
  const { handle: handleError } = useApiError();

  // ── Local filter state ──
  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);
  const [statusFilter, setStatusFilter] = React.useState<StatusFilter>("all");
  const [protocolFilter, setProtocolFilter] = React.useState<string>("");
  const [subFilter, setSubFilter] = React.useState<string>("");

  // ── Batch TCPing ──
  const [batchOpen, setBatchOpen] = React.useState(false);

  // ── Mutations ──
  const copyMutation = useCopyNodeURIMutation();
  const deleteMutation = useDeleteNodeMutation();

  // ── Data: fetch ALL nodes for client-side filtering ──
  const {
    data: nodesData,
    isLoading,
    isError,
    error,
    refetch,
  } = useNodesQuery({ page: 1, pageSize: 5000 });

  // ── Subscription list for dropdown ──
  const { data: subsData } = useSubscriptionsQuery({
    page: 1,
    pageSize: 500,
  });
  const subscriptions = subsData?.items ?? [];

  const allItems = nodesData?.items ?? [];

  // ── Client-side filtering ──
  const filtered = React.useMemo(() => {
    let items = allItems;

    // Keyword search (name / server / tags)
    if (keyword) {
      const kw = keyword.toLowerCase();
      items = items.filter(
        (n) =>
          (n.tag || "").toLowerCase().includes(kw) ||
          n.server.toLowerCase().includes(kw) ||
          n.tags.some((tag) => tag.toLowerCase().includes(kw)),
      );
    }

    // Status filter
    if (statusFilter === "online") {
      items = items.filter((n) => n.reachable);
    } else if (statusFilter === "offline") {
      items = items.filter((n) => !n.reachable);
    }

    // Protocol filter
    if (protocolFilter) {
      items = items.filter((n) => n.protocol === protocolFilter);
    }

    // Subscription filter
    if (subFilter) {
      items = items.filter((n) => n.subscription_id === subFilter);
    }

    return items;
  }, [allItems, keyword, statusFilter, protocolFilter, subFilter]);

  // ── Summary computations (from all items, not filtered) ──
  const summary = React.useMemo(() => {
    const total = allItems.length;
    const online = allItems.filter((n) => n.reachable).length;
    const offline = total - online;

    const reachable = allItems.filter(
      (n) => n.reachable && n.latency_ms > 0,
    );
    const avgLatency =
      reachable.length > 0
        ? Math.round(
            reachable.reduce((acc, n) => acc + n.latency_ms, 0) /
              reachable.length,
          )
        : 0;

    // Protocol distribution
    const protoCounts: Record<string, number> = {};
    for (const n of allItems) {
      protoCounts[n.protocol] = (protoCounts[n.protocol] ?? 0) + 1;
    }
    // Sort by count desc, format as "vless 8 · trojan 6 · ..."
    const protoDistribution = Object.entries(protoCounts)
      .sort((a, b) => b[1] - a[1])
      .map(([p, c]) => `${p} ${c}`)
      .join(" · ");

    return { total, online, offline, avgLatency, protoDistribution };
  }, [allItems]);

  // ── Unique protocols in current data (for filter chips) ──
  const availableProtocols = React.useMemo(() => {
    const set = new Set(allItems.map((n) => n.protocol));
    return PROTOCOL_OPTIONS.filter((p) => set.has(p));
  }, [allItems]);

  // ── Handlers ──
  const onCopyURI = async (node: NodeWithLatency) => {
    try {
      const res = await copyMutation.mutateAsync(node.id);
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(res.raw_uri);
      }
      toast.success(t("node:detail.copied"));
    } catch (err) {
      handleError(err);
    }
  };

  const onDelete = async (node: NodeWithLatency) => {
    try {
      await deleteMutation.mutateAsync(node.id);
      toast.success(t("common:actions.delete"));
    } catch (err) {
      handleError(err);
    }
  };

  const openBatch = () => {
    const ids = filtered.map((n) => n.id);
    if (ids.length === 0) {
      toast.error(t("node:batch.select_some_hint"));
      return;
    }
    if (ids.length > TCPING_MAX) {
      toast.error(t("node:batch.limit_exceeded"));
      return;
    }
    setBatchOpen(true);
  };

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="flex flex-col gap-5">
      {/* ── Page header ── */}
      <header className="flex items-end justify-between">
        <div>
          <h1 className="text-[22px] font-bold tracking-tight text-[var(--color-text-primary)]">
            {t("node:title")}
          </h1>
          <p className="mt-1 text-[12px] text-[var(--color-text-tertiary)]">
            {!isLoading && !isError && allItems.length > 0
              ? t("node:subtitle_dynamic", {
                  total: summary.total,
                  online: summary.online,
                  avgLatency: summary.avgLatency,
                })
              : t("node:subtitle")}
          </p>
        </div>
        <Button variant="outline" onClick={openBatch}>
          <Activity className="mr-1.5 h-3.5 w-3.5" />
          {t("node:actions.batch_tcping")}
        </Button>
      </header>

      {/* ── Summary strip (5 cells) ── */}
      {!isLoading && !isError && allItems.length > 0 && (
        <div
          className={cn(
            "flex overflow-hidden rounded-[var(--radius-lg)]",
            "border border-[var(--color-border)]",
            "bg-[var(--color-surface)] backdrop-blur-[16px]",
          )}
        >
          <SummaryCell
            label={t("node:summary.total")}
            value={String(summary.total)}
          />
          <SummaryCell
            label={t("node:summary.online")}
            value={String(summary.online)}
            valueColor="var(--color-success)"
          />
          <SummaryCell
            label={t("node:summary.offline")}
            value={String(summary.offline)}
            valueColor="var(--color-error)"
          />
          <SummaryCell
            label={t("node:summary.avg_latency")}
            value={String(summary.avgLatency)}
            suffix={t("node:summary.ms")}
          />
          <SummaryCell
            label={t("node:summary.protocols")}
            value={summary.protoDistribution}
            small
            last
          />
        </div>
      )}

      {/* ── Toolbar: search + status chips + protocol chips + subscription dropdown ── */}
      <div className="flex flex-wrap items-center gap-1.5">
        {/* Search */}
        <div className="relative w-full max-w-[200px]">
          <Search className="pointer-events-none absolute left-2 top-1/2 h-3 w-3 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("node:filters.search_placeholder")}
            className="h-7 pl-7 text-[11px]"
          />
        </div>

        {/* Status chips */}
        <FilterChip
          active={statusFilter === "all"}
          onClick={() => setStatusFilter("all")}
        >
          {t("node:filters.all")}
        </FilterChip>
        <FilterChip
          active={statusFilter === "online"}
          onClick={() => setStatusFilter("online")}
        >
          {t("node:filters.status_online")}
        </FilterChip>
        <FilterChip
          active={statusFilter === "offline"}
          onClick={() => setStatusFilter("offline")}
        >
          {t("node:filters.status_offline")}
        </FilterChip>

        {/* Separator */}
        <span className="mx-0.5 h-4 w-px bg-[var(--color-border)]" />

        {/* Protocol chips */}
        {availableProtocols.map((p) => (
          <FilterChip
            key={p}
            active={protocolFilter === p}
            onClick={() =>
              setProtocolFilter((prev) => (prev === p ? "" : p))
            }
          >
            {p}
          </FilterChip>
        ))}

        {/* Subscription dropdown */}
        {subscriptions.length > 0 && (
          <select
            value={subFilter}
            onChange={(e) => setSubFilter(e.target.value)}
            className={cn(
              "ml-auto h-7 rounded-[5px] border border-[var(--color-border-strong)]",
              "bg-[var(--color-surface-hover)] px-2 font-sans text-[10px]",
              "text-[var(--color-text-secondary)] outline-none",
            )}
          >
            <option value="">{t("node:filters.all_subscriptions")}</option>
            {subscriptions.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        )}
      </div>

      {/* ── Content ── */}
      {isLoading && <CardGridSkeleton />}

      {isError && (
        <ErrorState
          message={
            t("node:error.load_failed") +
            (error instanceof Error ? ` (${error.message})` : "")
          }
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      )}

      {/* No nodes at all */}
      {!isLoading && !isError && allItems.length === 0 && (
        <EmptyState
          icon={<Boxes />}
          title={t("node:empty.title")}
          description={t("node:empty.description")}
        />
      )}

      {/* Filter returned nothing */}
      {!isLoading &&
        !isError &&
        allItems.length > 0 &&
        filtered.length === 0 && (
          <EmptyState
            icon={<Search />}
            title={t("node:filters.no_match_title")}
            description={t("node:filters.no_match_desc")}
          />
        )}

      {/* Card grid */}
      {!isLoading && !isError && filtered.length > 0 && (
        <div
          className="grid gap-2.5"
          style={{
            gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))",
          }}
        >
          {filtered.map((node) => (
            <NodeCard
              key={node.id}
              node={node}
              onCopyURI={onCopyURI}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}

      {/* ── Batch TCPing dialog ── */}
      <BatchTCPingDialog
        open={batchOpen}
        nodeIds={filtered.map((n) => n.id).slice(0, TCPING_MAX)}
        onClose={() => setBatchOpen(false)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function SummaryCell({
  label,
  value,
  valueColor,
  suffix,
  small,
  last,
}: {
  label: string;
  value: string;
  valueColor?: string;
  suffix?: string;
  small?: boolean;
  last?: boolean;
}) {
  return (
    <div
      className={cn(
        "flex flex-1 flex-col gap-0.5 px-4 py-3",
        !last && "border-r border-[var(--color-border)]",
      )}
    >
      <span className="text-[9px] font-semibold uppercase tracking-wider text-[var(--color-text-tertiary)]">
        {label}
      </span>
      <span
        className={cn(
          "font-bold tabular-nums tracking-tight",
          small ? "text-[12px]" : "text-[18px]",
        )}
        style={valueColor ? { color: valueColor } : undefined}
      >
        {value}
        {suffix && (
          <span className="ml-0.5 text-[10px] font-normal text-[var(--color-text-tertiary)]">
            {suffix}
          </span>
        )}
      </span>
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
        "rounded-[5px] border px-2 py-1 text-[10px] font-medium transition-all duration-100",
        active
          ? "border-[var(--color-primary)] bg-[var(--color-primary-soft)] text-[var(--color-primary)]"
          : "border-[var(--color-border)] bg-transparent text-[var(--color-text-tertiary)] hover:bg-[var(--color-surface-hover)] hover:text-[var(--color-text-primary)]",
      )}
    >
      {children}
    </button>
  );
}

function CardGridSkeleton() {
  return (
    <div
      className="grid gap-2.5"
      style={{
        gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))",
      }}
    >
      {Array.from({ length: 12 }).map((_, i) => (
        <div
          key={i}
          className="flex flex-col gap-2 rounded-[10px] border border-[var(--color-border)] bg-[var(--color-surface-hover)] p-3.5"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5">
              <Skeleton className="h-[7px] w-[7px] rounded-full" />
              <Skeleton className="h-3.5 w-24" />
            </div>
            <Skeleton className="h-3.5 w-10" />
          </div>
          <Skeleton className="ml-4 h-3 w-32" />
          <div className="ml-4 flex items-center justify-between">
            <div className="flex gap-1">
              <Skeleton className="h-3 w-8" />
              <Skeleton className="h-3 w-8" />
            </div>
            <Skeleton className="h-3.5 w-12" />
          </div>
        </div>
      ))}
    </div>
  );
}
