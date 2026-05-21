import * as React from "react";
import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Plus, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { useDebounce } from "@/hooks/use-debounce";
import { useEventStream, type SSEHandlers } from "@/hooks/use-event-stream";
import { AgentList } from "@/components/agent/agent-list";
import { AgentCreateDialog } from "@/components/agent/agent-create-dialog";
import {
  useDeleteAgentMutation,
  useRotateTokenMutation,
  useSendCommandMutation,
} from "@/api/agent";
import i18n from "@/lib/i18n";
import agentZh from "@/locales/zh-CN/agent.json";
import agentEn from "@/locales/en/agent.json";
import agentJa from "@/locales/ja/agent.json";
import agentKo from "@/locales/ko/agent.json";
import type {
  AgentListItem,
  AgentStatus,
  RotateTokenResponse,
  SSEAgentStatusPayload,
} from "@/types/api";
import { Copy } from "lucide-react";

// Lazily register the agent i18n namespace before the route mounts. Mirrors
// the pattern used by /nodes and /subscriptions so the first-screen bundle
// stays small.
function ensureAgentNamespace() {
  if (!i18n.hasResourceBundle("zh-CN", "agent")) {
    i18n.addResourceBundle("zh-CN", "agent", agentZh, true, true);
    i18n.addResourceBundle("en", "agent", agentEn, true, true);
    i18n.addResourceBundle("ja", "agent", agentJa, true, true);
    i18n.addResourceBundle("ko", "agent", agentKo, true, true);
  }
}

export const Route = createFileRoute("/_authed/agents/")({
  beforeLoad: () => {
    ensureAgentNamespace();
  },
  component: AgentsPage,
});

