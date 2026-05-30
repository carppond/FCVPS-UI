import * as React from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  ArrowLeft,
  Copy,
  Gauge,
  RefreshCw,
  RotateCw,
  Send,
  Trash2,
} from "lucide-react";
import i18n from "@/lib/i18n";
import agentZh from "@/locales/zh-CN/agent.json";
import agentEn from "@/locales/en/agent.json";
import agentJa from "@/locales/ja/agent.json";
import agentKo from "@/locales/ko/agent.json";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useEventStream, type SSEHandlers } from "@/hooks/use-event-stream";
import { AgentDetailTabs } from "@/components/agent/agent-detail-tabs";
import { AgentStatusDot } from "@/components/agent/agent-status-dot";
import { AgentKindBadge } from "@/components/agent/agent-kind-badge";
import {
  useAgentQuery,
  AGENT_UNINSTALL_CMD,
  useDeleteAgentMutation,
  useRotateTokenMutation,
  useSendCommandMutation,
  useUpdateAgentMutation,
} from "@/api/agent";
import {
  useTrafficSummaryQuery,
  useRecomputeTrafficMutation,
} from "@/api/traffic";
import { formatBytes } from "@/lib/format";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  AgentListItem,
  AgentMetric,
  AgentTrafficSummary,
  RotateTokenResponse,
  SSEAgentStatusPayload,
} from "@/types/api";

function ensureAgentNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "agent")) {
    i18n.addResourceBundle("zh-CN", "agent", agentZh, true, true);
    i18n.addResourceBundle("en", "agent", agentEn, true, true);
    i18n.addResourceBundle("ja", "agent", agentJa, true, true);
    i18n.addResourceBundle("ko", "agent", agentKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/agents/$agentId")({
  beforeLoad: () => {
    ensureAgentNamespace();
  },
  component: AgentDetailPage,
});

// Cap realtime buffer so a long-lived tab can't bloat memory: at ≤2 samples
// per second the cap covers ~2 hours of history while staying responsive.
const REALTIME_BUFFER = 240;

