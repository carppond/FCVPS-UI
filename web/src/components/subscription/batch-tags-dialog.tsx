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
import { Button } from "@/components/ui/button";
import { SubTagInput } from "@/components/subscription/sub-tag-input";

interface BatchTagsDialogProps {
  open: boolean;
  count: number;
  pending: boolean;
  onClose: () => void;
  onSubmit: (add: string[], remove: string[]) => void;
}

/** Batch add/remove tags across the selected subscriptions. */
export function BatchTagsDialog({
  open,
  count,
  pending,
  onClose,
  onSubmit,
}: BatchTagsDialogProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const [add, setAdd] = React.useState<string[]>([]);
  const [remove, setRemove] = React.useState<string[]>([]);

  React.useEffect(() => {
    if (open) {
      setAdd([]);
      setRemove([]);
    }
  }, [open]);

  const canSubmit = add.length > 0 || remove.length > 0;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("subscription:batch.tags.title")}</DialogTitle>
          <DialogDescription>
            {t("subscription:batch.selected", { count })}
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label>{t("subscription:batch.tags.add_label")}</Label>
            <SubTagInput value={add} onChange={setAdd} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>{t("subscription:batch.tags.remove_label")}</Label>
            <SubTagInput value={remove} onChange={setRemove} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={pending}>
            {t("common:actions.cancel")}
          </Button>
          <Button onClick={() => onSubmit(add, remove)} disabled={!canSubmit || pending}>
            {t("subscription:batch.tags.submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
