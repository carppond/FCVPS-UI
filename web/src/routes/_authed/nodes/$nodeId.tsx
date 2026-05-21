import * as React from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, ClipboardCopy, Trash2 } from "lucide-react";
import i18n from "@/lib/i18n";
import nodeZh from "@/locales/zh-CN/node.json";
import nodeEn from "@/locales/en/node.json";
import nodeJa from "@/locales/ja/node.json";
import nodeKo from "@/locales/ko/node.json";

function ensureNodeNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "node")) {
    i18n.addResourceBundle("zh-CN", "node", nodeZh, true, true);
    i18n.addResourceBundle("en", "node", nodeEn, true, true);
    i18n.addResourceBundle("ja", "node", nodeJa, true, true);
    i18n.addResourceBundle("ko", "node", nodeKo, true, true);
  }
}
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { ProtocolBadge } from "@/components/nodes/protocol-badge";
import { LatencyBadge } from "@/components/nodes/latency-badge";
import { TCPingButton } from "@/components/nodes/tcping-button";
import {
  useCopyNodeURIMutation,
  useDeleteNodeMutation,
  useNodeQuery,
} from "@/api/node";
import type { NodeWithLatency } from "@/types/api";

export const Route = createFileRoute("/_authed/nodes/$nodeId")({
  beforeLoad: () => {
    ensureNodeNamespace();
  },
  component: NodeDetailPage,
});

function NodeDetailPage() {
  const { nodeId } = Route.useParams();
  const { t } = useTranslation(["node", "common"]);
  const navigate = useNavigate();
  const { handle: handleError } = useApiError();
  const { data, isLoading, isError, error, refetch } = useNodeQuery(nodeId);
  const copy = useCopyNodeURIMutation();
  const del = useDeleteNodeMutation();

  if (isLoading) return <DetailSkeleton />;
  if (isError) {
    const errMsg = error instanceof Error ? error.message : String(error ?? "");
    return (
      <div className="p-6">
        <ErrorState
          message={t("node:error.load_failed") + (errMsg ? ` (${errMsg})` : "")}
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      </div>
    );
  }
  if (!data) return null;

  const onCopy = async () => {
    try {
      const res = await copy.mutateAsync(data.id);
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(res.raw_uri);
      }
      toast.success(t("node:detail.copied"));
    } catch (err) {
      handleError(err);
    }
  };

  const onDelete = async () => {
    try {
      await del.mutateAsync(data.id);
      toast.success(t("common:actions.delete"));
      void navigate({ to: "/nodes" });
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex items-center gap-3">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/nodes">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex flex-1 flex-col gap-1">
          <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
            {data.tag || data.id.slice(0, 8)}
          </h1>
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)] font-mono">
            {data.server}:{data.port}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <TCPingButton nodeId={data.id} size="default" />
          <Button variant="outline" onClick={onCopy}>
            <ClipboardCopy className="h-4 w-4" />
            {t("node:actions.copy_uri")}
          </Button>
          <Button variant="destructive" onClick={onDelete}>
            <Trash2 className="h-4 w-4" />
            {t("node:actions.delete_node")}
          </Button>
        </div>
      </header>

      <Section title={t("node:detail.metadata_section")}>
        <MetadataGrid node={data} />
      </Section>

      <Section title={t("node:detail.raw_uri_section")}>
        {data.raw_uri ? (
          <pre className="overflow-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]">
            {data.raw_uri}
          </pre>
        ) : (
          <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {t("node:detail.no_uri")}
          </p>
        )}
      </Section>

      <Section title={t("node:detail.config_section")}>
        <pre className="overflow-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]">
          {JSON.stringify(data.parsed_config, null, 2)}
        </pre>
      </Section>
    </div>
  );
}

function MetadataGrid({ node }: { node: NodeWithLatency }) {
  const { t } = useTranslation(["node", "common"]);
  const fields: Array<{ label: string; value: React.ReactNode }> = [
    { label: t("node:columns.protocol"), value: <ProtocolBadge protocol={node.protocol} /> },
    { label: t("node:columns.server"), value: node.server },
    { label: t("node:columns.port"), value: <span className="tabular-nums">{node.port}</span> },
    { label: t("node:columns.latency"), value: <LatencyBadge latencyMs={node.latency_ms} /> },
    {
      label: t("node:columns.tags"),
      value: node.tags.length === 0 ? "—" : node.tags.join(", "),
    },
    {
      label: t("node:columns.updated_at"),
      value: new Date(node.updated_at).toLocaleString(),
    },
  ];
  return (
    <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {fields.map((f) => (
        <div
          key={f.label}
          className="flex flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3"
        >
          <dt className="text-[var(--font-size-xs)] uppercase tracking-wide text-[var(--color-text-tertiary)]">
            {f.label}
          </dt>
          <dd className="text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
            {f.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
        {title}
      </h2>
      {children}
    </section>
  );
}

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-6 p-6">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="h-4 w-40" />
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}