function AgentDetailPage() {
  const { agentId } = Route.useParams();
  const { t } = useTranslation(["agent", "common"]);
  const navigate = useNavigate();
  const { handle: handleError } = useApiError();

  const { data: agent, isLoading, isError, error, refetch } =
    useAgentQuery(agentId);
  const rotate = useRotateTokenMutation();
  const sendCmd = useSendCommandMutation();
  const del = useDeleteAgentMutation();

  const [rotateOpen, setRotateOpen] = React.useState(false);
  const [rotateResult, setRotateResult] =
    React.useState<RotateTokenResponse | null>(null);
  const [commandOpen, setCommandOpen] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const [deleteUninstall, setDeleteUninstall] = React.useState(true);

  // Per-agent monthly traffic: used comes from the traffic summary (measured /
  // provider figure), limit/source from the same row. Config is set via the
  // settings dialog (manual limit + BandwagonHost creds).
  const update = useUpdateAgentMutation();
  const recompute = useRecomputeTrafficMutation();
  const { data: trafficSummary } = useTrafficSummaryQuery();
  const agentTraffic = trafficSummary?.agents.find((a) => a.agent_id === agentId);
  const onRecompute = async () => {
    try {
      await recompute.mutateAsync();
      toast.success(t("agent:traffic_card.recomputed"));
    } catch (err) {
      handleError(err);
    }
  };

  const [settingsOpen, setSettingsOpen] = React.useState(false);
  const [form, setForm] = React.useState({ name: "", limitGB: "", veid: "", key: "" });
  const openSettings = () => {
    if (!agent) return;
    setForm({
      name: agent.name,
      limitGB: agent.traffic_limit ? String(agent.traffic_limit / 1024 ** 3) : "",
      veid: agent.bwg_veid ?? "",
      key: "",
    });
    setSettingsOpen(true);
  };
  const onSettingsSave = async () => {
    if (!agent) return;
    const gb = parseFloat(form.limitGB);
    const limitBytes =
      form.limitGB.trim() && gb > 0 ? Math.round(gb * 1024 ** 3) : 0;
    try {
      await update.mutateAsync({
        id: agent.id,
        payload: {
          name: form.name.trim() || agent.name,
          traffic_limit: limitBytes,
          bwg_veid: form.veid.trim(),
          bwg_api_key: form.key.trim(),
        },
      });
      toast.success(t("common:actions.save"));
      setSettingsOpen(false);
    } catch (err) {
      handleError(err);
    }
  };

  // Realtime metric ring buffer fed by the SSE `agent_metrics` event. Kept in
  // a ref-backed state so charts always show a stable order without flapping
  // on every push.
  const [realtimeMetrics, setRealtimeMetrics] = React.useState<AgentMetric[]>(
    [],
  );

  const handlers = React.useMemo<SSEHandlers>(
    () => ({
      agent_status: (payload: unknown) => {
        const p = payload as SSEAgentStatusPayload | null;
        if (!p || p.agent_id !== agentId) return;
        // Patch the cached detail so the header status updates instantly.
        queryClient.setQueryData<AgentListItem | undefined>(
          queryKeys.agent.detail(agentId),
          (prev: AgentListItem | undefined) =>
            prev
              ? { ...prev, status: p.status, online: p.status === "online" }
              : prev,
        );
      },
      agent_status_change: (payload: unknown) => {
        const p = payload as SSEAgentStatusPayload | null;
        if (!p || p.agent_id !== agentId) return;
        queryClient.setQueryData<AgentListItem | undefined>(
          queryKeys.agent.detail(agentId),
          (prev: AgentListItem | undefined) =>
            prev
              ? { ...prev, status: p.status, online: p.status === "online" }
              : prev,
        );
      },
      agent_metrics: (payload: unknown) => {
        const m = payload as AgentMetric | null;
        if (!m || m.agent_id !== agentId) return;
        setRealtimeMetrics((prev) => {
          const next = [...prev, m];
          return next.length > REALTIME_BUFFER
            ? next.slice(next.length - REALTIME_BUFFER)
            : next;
        });
      },
    }),
    [agentId],
  );
  useEventStream("/api/notify/stream", handlers);

  // Hydrate the realtime buffer with the latest snapshot the API already
  // returned so the charts have something to show before the first SSE push.
  React.useEffect(() => {
    if (agent?.latest_metrics) {
      setRealtimeMetrics((prev) =>
        prev.length === 0 ? [agent.latest_metrics as AgentMetric] : prev,
      );
    }
  }, [agent?.latest_metrics]);

  if (isLoading) return <DetailSkeleton />;
  if (isError) {
    const errMsg = error instanceof Error ? error.message : String(error ?? "");
    return (
      <div className="p-6">
        <ErrorState
          message={t("agent:error.load_failed") + (errMsg ? ` (${errMsg})` : "")}
          onRetry={() => refetch()}
          retryLabel={t("common:actions.retry")}
        />
      </div>
    );
  }
  if (!agent) return null;

  const onRotateConfirm = async () => {
    try {
      const res = await rotate.mutateAsync(agent.id);
      setRotateResult(res);
    } catch (err) {
      handleError(err);
    }
  };

  const onCommandConfirm = async () => {
    try {
      const res = await sendCmd.mutateAsync({ id: agent.id, cmd: "restart" });
      toast.success(t("agent:command_dialog.queued", { id: res.cmd_id }));
      setCommandOpen(false);
    } catch (err) {
      handleError(err);
    }
  };

  const onDeleteConfirm = async () => {
    try {
      await del.mutateAsync({ id: agent.id, uninstall: deleteUninstall });
      toast.success(t("common:actions.delete"));
      setDeleteOpen(false);
      void navigate({ to: "/agents" });
    } catch (err) {
      handleError(err);
    }
  };

  const copy = async (text: string) => {
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(text);
      }
      toast.success(t("agent:wizard.copied"));
    } catch {
      /* ignore */
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex items-center gap-3">
        <Button variant="ghost" size="icon" asChild>
          <Link to="/agents">
            <ArrowLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex flex-1 flex-col gap-1">
          <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
            {agent.name}
          </h1>
          <div className="flex items-center gap-2">
            <AgentKindBadge kind={agent.kind} />
            <AgentStatusDot status={agent.status} withLabel />
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={openSettings}>
            <Gauge className="h-4 w-4" />
            {t("agent:actions.traffic_settings")}
          </Button>
          <Button
            variant="outline"
            onClick={() => {
              setRotateResult(null);
              setRotateOpen(true);
            }}
          >
            <RotateCw className="h-4 w-4" />
            {t("agent:actions.rotate_token")}
          </Button>
          <Button
            variant="outline"
            onClick={() => setCommandOpen(true)}
            disabled={agent.status !== "online"}
          >
            <Send className="h-4 w-4" />
            {t("agent:actions.send_command")}
          </Button>
          <Button
            variant="destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="h-4 w-4" />
            {t("agent:actions.delete")}
          </Button>
        </div>
      </header>

      {agentTraffic && (
        <MonthlyTrafficCard
          traffic={agentTraffic}
          t={t}
          onRecompute={onRecompute}
          recomputing={recompute.isPending}
        />
      )}

      <AgentDetailTabs agent={agent} realtimeMetrics={realtimeMetrics} />

      <Dialog open={rotateOpen} onOpenChange={setRotateOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("agent:rotate_dialog.title")}</DialogTitle>
            <DialogDescription>
              {t("agent:rotate_dialog.description")}
            </DialogDescription>
          </DialogHeader>
          {rotateResult ? (
            <div className="flex flex-col gap-2">
              <Label>{t("agent:rotate_dialog.new_token_label")}</Label>
              <div className="flex items-stretch gap-2">
                <code className="flex-1 overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]">
                  {rotateResult.token}
                </code>
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => copy(rotateResult.token)}
                  aria-label={t("agent:actions.copy_token")}
                >
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
            </div>
          ) : null}
          <DialogFooter>
            {rotateResult ? (
              <Button onClick={() => setRotateOpen(false)}>
                {t("common:actions.close")}
              </Button>
            ) : (
              <>
                <Button
                  variant="outline"
                  onClick={() => setRotateOpen(false)}
                >
                  {t("common:actions.cancel")}
                </Button>
                <Button onClick={onRotateConfirm} disabled={rotate.isPending}>
                  {rotate.isPending
                    ? t("common:loading")
                    : t("agent:rotate_dialog.confirm")}
                </Button>
              </>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={commandOpen} onOpenChange={setCommandOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("agent:command_dialog.title")}</DialogTitle>
            <DialogDescription>
              {t("agent:command_dialog.description", { name: agent.name })}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label>{t("agent:command_dialog.cmd_label")}</Label>
            <code className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]">
              restart
            </code>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCommandOpen(false)}>
              {t("common:actions.cancel")}
            </Button>
            <Button onClick={onCommandConfirm} disabled={sendCmd.isPending}>
              {sendCmd.isPending
                ? t("common:loading")
                : t("agent:command_dialog.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={deleteOpen}
        onOpenChange={(o) => {
          setDeleteOpen(o);
          if (o) setDeleteUninstall(true);
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("agent:delete_dialog.title")}</DialogTitle>
            <DialogDescription>
              {t("agent:delete_dialog.description", { name: agent.name })}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-primary)]">
              <Checkbox
                checked={deleteUninstall}
                onCheckedChange={(v) => setDeleteUninstall(v === true)}
              />
              {t("agent:delete_dialog.uninstall_label")}
            </label>
            <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("agent:delete_dialog.uninstall_hint")}
            </p>
            <code className="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
              {AGENT_UNINSTALL_CMD}
            </code>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={onDeleteConfirm}
              disabled={del.isPending}
            >
              {del.isPending
                ? t("common:loading")
                : t("agent:delete_dialog.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={settingsOpen} onOpenChange={setSettingsOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("agent:settings.title")}</DialogTitle>
            <DialogDescription>
              {t("agent:settings.description")}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ag-name">{t("agent:settings.name_label")}</Label>
              <Input
                id="ag-name"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="ag-limit">{t("agent:settings.limit_label")}</Label>
              <Input
                id="ag-limit"
                type="number"
                inputMode="decimal"
                min="0"
                placeholder="0"
                value={form.limitGB}
                onChange={(e) =>
                  setForm((f) => ({ ...f, limitGB: e.target.value }))
                }
              />
            </div>
            <div className="flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] p-3">
              <p className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-secondary)]">
                {t("agent:settings.bwg_section")}
              </p>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ag-veid">{t("agent:settings.bwg_veid_label")}</Label>
                <Input
                  id="ag-veid"
                  value={form.veid}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, veid: e.target.value }))
                  }
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="ag-key">{t("agent:settings.bwg_key_label")}</Label>
                <Input
                  id="ag-key"
                  type="password"
                  value={form.key}
                  placeholder={agent.has_bwg_key ? "••••••••" : ""}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, key: e.target.value }))
                  }
                />
                <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                  {t("agent:settings.bwg_key_hint")}
                </p>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSettingsOpen(false)}>
              {t("common:actions.cancel")}
            </Button>
            <Button onClick={onSettingsSave} disabled={update.isPending}>
              {update.isPending ? t("common:loading") : t("common:actions.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function MonthlyTrafficCard({
  traffic,
  t,
  onRecompute,
  recomputing,
}: {
  traffic: AgentTrafficSummary;
  t: (key: string, opts?: Record<string, unknown>) => string;
  onRecompute: () => void;
  recomputing: boolean;
}) {
  const used = traffic.total_used;
  const limit = traffic.limit ?? 0;
  const pct = limit > 0 ? Math.min(100, (used / limit) * 100) : null;
  return (
    <div className="flex flex-col gap-2 rounded-[var(--radius-xl)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
      <div className="flex items-center justify-between">
        <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-secondary)]">
          {t("agent:traffic_card.title")}
        </span>
        <div className="flex items-center gap-2">
          {traffic.source === "bandwagon" && (
            <span className="rounded-[var(--radius-sm)] bg-[var(--color-surface-hover)] px-1.5 py-0.5 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("agent:traffic_card.source_bandwagon")}
            </span>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={onRecompute}
            disabled={recomputing}
          >
            <RefreshCw
              className={`h-3.5 w-3.5${recomputing ? " animate-spin" : ""}`}
            />
            {recomputing
              ? t("common:loading")
              : t("agent:traffic_card.recompute")}
          </Button>
        </div>
      </div>
      <div className="flex items-baseline gap-2 tabular-nums">
        <span className="text-[var(--font-size-2xl)] font-bold text-[var(--color-text-primary)]">
          {formatBytes(used)}
        </span>
        <span className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {limit > 0
            ? `/ ${formatBytes(limit)} · ${Math.round(pct as number)}%`
            : t("agent:traffic_card.no_limit")}
        </span>
      </div>
      {pct !== null && (
        <div className="h-1.5 overflow-hidden rounded-[var(--radius-sm)] bg-[var(--color-surface-hover)]">
          <div
            className="h-full rounded-[var(--radius-sm)] bg-[var(--color-primary)]"
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-6 p-6">
      <Skeleton className="h-8 w-64" />
      <Skeleton className="h-4 w-40" />
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <Skeleton className="h-64 w-full" />
        <Skeleton className="h-64 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    </div>
  );
}
