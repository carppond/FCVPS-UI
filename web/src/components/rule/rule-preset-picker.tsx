import * as React from "react";
import { useTranslation } from "react-i18next";
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
import { toast } from "@/components/ui/toast";
import { useCreateRuleMutation, useRuleTemplatesQuery } from "@/api/rule";

interface RulePresetPickerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAdded?: () => void;
}

/**
 * Multi-select picker over the built-in rule templates. Each checked template
 * is created as a new rule via one POST (client-side loop, mirroring the
 * rule-set / proxy-group preset pickers); a partial failure is surfaced rather
 * than aborting the batch.
 */
export function RulePresetPicker({
  open,
  onOpenChange,
  onAdded,
}: RulePresetPickerProps) {
  const { t } = useTranslation(["rule", "common"]);
  const { data: templates } = useRuleTemplatesQuery();
  const createMutation = useCreateRuleMutation();
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [busy, setBusy] = React.useState(false);

  React.useEffect(() => {
    if (open) setSelected(new Set());
  }, [open]);

  const list = templates ?? [];
  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const add = async () => {
    const picks = list.filter((tpl) => selected.has(tpl.id));
    if (picks.length === 0) return;
    setBusy(true);
    const results = await Promise.allSettled(
      picks.map((tpl) =>
        createMutation.mutateAsync({
          name: tpl.name,
          type: tpl.rule_type ?? "rules",
          mode: tpl.mode ?? "append",
          content: tpl.content,
          enabled: true,
        }),
      ),
    );
    setBusy(false);
    const ok = results.filter((r) => r.status === "fulfilled").length;
    const fail = results.length - ok;
    if (fail === 0) toast.success(t("rule:batch.added", { count: ok }));
    else toast.message(t("rule:batch.added_partial", { ok, fail }));
    onOpenChange(false);
    onAdded?.();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("rule:batch.preset_title")}</DialogTitle>
          <DialogDescription>{t("rule:batch.preset_desc")}</DialogDescription>
        </DialogHeader>
        <ul className="flex max-h-[55vh] flex-col gap-1 overflow-y-auto">
          {list.map((tpl) => (
            <li key={tpl.id}>
              <label className="flex cursor-pointer items-start gap-2.5 rounded-[var(--radius-md)] px-2 py-2 hover:bg-[var(--color-surface-hover)]">
                <Checkbox
                  checked={selected.has(tpl.id)}
                  onCheckedChange={() => toggle(tpl.id)}
                  className="mt-0.5"
                />
                <span className="min-w-0">
                  <span className="block text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                    {tpl.emoji ? `${tpl.emoji} ` : ""}
                    {tpl.name}
                  </span>
                  <span className="block text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                    {tpl.description}
                  </span>
                </span>
              </label>
            </li>
          ))}
        </ul>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={busy}
          >
            {t("common:actions.cancel")}
          </Button>
          <Button onClick={add} disabled={busy || selected.size === 0}>
            {t("rule:batch.preset_add", { count: selected.size })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
