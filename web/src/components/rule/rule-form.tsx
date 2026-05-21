import * as React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { LayoutTemplate, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
 * Two-mode form (create / edit) for a single custom rule. Layout follows the
 * Swiss / minimalism cheatsheet: vertical sections separated by hairline rules,
 * tabs for the enum fields (type / mode), monospaced textarea for content.
 *
 * The Save button stays enabled even when the form is pristine — the zod
 * schema surfaces any validation issue inline. This avoids the "click does
 * nothing" trap users hit when the button is disabled but they don't know
 * why.
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

  // Cmd/Ctrl + S anywhere inside the form fires Save. Keeps the keyboard-first
  // promise from the cheatsheet without adding a global hot-key helper.
  const onKeyDown = (e: React.KeyboardEvent<HTMLFormElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      void onSubmit();
    }
  };

  return (
    <form
      data-testid="rule-form"
      onSubmit={onSubmit}
      onKeyDown={onKeyDown}
      className={cn("flex h-full min-h-0 flex-col", className)}
    >
      {/* ── Header ─────────────────────────────────────────────────────── */}
      <header className="flex items-center justify-between gap-3 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-6 py-4">
        <div className="flex min-w-0 items-center gap-3">
          <h2 className="truncate text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
            {rule ? rule.name || t("rule:form.section_title") : t("rule:form.section_title_new")}
          </h2>
          {!rule && (
            <Badge variant="default" className="shrink-0">
              {t("rule:form.section_title_new")}
            </Badge>
          )}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => setTemplateOpen(true)}
        >
          <LayoutTemplate className="h-3.5 w-3.5" />
          {t("rule:form.use_template")}
        </Button>
      </header>

      {/* ── Body ───────────────────────────────────────────────────────── */}
      <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-6 py-4">
        {/* Name */}
        <section className="flex flex-col gap-2">
          <Label htmlFor="rule-name">{t("rule:form.name_label")}</Label>
          <Input
            id="rule-name"
            placeholder={t("rule:form.name_placeholder")}
            autoComplete="off"
            {...form.register("name")}
            aria-invalid={Boolean(form.formState.errors.name)}
          />
          {form.formState.errors.name && (
            <p className="text-[var(--font-size-xs)] text-[var(--color-error)]">
              {form.formState.errors.name.message}
            </p>
          )}
        </section>

        {/* Type — disabled when editing (server can't migrate type) */}
        <section className="flex flex-col gap-2">
          <Label>{t("rule:form.type_label")}</Label>
          <Controller
            control={form.control}
            name="type"
            render={({ field }) => (
              <Tabs
                value={field.value}
                onValueChange={(v) => field.onChange(v as RuleType)}
              >
                <TabsList className="w-full">
                  <TabsTrigger
                    value="rules"
                    className="flex-1"
                    disabled={Boolean(rule)}
                  >
                    {t("rule:types.rules")}
                  </TabsTrigger>
                  <TabsTrigger
                    value="dns"
                    className="flex-1"
                    disabled={Boolean(rule)}
                  >
                    {t("rule:types.dns")}
                  </TabsTrigger>
                  <TabsTrigger
                    value="rule-providers"
                    className="flex-1"
                    disabled={Boolean(rule)}
                  >
                    {t("rule:types.rule-providers")}
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            )}
          />
        </section>

        {/* Mode */}
        <section className="flex flex-col gap-2">
          <Label>{t("rule:form.mode_label")}</Label>
          <Controller
            control={form.control}
            name="mode"
            render={({ field }) => (
              <Tabs
                value={field.value}
                onValueChange={(v) => field.onChange(v as RuleMode)}
              >
                <TabsList className="w-full">
                  <TabsTrigger value="append" className="flex-1">
                    {t("rule:modes.append")}
                  </TabsTrigger>
                  <TabsTrigger value="prepend" className="flex-1">
                    {t("rule:modes.prepend")}
                  </TabsTrigger>
                  <TabsTrigger value="replace" className="flex-1">
                    {t("rule:modes.replace")}
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            )}
          />
        </section>

        {/* Content */}
        <section className="flex min-h-0 flex-1 flex-col gap-2">
          <Label htmlFor="rule-content">{t("rule:form.content_label")}</Label>
          <textarea
            id="rule-content"
            rows={14}
            spellCheck={false}
            placeholder={contentPlaceholder(t, currentType)}
            className={cn(
              "min-h-64 w-full flex-1 resize-none",
              "rounded-[var(--radius-md)] border border-[var(--color-border-strong)]",
              "bg-[var(--color-surface)] px-3 py-2",
              "font-mono text-[var(--font-size-xs)] leading-relaxed text-[var(--color-text-primary)]",
              "placeholder:text-[var(--color-text-disabled)]",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
              "transition-colors duration-[var(--duration-fast)]",
            )}
            {...form.register("content")}
          />
        </section>
      </div>

      {/* ── Footer ─────────────────────────────────────────────────────── */}
      <footer className="flex items-center justify-between gap-4 border-t border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-6 py-3">
        <label className="flex cursor-pointer items-center gap-2 text-[var(--font-size-sm)] text-[var(--color-text-secondary)]">
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

        <div className="flex items-center gap-2">
          {onCancel && (
            <Button type="button" variant="outline" size="sm" onClick={onCancel}>
              {t("rule:form.cancel")}
            </Button>
          )}
          <Button
            type="submit"
            size="sm"
            disabled={form.formState.isSubmitting}
          >
            <Save className="h-3.5 w-3.5" />
            {t("rule:form.save")}
          </Button>
        </div>
      </footer>

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
