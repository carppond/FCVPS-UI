import * as React from "react";
import { useTranslation } from "react-i18next";
import { RefreshCw, Tag, Settings2, Trash2, X } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { BatchTagsDialog } from "@/components/subscription/batch-tags-dialog";
import { BatchConfigDialog } from "@/components/subscription/batch-config-dialog";
import {
  useBatchDeleteSubscriptionsMutation,
  useBatchSyncSubscriptionsMutation,
  useBatchTagsSubscriptionsMutation,
  useBatchUpdateSubscriptionsMutation,
} from "@/api/subscription-batch";
import type { SubscriptionBatchResult } from "@/types/api";

interface BatchActionBarProps {
  selectedIds: string[];
  /** Clears the selection (and exits selection mode in the parent). */
  onClear: () => void;
}

/** Floating action bar shown while subscriptions are selected. Owns the four
 *  batch mutations plus the tags / config / delete-confirm dialogs. */
export function BatchActionBar({ selectedIds, onClear }: BatchActionBarProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const [tagsOpen, setTagsOpen] = React.useState(false);
  const [configOpen, setConfigOpen] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);

  const syncM = useBatchSyncSubscriptionsMutation();
  const deleteM = useBatchDeleteSubscriptionsMutation();
  const tagsM = useBatchTagsSubscriptionsMutation();
  const updateM = useBatchUpdateSubscriptionsMutation();

  const count = selectedIds.length;
  const busy =
    syncM.isPending || deleteM.isPending || tagsM.isPending || updateM.isPending;

  const summarize = (res: SubscriptionBatchResult) => {
    if (res.failed_count === 0) {
      toast.success(t("subscription:batch.done", { ok: res.succeeded_count }));
    } else {
      toast.message(
        t("subscription:batch.partial", {
          ok: res.succeeded_count,
          fail: res.failed_count,
        }),
      );
    }
  };

  const run = async (p: Promise<SubscriptionBatchResult>, after?: () => void) => {
    try {
      summarize(await p);
      after?.();
      onClear();
    } catch (err) {
      handleError(err);
    }
  };

  if (count === 0) return null;

  return (
    <>
      <div className="pointer-events-none fixed inset-x-0 bottom-4 z-40 flex justify-center px-4">
        <div className="pointer-events-auto flex flex-wrap items-center gap-2 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-2.5 shadow-lg backdrop-blur-xl">
          <span className="mr-1 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            {t("subscription:batch.selected", { count })}
          </span>
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => run(syncM.mutateAsync(selectedIds))}
          >
            <RefreshCw className="size-3.5" />
            {t("subscription:batch.actions.sync")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => setTagsOpen(true)}
          >
            <Tag className="size-3.5" />
            {t("subscription:batch.actions.tags")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => setConfigOpen(true)}
          >
            <Settings2 className="size-3.5" />
            {t("subscription:batch.actions.config")}
          </Button>
          <Button
            size="sm"
            variant="destructive"
            disabled={busy}
            onClick={() => setDeleteOpen(true)}
          >
            <Trash2 className="size-3.5" />
            {t("subscription:batch.actions.delete")}
          </Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={onClear}>
            <X className="size-3.5" />
            {t("subscription:batch.actions.cancel")}
          </Button>
        </div>
      </div>

      <BatchTagsDialog
        open={tagsOpen}
        count={count}
        pending={tagsM.isPending}
        onClose={() => setTagsOpen(false)}
        onSubmit={(add, remove) =>
          run(tagsM.mutateAsync({ ids: selectedIds, add, remove }), () =>
            setTagsOpen(false),
          )
        }
      />

      <BatchConfigDialog
        open={configOpen}
        count={count}
        pending={updateM.isPending}
        onClose={() => setConfigOpen(false)}
        onSubmit={(payload) =>
          run(updateM.mutateAsync({ ids: selectedIds, ...payload }), () =>
            setConfigOpen(false),
          )
        }
      />

      <Dialog open={deleteOpen} onOpenChange={(o) => !o && setDeleteOpen(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>
              {t("subscription:batch.delete_confirm.title")}
            </DialogTitle>
            <DialogDescription>
              {t("subscription:batch.delete_confirm.description", { count })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteOpen(false)}
              disabled={deleteM.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={deleteM.isPending}
              onClick={() =>
                run(deleteM.mutateAsync(selectedIds), () => setDeleteOpen(false))
              }
            >
              {t("subscription:batch.delete_confirm.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