function AgentsPage() {
  const { t } = useTranslation(["agent", "common"]);
  const { handle: handleError } = useApiError();

  const [searchInput, setSearchInput] = React.useState("");
  const keyword = useDebounce(searchInput, 300);
  const [createOpen, setCreateOpen] = React.useState(false);

  // SSE-driven status overrides. The map is keyed by agent_id so we can patch
  // a single row's badge in real time without forcing a full list refetch.
  const [statusOverrides, setStatusOverrides] = React.useState<
    Record<string, AgentStatus>
  >({});

  // Action-target state for the three confirmation dialogs (rotate / command /
  // delete). Keeping a single "selected" reference avoids stale targets when
  // multiple dropdown menus are clicked in quick succession.
  const [rotateTarget, setRotateTarget] =
    React.useState<AgentListItem | null>(null);
  const [rotateResult, setRotateResult] =
    React.useState<RotateTokenResponse | null>(null);
  const [commandTarget, setCommandTarget] =
    React.useState<AgentListItem | null>(null);
  const [deleteTarget, setDeleteTarget] =
    React.useState<AgentListItem | null>(null);

  const rotate = useRotateTokenMutation();
  const sendCmd = useSendCommandMutation();
  const del = useDeleteAgentMutation();

  const handlers = React.useMemo<SSEHandlers>(
    () => ({
      // The contract emits "agent_status"; tolerate the "agent_status_change"
      // alias documented in some legacy notes so any hub variant works.
      agent_status: (payload: unknown) => {
        const p = payload as SSEAgentStatusPayload | null;
        if (!p?.agent_id) return;
        setStatusOverrides((prev) => ({ ...prev, [p.agent_id]: p.status }));
      },
      agent_status_change: (payload: unknown) => {
        const p = payload as SSEAgentStatusPayload | null;
        if (!p?.agent_id) return;
        setStatusOverrides((prev) => ({ ...prev, [p.agent_id]: p.status }));
      },
    }),
    [],
  );
  useEventStream("/api/notify/stream", handlers);

  // ── action handlers ───────────────────────────────────────────────────────

  const onRotateConfirm = async () => {
    if (!rotateTarget) return;
    try {
      const res = await rotate.mutateAsync(rotateTarget.id);
      setRotateResult(res);
    } catch (err) {
      handleError(err);
    }
  };

  const onDeleteConfirm = async () => {
    if (!deleteTarget) return;
    try {
      await del.mutateAsync(deleteTarget.id);
      toast.success(t("common:actions.delete"));
      setDeleteTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  const onCommandConfirm = async () => {
    if (!commandTarget) return;
    try {
      const res = await sendCmd.mutateAsync({
        id: commandTarget.id,
        cmd: "restart",
      });
      toast.success(t("agent:command_dialog.queued", { id: res.cmd_id }));
      setCommandTarget(null);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex flex-col gap-2">
        <h1 className="text-[var(--font-size-2xl)] font-semibold tracking-tight text-[var(--color-text-primary)]">
          {t("agent:title")}
        </h1>
        <p className="text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {t("agent:subtitle")}
        </p>
      </header>

      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-[16rem] flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-tertiary)]" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("agent:filters.search_placeholder")}
            className="pl-9"
          />
        </div>
        <Button onClick={() => setCreateOpen(true)} className="ml-auto">
          <Plus className="h-4 w-4" />
          {t("agent:actions.create")}
        </Button>
      </div>

      <AgentList
        params={{ keyword }}
        statusOverrides={statusOverrides}
        onRotateToken={(a) => {
          setRotateResult(null);
          setRotateTarget(a);
        }}
        onSendCommand={(a) => setCommandTarget(a)}
        onDelete={(a) => setDeleteTarget(a)}
      />

      <AgentCreateDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
      />

      <RotateTokenDialog
        target={rotateTarget}
        result={rotateResult}
        pending={rotate.isPending}
        onConfirm={onRotateConfirm}
        onClose={() => {
          setRotateTarget(null);
          setRotateResult(null);
        }}
      />

      <CommandDialog
        target={commandTarget}
        pending={sendCmd.isPending}
        onConfirm={onCommandConfirm}
        onClose={() => setCommandTarget(null)}
      />

      <DeleteAgentDialog
        target={deleteTarget}
        pending={del.isPending}
        onConfirm={onDeleteConfirm}
        onClose={() => setDeleteTarget(null)}
      />
    </div>
  );
}

// ── Reusable confirmation dialogs ───────────────────────────────────────────

function RotateTokenDialog({
  target,
  result,
  pending,
  onConfirm,
  onClose,
}: {
  target: AgentListItem | null;
  result: RotateTokenResponse | null;
  pending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation(["agent", "common"]);
  const copy = async (text: string) => {
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard) {
        await navigator.clipboard.writeText(text);
      }
      toast.success(t("agent:wizard.copied"));
    } catch {
      /* ignore — toast already surfaced via global handler */
    }
  };

  return (
    <Dialog open={Boolean(target)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("agent:rotate_dialog.title")}</DialogTitle>
          <DialogDescription>
            {t("agent:rotate_dialog.description")}
          </DialogDescription>
        </DialogHeader>
        {result ? (
          <div className="flex flex-col gap-2">
            <Label>{t("agent:rotate_dialog.new_token_label")}</Label>
            <div className="flex items-stretch gap-2">
              <code className="flex-1 overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]">
                {result.token}
              </code>
              <Button
                variant="outline"
                size="icon"
                onClick={() => copy(result.token)}
                aria-label={t("agent:actions.copy_token")}
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>
        ) : null}
        <DialogFooter>
          {result ? (
            <Button onClick={onClose}>{t("common:actions.close")}</Button>
          ) : (
            <>
              <Button variant="outline" onClick={onClose}>
                {t("common:actions.cancel")}
              </Button>
              <Button onClick={onConfirm} disabled={pending}>
                {pending
                  ? t("common:loading")
                  : t("agent:rotate_dialog.confirm")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function CommandDialog({
  target,
  pending,
  onConfirm,
  onClose,
}: {
  target: AgentListItem | null;
  pending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation(["agent", "common"]);
  return (
    <Dialog open={Boolean(target)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("agent:command_dialog.title")}</DialogTitle>
          <DialogDescription>
            {target
              ? t("agent:command_dialog.description", { name: target.name })
              : ""}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2">
          <Label>{t("agent:command_dialog.cmd_label")}</Label>
          <code className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)]">
            restart
          </code>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t("common:actions.cancel")}
          </Button>
          <Button onClick={onConfirm} disabled={pending}>
            {pending ? t("common:loading") : t("agent:command_dialog.submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function DeleteAgentDialog({
  target,
  pending,
  onConfirm,
  onClose,
}: {
  target: AgentListItem | null;
  pending: boolean;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation(["agent", "common"]);
  return (
    <Dialog open={Boolean(target)} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("agent:delete_dialog.title")}</DialogTitle>
          <DialogDescription>
            {target
              ? t("agent:delete_dialog.description", { name: target.name })
              : ""}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            variant="destructive"
            onClick={onConfirm}
            disabled={pending}
          >
            {pending ? t("common:loading") : t("agent:delete_dialog.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
