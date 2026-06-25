import * as React from "react";
import { useTranslation } from "react-i18next";
import { CheckCircle2, CircleSlash, LayoutTemplate, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { RulePresetPicker } from "@/components/rule/rule-preset-picker";
import { useDeleteRuleMutation, useUpdateRuleMutation } from "@/api/rule";

interface RuleBatchBarProps {
  selectedIds: string[];
  /** Exits selection mode and clears the selection in the parent. */
  onExit: () => void;
}

/**
 * Floating bar shown while the rules page is in selection mode. Enable / disable
 * / delete loop the existing single-rule endpoints client-side (the list is one
 * page of ≤200), summarising success/failure; "add presets" opens the
 * multi-select template picker. All actions are owner-scoped by those endpoints.
 */
export function RuleBatchBar({ selectedIds, onExit }: RuleBatchBarProps) {
  const { t } = useTranslation(["rule", "common"]);
  const updateMutation = useUpdateRuleMutation();
  const deleteMutation = useDeleteRuleMutation();
  const [presetOpen, setPresetOpen] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const count = selectedIds.length;

  const summarize = (results: PromiseSettledResult<unknown>[]) => {
    const ok = results.filter((r) => r.status === "fulfilled").length;
    const fail = results.length - ok;
    if (fail === 0) toast.success(t("rule:batch.done", { count: ok }));
    else toast.message(t("rule:batch.partial", { ok, fail }));
  };

  const setEnabled = async (enabled: boolean) => {
    setBusy(true);
    const results = await Promise.allSettled(
      selectedIds.map((id) =>
        updateMutation.mutateAsync({ id, payload: { enabled } }),
      ),
    );
    setBusy(false);
    summarize(results);
    onExit();
  };

  const removeAll = async () => {
    if (!confirm(t("rule:batch.delete_confirm", { count }))) return;
    setBusy(true);
    const results = await Promise.allSettled(
      selectedIds.map((id) => deleteMutation.mutateAsync(id)),
    );
    setBusy(false);
    summarize(results);
    onExit();
  };

  return (
    <>
      <div className="pointer-events-none fixed inset-x-0 bottom-4 z-40 flex justify-center px-4">
        <div className="pointer-events-auto flex flex-wrap items-center gap-2 rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-solid)] px-4 py-2.5 shadow-lg">
          <span className="mr-1 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
            {t("rule:batch.selected", { count })}
          </span>
          <Button
            size="sm"
            variant="outline"
            disabled={busy || count === 0}
            onClick={() => void setEnabled(true)}
          >
            <CheckCircle2 className="size-3.5" />
            {t("rule:batch.enable")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={busy || count === 0}
            onClick={() => void setEnabled(false)}
          >
            <CircleSlash className="size-3.5" />
            {t("rule:batch.disable")}
          </Button>
          <Button
            size="sm"
            variant="destructive"
            disabled={busy || count === 0}
            onClick={() => void removeAll()}
          >
            <Trash2 className="size-3.5" />
            {t("rule:batch.delete")}
          </Button>
          <span className="mx-1 h-4 w-px bg-[var(--color-border)]" />
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => setPresetOpen(true)}
          >
            <LayoutTemplate className="size-3.5" />
            {t("rule:batch.add_presets")}
          </Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={onExit}>
            <X className="size-3.5" />
            {t("rule:batch.cancel")}
          </Button>
        </div>
      </div>

      <RulePresetPicker
        open={presetOpen}
        onOpenChange={setPresetOpen}
        onAdded={onExit}
      />
    </>
  );
}
