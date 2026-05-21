import * as React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { LayoutTemplate, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useCreateRuleMutation,
  useUpdateRuleMutation,
} from "@/api/rule";
import { RuleTemplatesDialog } from "@/components/rule/rule-templates";
import type {
  CreateRuleRequest,
  CustomRule,
  RuleMode,
  RuleType,
  UpdateRuleRequest,
} from "@/types/api";

const NAME_MAX = 100;

interface RuleFormProps {
  /**
   * `null` ⇒ "new rule" mode (Create + reset on save).
   * An existing rule ⇒ "edit" mode (PATCH).
   */
  rule: CustomRule | null;
  /**
   * Notifies the parent that an editing session has been committed
   * (saved or cancelled). Parent uses this to clear selection / re-fetch.
   */
  onSaved?: (rule: CustomRule | null) => void;
  /**
   * Optional cancel handler. When supplied, a "cancel" button is rendered.
   */
  onCancel?: () => void;
  className?: string;
}

interface FormValues {
  name: string;
  type: RuleType;
  mode: RuleMode;
  content: string;
  enabled: boolean;
}

function buildSchema(t: (key: string) => string) {
  return z.object({
    name: z
      .string()
      .min(1, t("rule:error.create_failed"))
      .max(NAME_MAX),
    type: z.enum(["dns", "rules", "rule-providers"]),
    mode: z.enum(["replace", "prepend", "append"]),
    content: z.string(),
    enabled: z.boolean(),
  });
}

/**
 * Two-mode form (create / edit) for a single custom rule. Keeps the layout
 * vertically dense — labels + inputs only, validation surfaced inline.
 *
 * The textarea is intentionally a plain `<textarea>` (not Monaco) to keep the
 * page bundle small; users who want syntax highlighting can paste into the
 * Monaco-backed preview pane. The placeholder copy switches per type so the
 * user knows what shape of content is expected.
 */
