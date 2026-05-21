import * as React from "react";
import { useTranslation } from "react-i18next";
import { CheckCircle2, Loader2, XCircle } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useApiError } from "@/hooks/use-api-error";
import { useTCPingBatchMutation } from "@/api/node";
import type { TCPingResult } from "@/types/api";

/**
 * Modal that drives a batch TCPing run.
 *
 * Lifecycle:
 *  1. opens with the supplied `nodeIds` — caller is responsible for clamping
 *     to ≤ 200 IDs (enforced in the parent UI; the backend rejects the
 *     overflow as well).
 *  2. fires the mutation when the dialog mounts; a spinner + count are
 *     displayed until the response lands.
 *  3. summarises reachable / unreachable counts; the user dismisses via close.
 */
interface BatchTCPingDialogProps {
  open: boolean;
  nodeIds: string[];
  onClose: () => void;
}

export function BatchTCPingDialog({ open, nodeIds, onClose }: BatchTCPingDialogProps) {
  const { t } = useTranslation("node");
  const { handle: handleError } = useApiError();
  const mutation = useTCPingBatchMutation();

  const [results, setResults] = React.useState<TCPingResult[]>([]);

  React.useEffect(() => {
    if (!open) return;
    setResults([]);
    if (nodeIds.length === 0) return;
    let cancelled = false;
    (async () => {
      try {
        const resp = await mutation.mutateAsync({ node_ids: nodeIds });
        if (!cancelled) setResults(resp.results);
      } catch (err) {
        if (!cancelled) handleError(err);
      }
    })();
    return () => {
      cancelled = true;
    };
    // We intentionally only re-run when the dialog re-opens with new ids.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, nodeIds.join(",")]);

  const total = results.length;
  const reachable = results.filter((r) => r.reachable).length;
  const unreachable = total - reachable;

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("batch.dialog_title")}</DialogTitle>
          <DialogDescription>
            {nodeIds.length > 0
              ? t("batch.running", { done: total, total: nodeIds.length })
              : t("batch.select_some_hint")}
          </DialogDescription>
        </DialogHeader>

        {mutation.isPending ? (
          <div className="flex flex-col items-center gap-3 py-8 text-[var(--color-text-secondary)]">
            <Loader2 className="h-8 w-8 animate-spin text-[var(--color-primary)]" />
            <p className="text-[var(--font-size-sm)] tabular-nums">
              {t("batch.running", { done: 0, total: nodeIds.length })}
            </p>
          </div>
        ) : total > 0 ? (
          <div className="grid grid-cols-2 gap-3 py-4 text-center text-[var(--font-size-sm)]">
            <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-success-bg)] p-4">
              <CheckCircle2 className="mx-auto mb-2 h-6 w-6 text-[var(--color-success)]" />
              <p className="text-[var(--color-text-primary)] tabular-nums">
                {t("batch.summary_reachable", { count: reachable })}
              </p>
            </div>
            <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-error-bg)] p-4">
              <XCircle className="mx-auto mb-2 h-6 w-6 text-[var(--color-error)]" />
              <p className="text-[var(--color-text-primary)] tabular-nums">
                {t("batch.summary_unreachable", { count: unreachable })}
              </p>
            </div>
          </div>
        ) : null}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t("actions.clear_selection")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
