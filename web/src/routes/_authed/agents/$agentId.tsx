import * as React from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Copy, RotateCw, Send, Trash2 } from "lucide-react";
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
  useDeleteAgentMutation,
  useRotateTokenMutation,
  useSendCommandMutation,
} from "@/api/agent";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  AgentListItem,
  AgentMetric,
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
      await del.mutateAsync(agent.id);
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

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("agent:delete_dialog.title")}</DialogTitle>
            <DialogDescription>
              {t("agent:delete_dialog.description", { name: agent.name })}
            </DialogDescription>
          </DialogHeader>
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