export function RuleForm({ rule, onSaved, onCancel, className }: RuleFormProps) {
  const { t } = useTranslation(["rule", "common"]);
  const { handle: handleError } = useApiError();
  const createMutation = useCreateRuleMutation();
  const updateMutation = useUpdateRuleMutation();

  const [templateOpen, setTemplateOpen] = React.useState(false);

  const schema = React.useMemo(() => buildSchema(t), [t]);

  const defaultValues: FormValues = React.useMemo(
    () => ({
      name: rule?.name ?? "",
      type: rule?.type ?? "rules",
      mode: rule?.mode ?? "append",
      content: rule?.content ?? "",
      enabled: rule?.enabled ?? true,
    }),
    [rule],
  );

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues,
  });

  React.useEffect(() => {
    form.reset(defaultValues);
  }, [defaultValues, form]);

  const currentType = form.watch("type");

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      if (rule) {
        const payload: UpdateRuleRequest = {
          name: values.name.trim(),
          mode: values.mode,
          content: values.content,
          enabled: values.enabled,
        };
        const updated = await updateMutation.mutateAsync({
          id: rule.id,
          payload,
        });
        toast.success(t("rule:toast.update_ok"));
        onSaved?.(updated);
      } else {
        const payload: CreateRuleRequest = {
          name: values.name.trim(),
          type: values.type,
          mode: values.mode,
          content: values.content,
          enabled: values.enabled,
        };
        const created = await createMutation.mutateAsync(payload);
        toast.success(t("rule:toast.create_ok"));
        onSaved?.(created);
      }
    } catch (err) {
      handleError(err);
    }
  });

  return (
    <form
      data-testid="rule-form"
      onSubmit={onSubmit}
      className={cn("flex h-full min-h-0 flex-col gap-4 p-4", className)}
    >
      <header className="flex items-center justify-between">
        <h2 className="text-[var(--font-size-base)] font-semibold text-[var(--color-text-primary)]">
          {rule ? t("rule:form.section_title") : t("rule:form.section_title_new")}
        </h2>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => setTemplateOpen(true)}
        >
          <LayoutTemplate className="mr-2 h-4 w-4" />
          {t("rule:form.use_template")}
        </Button>
      </header>

      <div className="flex flex-col gap-2">
        <Label htmlFor="rule-name">{t("rule:form.name_label")}</Label>
        <Input
          id="rule-name"
          placeholder={t("rule:form.name_placeholder")}
          {...form.register("name")}
          aria-invalid={Boolean(form.formState.errors.name)}
        />
        {form.formState.errors.name && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
            {form.formState.errors.name.message}
          </p>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="flex flex-col gap-2">
          <Label htmlFor="rule-type">{t("rule:form.type_label")}</Label>
          <Controller
            control={form.control}
            name="type"
            render={({ field }) => (
              <select
                id="rule-type"
                className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
                value={field.value}
                onChange={(e) => field.onChange(e.target.value as RuleType)}
                disabled={Boolean(rule)}
              >
                <option value="rules">{t("rule:types.rules")}</option>
                <option value="dns">{t("rule:types.dns")}</option>
                <option value="rule-providers">
                  {t("rule:types.rule-providers")}
                </option>
              </select>
            )}
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="rule-mode">{t("rule:form.mode_label")}</Label>
          <Controller
            control={form.control}
            name="mode"
            render={({ field }) => (
              <select
                id="rule-mode"
                className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
                value={field.value}
                onChange={(e) => field.onChange(e.target.value as RuleMode)}
              >
                <option value="replace">{t("rule:modes.replace")}</option>
                <option value="prepend">{t("rule:modes.prepend")}</option>
                <option value="append">{t("rule:modes.append")}</option>
              </select>
            )}
          />
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-2">
        <Label htmlFor="rule-content">{t("rule:form.content_label")}</Label>
        <textarea
          id="rule-content"
          rows={12}
          spellCheck={false}
          placeholder={contentPlaceholder(t, currentType)}
          className="h-full min-h-[16rem] w-full resize-none rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 py-2 font-mono text-[var(--font-size-xs)] text-[var(--color-text-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
          {...form.register("content")}
        />
      </div>

      <div className="flex items-center justify-between gap-4">
        <label className="flex items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
          <Controller
            control={form.control}
            name="enabled"
            render={({ field }) => (
              <input
                type="checkbox"
                checked={field.value}
                onChange={(e) => field.onChange(e.target.checked)}
                className="h-4 w-4 rounded border-[var(--color-border-strong)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]"
              />
            )}
          />
          {t("rule:form.enabled_label")}
        </label>

        <div className="flex gap-2">
          {onCancel && (
            <Button type="button" variant="outline" size="sm" onClick={onCancel}>
              {t("rule:form.cancel")}
            </Button>
          )}
          <Button
            type="submit"
            size="sm"
            disabled={form.formState.isSubmitting || !form.formState.isDirty}
          >
            <Save className="mr-2 h-4 w-4" />
            {t("rule:form.save")}
          </Button>
        </div>
      </div>

      <RuleTemplatesDialog
        open={templateOpen}
        onOpenChange={setTemplateOpen}
        onTemplateSelect={(tpl) => {
          form.setValue("content", tpl.content, {
            shouldDirty: true,
            shouldValidate: true,
          });
          if (!form.getValues("name")) {
            form.setValue("name", tpl.name, { shouldDirty: true });
          }
          // All built-in templates render to the rules section.
          if (!rule) {
            form.setValue("type", "rules", { shouldDirty: true });
          }
          setTemplateOpen(false);
          toast.success(t("rule:templates.applied"));
        }}
      />
    </form>
  );
}

function contentPlaceholder(
  t: (key: string) => string,
  type: RuleType,
): string {
  switch (type) {
    case "dns":
      return t("rule:form.content_placeholder_dns");
    case "rule-providers":
      return t("rule:form.content_placeholder_rule_providers");
    default:
      return t("rule:form.content_placeholder_rules");
  }
}
