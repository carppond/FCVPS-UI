import * as React from "react";
import { useTranslation } from "react-i18next";
import { Save } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useCreateRuleSet,
  useUpdateRuleSet,
} from "@/api/rule-set";
import type {
  RuleSetBehavior,
  RuleSetFormat,
  RuleSetProvider,
} from "@/types/api";

interface RuleSetFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** When non-null, the dialog operates in edit mode. */
  ruleSet?: RuleSetProvider | null;
}

const DEFAULT_INTERVAL = 86400;

/**
 * Create / edit modal for a single rule provider. Pre-fills with the supplied
 * record (or empty defaults) and resets whenever `open` flips back to true so
 * reopening for "new" doesn't carry over the previously-edited values.
 */
export function RuleSetFormDialog({
  open,
  onOpenChange,
  ruleSet,
}: RuleSetFormDialogProps) {
  const { t } = useTranslation(["rule-set", "common"]);
  const { handle: handleError } = useApiError();
  const createMutation = useCreateRuleSet();
  const updateMutation = useUpdateRuleSet();

  const editing = Boolean(ruleSet);

  const [name, setName] = React.useState("");
  const [behavior, setBehavior] = React.useState<RuleSetBehavior>("domain");
  const [format, setFormat] = React.useState<RuleSetFormat>("mrs");
  const [url, setUrl] = React.useState("");
  const [intervalSec, setIntervalSec] = React.useState<number>(DEFAULT_INTERVAL);
  const [enabled, setEnabled] = React.useState(true);
  const [submitting, setSubmitting] = React.useState(false);

  React.useEffect(() => {
    if (!open) return;
    if (ruleSet) {
      setName(ruleSet.name);
      setBehavior(ruleSet.behavior);
      setFormat(ruleSet.format);
      setUrl(ruleSet.url);
      setIntervalSec(ruleSet.interval_seconds || DEFAULT_INTERVAL);
      setEnabled(ruleSet.enabled);
    } else {
      setName("");
      setBehavior("domain");
      setFormat("mrs");
      setUrl("");
      setIntervalSec(DEFAULT_INTERVAL);
      setEnabled(true);
    }
  }, [open, ruleSet]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      if (ruleSet) {
        await updateMutation.mutateAsync({
          id: ruleSet.id,
          payload: {
            name: name.trim(),
            behavior,
            format,
            url: url.trim(),
            interval_seconds: intervalSec,
            enabled,
          },
        });
        toast.success(t("rule-set:toast.update_ok"));
      } else {
        await createMutation.mutateAsync({
          name: name.trim(),
          behavior,
          format,
          url: url.trim(),
          interval_seconds: intervalSec,
          enabled,
        });
        toast.success(t("rule-set:toast.create_ok"));
      }
      onOpenChange(false);
    } catch (err) {
      handleError(err);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !submitting && onOpenChange(o)}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>
            {editing
              ? t("rule-set:form.edit_title")
              : t("rule-set:form.new_title")}
          </DialogTitle>
        </DialogHeader>

        <form className="flex flex-col gap-4" onSubmit={onSubmit}>
          <section className="flex flex-col gap-2">
            <Label htmlFor="rs-name">{t("rule-set:form.name_label")}</Label>
            <Input
              id="rs-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t("rule-set:form.name_placeholder")}
              required
              autoComplete="off"
            />
          </section>

          <section className="flex flex-col gap-2">
            <Label>{t("rule-set:form.behavior_label")}</Label>
            <Tabs
              value={behavior}
              onValueChange={(v) => setBehavior(v as RuleSetBehavior)}
            >
              <TabsList className="w-full">
                <TabsTrigger value="domain" className="flex-1">
                  {t("rule-set:behavior.domain")}
                </TabsTrigger>
                <TabsTrigger value="ipcidr" className="flex-1">
                  {t("rule-set:behavior.ipcidr")}
                </TabsTrigger>
                <TabsTrigger value="classical" className="flex-1">
                  {t("rule-set:behavior.classical")}
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </section>

          <section className="flex flex-col gap-2">
            <Label>{t("rule-set:form.format_label")}</Label>
            <Tabs
              value={format}
              onValueChange={(v) => setFormat(v as RuleSetFormat)}
            >
              <TabsList className="w-full">
                <TabsTrigger value="mrs" className="flex-1">
                  {t("rule-set:format.mrs")}
                </TabsTrigger>
                <TabsTrigger value="yaml" className="flex-1">
                  {t("rule-set:format.yaml")}
                </TabsTrigger>
                <TabsTrigger value="text" className="flex-1">
                  {t("rule-set:format.text")}
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </section>

          <section className="flex flex-col gap-2">
            <Label htmlFor="rs-url">{t("rule-set:form.url_label")}</Label>
            <Input
              id="rs-url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder={t("rule-set:form.url_placeholder")}
              type="url"
              required
              autoComplete="off"
            />
          </section>

          <section className="flex flex-col gap-2">
            <Label htmlFor="rs-interval">
              {t("rule-set:form.interval_label")}
            </Label>
            <Input
              id="rs-interval"
              type="number"
              min={60}
              value={intervalSec}
              onChange={(e) =>
                setIntervalSec(Math.max(60, Number(e.target.value) || DEFAULT_INTERVAL))
              }
            />
            <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("rule-set:form.interval_hint")}
            </p>
          </section>

          <label className={cn(
            "flex cursor-pointer items-center gap-2",
            "text-[var(--font-size-sm)] text-[var(--color-text-secondary)]",
          )}>
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="h-4 w-4 rounded border-[var(--color-border-strong)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]"
            />
            {t("rule-set:form.enabled_label")}
          </label>

          <DialogFooter className="mt-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onOpenChange(false)}
              disabled={submitting}
            >
              {t("rule-set:form.cancel")}
            </Button>
            <Button type="submit" size="sm" disabled={submitting}>
              <Save className="h-3.5 w-3.5" />
              {t("rule-set:form.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
