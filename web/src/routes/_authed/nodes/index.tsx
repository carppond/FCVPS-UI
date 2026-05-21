import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Activity, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import { NodeTable } from "@/components/nodes/node-table";
import { BatchTCPingDialog } from "@/components/nodes/batch-tcping-dialog";
import {
  useCopyNodeURIMutation,
  useDeleteNodeMutation,
  type ListNodesParams,
} from "@/api/node";
import i18n from "@/lib/i18n";
import nodeZh from "@/locales/zh-CN/node.json";
import nodeEn from "@/locales/en/node.json";
import nodeJa from "@/locales/ja/node.json";
import nodeKo from "@/locales/ko/node.json";
import type { NodeWithLatency } from "@/types/api";

// Lazy-register the "node" namespace before the route mounts. Mirrors the
// pattern used by /admin/users so first-screen bundles stay slim.
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

const TCPING_MAX = 200;

function NodesPage() {
  const { t } = useTranslation(["node", "common"]);
  const { handle: handleError } = useApiError();

  const [searchInput, setSearchInput] = React.useState("");
  const search = useDebounce(searchInput, 300);
  const [protocol, setProtocol] = React.useState("");
  const [tag, setTag] = React.useState("");
  const [sort, setSort] = React.useState<ListNodesParams["sort"]>("created_desc");

  const [selected, setSelected] = React.useState<string[]>([]);
  const [batchOpen, setBatchOpen] = React.useState(false);

  const copyMutation = useCopyNodeURIMutation();
  const deleteMutation = useDeleteNodeMutation();

  const toggleSelect = (id: string, next: boolean) => {
    setSelected((prev) =>
      next ? [...new Set([...prev, id])] : prev.filter((v) => v !== id),
    );
  };
  const toggleSelectAll = (ids: string[], next: boolean) => {
    setSelected((prev) =>
      next ? [...new Set([...prev, ...ids])] : prev.filter((v) => !ids.includes(v)),
    );
  };

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
      setSelected((prev) => prev.filter((v) => v !== node.id));
    } catch (err) {
      handleError(err);
    }
  };

  const openBatch = () => {
    if (selected.length === 0) {
      toast.error(t("node:batch.select_some_hint"));
      return;
    }
    if (selected.length > TCPING_MAX) {
      toast.error(t("node:batch.limit_exceeded"));
      return;
    }
    setBatchOpen(true);
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex flex-col gap-2">
        <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
          {t("node:title")}
        </h1>
        <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("node:subtitle")}
        </p>
      </header>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-[16rem] flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("node:filters.search_placeholder")}
            className="pl-9"
          />
        </div>

        <select
          value={protocol}
          onChange={(e) => setProtocol(e.target.value)}
          className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
          aria-label={t("node:filters.all_protocols")}
        >
          <option value="">{t("node:filters.all_protocols")}</option>
          {["vmess", "vless", "ss", "ssr", "trojan", "hysteria", "hysteria2", "tuic", "wireguard", "anytls", "socks5", "naive"].map(
            (p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ),
          )}
        </select>

        <Input
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          placeholder={t("node:filters.all_tags")}
          className="w-32"
        />

        <select
          value={sort}
          onChange={(e) => setSort(e.target.value as ListNodesParams["sort"])}
          className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
          aria-label={t("node:filters.sort_label")}
        >
          <option value="created_desc">{t("node:filters.sort_created_desc")}</option>
          <option value="created_asc">{t("node:filters.sort_created_asc")}</option>
          <option value="latency_asc">{t("node:filters.sort_latency_asc")}</option>
          <option value="latency_desc">{t("node:filters.sort_latency_desc")}</option>
        </select>

        <Button
          variant="outline"
          onClick={openBatch}
          disabled={selected.length === 0}
          className="ml-auto"
        >
          <Activity className="h-4 w-4" />
          {t("node:actions.tcping_selected")} ({selected.length})
        </Button>
      </div>

      <NodeTable
        params={{ search, protocol, tag, sort }}
        selected={selected}
        onToggleSelect={toggleSelect}
        onToggleSelectAll={toggleSelectAll}
        onCopyURI={onCopyURI}
        onDelete={onDelete}
      />

      <BatchTCPingDialog
        open={batchOpen}
        nodeIds={selected}
        onClose={() => setBatchOpen(false)}
      />
    </div>
  );
}
