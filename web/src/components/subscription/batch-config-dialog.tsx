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
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";

type InsecureChoice = "keep" | "on" | "off";

interface BatchConfigPayload {
  sync_interval?: number;
  allow_insecure?: boolean;
}

interface BatchConfigDialogProps {
  open: boolean;
  count: number;
  pending: boolean;
  onClose: () => void;
  onSubmit: (payload: BatchConfigPayload) => void;
}

/** Batch-edit the shared fields (sync interval, allow_insecure). Untouched
 *  controls are omitted from the payload so they stay unchanged per item. */
export function BatchConfigDialog({
  open,
  count,
  pending,
  onClose,
  onSubmit,
}: BatchConfigDialogProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const [intervalEnabled, setIntervalEnabled] = React.useState(false);
  const [interval, setIntervalValue] = React.useState("");
  const [insecure, setInsecure] = React.useState<InsecureChoice>("keep");

  React.useEffect(() => {
    if (open) {
      setIntervalEnabled(false);
      setIntervalValue("");
      setInsecure("keep");
    }
  }, [open]);

  const intervalNum = Number(interval);
  const intervalValid =
    intervalEnabled && Number.isFinite(intervalNum) && intervalNum > 0;
  const canSubmit = intervalValid || insecure !== "keep";

  const submit = () => {
    const payload: BatchConfigPayload = {};
    if (intervalValid) payload.sync_interval = Math.floor(intervalNum);
    if (insecure !== "keep") payload.allow_insecure = insecure === "on";
    onSubmit(payload);
  };

  const choices: InsecureChoice[] = ["keep", "on", "off"];

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("subscription:batch.config.title")}</DialogTitle>
          <DialogDescription>
            {t("subscription:batch.selected", { count })}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-5">
          {/* Sync interval */}
          <div className="flex flex-col gap-2">
            <label className="flex items-center gap-2 text-[var(--font-size-sm)]">
              <Checkbox
                checked={intervalEnabled}
                onCheckedChange={(c) => setIntervalEnabled(c === true)}
              />
              {t("subscription:batch.config.interval_enable")}
            </label>
            <div className="flex items-center gap-2">
              <Input
                type="number"
                min={1}
                value={interval}
                disabled={!intervalEnabled}
                onChange={(e) => setIntervalValue(e.target.value)}
                className="max-w-[160px]"
              />
              <span className="text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
                {t("subscription:batch.config.interval_unit")}
              </span>
            </div>
          </div>

          {/* allow_insecure tri-state */}
          <div className="flex flex-col gap-2">
            <Label>{t("subscription:batch.config.insecure_label")}</Label>
            <div className="flex gap-2">
              {choices.map((c) => (
                <Button
                  key={c}
                  type="button"
                  size="sm"
                  variant={insecure === c ? "default" : "outline"}
                  onClick={() => setInsecure(c)}
                  className={cn(insecure === c && "pointer-events-none")}
                >
                  {t(`subscription:batch.config.insecure_${c}`)}
                </Button>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={pending}>
            {t("common:actions.cancel")}
          </Button>
          <Button onClick={submit} disabled={!canSubmit || pending}>
            {t("subscription:batch.config.submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
