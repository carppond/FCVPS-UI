import * as React from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { LayoutTemplate, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useCreateRuleMutation,
  useUpdateRuleMutation,
} from "@/api/rule";
import { RuleTemplatesDialog } from "@/components/rule/rule-templates";
import { RuleIssues } from "@/components/rule/rule-issues";
import { ApiError } from "@/lib/api-client";
import type {
  CreateRuleRequest,
  CustomRule,
  RuleLineIssue,
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
   * Optional seed values applied when `rule` is null (e.g. a template the
   * user chose from the top-bar). Ignored in edit mode.
   */
  initialValues?: Partial<{
    name: string;
    type: RuleType;
    mode: RuleMode;
    content: string;
    enabled: boolean;
  }>;
  /**
   * Notifies the parent that an editing session has been committed. The
   * parent typically closes the hosting Dialog.
   */
  onSaved?: (rule: CustomRule | null) => void;
  /**
   * Cancel handler — wired to the footer's Cancel button.
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
 * Two-mode form (create / edit) for a single custom rule. Designed to live
 * inside a Dialog — no internal header / footer chrome, since the host
 * Dialog already supplies title + close button.
 *
 * Save / Cancel buttons sit in a sticky footer at the bottom of the form
 * body so they stay accessible while editing long YAML content.
 */
export function RuleForm({
  rule,
  initialValues,
  onSaved,
  onCancel,
  className,
}: RuleFormProps) {
  const { t } = useTranslation(["rule", "common"]);
  const { handle: handleError } = useApiError();
  const createMutation = useCreateRuleMutation();
  const updateMutation = useUpdateRuleMutation();
  const [issues, setIssues] = React.useState<RuleLineIssue[]>([]);

  const [templateOpen, setTemplateOpen] = React.useState(false);

  const schema = React.useMemo(() => buildSchema(t), [t]);

  const defaultValues: FormValues = React.useMemo(
    () => ({
      name: rule?.name ?? initialValues?.name ?? "",
      type: rule?.type ?? initialValues?.type ?? "rules",
      mode: rule?.mode ?? initialValues?.mode ?? "append",
      content: rule?.content ?? initialValues?.content ?? "",
      enabled: rule?.enabled ?? initialValues?.enabled ?? true,
    }),
    [rule, initialValues],
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
    setIssues([]);
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
      if (err instanceof ApiError && Array.isArray(err.details)) {
        setIssues(err.details as RuleLineIssue[]);
      }
      handleError(err);
    }
  });

  // Cmd/Ctrl + S anywhere inside the form fires Save.
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
      className={cn("flex min-h-0 flex-col gap-4", className)}
    >
      {/* ── Body ─────────────────────────────────────────────────────────── */}
      <div className="flex min-h-0 flex-1 flex-col gap-5">
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
        <section className="flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <Label htmlFor="rule-content">{t("rule:form.content_label")}</Label>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setTemplateOpen(true)}
            >
              <LayoutTemplate className="h-3.5 w-3.5" />
              {t("rule:form.use_template")}
            </Button>
          </div>
          <textarea
            id="rule-content"
            rows={12}
            spellCheck={false}
            placeholder={contentPlaceholder(t, currentType)}
            className={cn(
              "w-full resize-y",
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

      {issues.length > 0 && <RuleIssues issues={issues} />}

      {/* ── Footer ───────────────────────────────────────────────────────── */}
      <footer className="flex items-center justify-between gap-4 border-t border-[var(--color-border)] pt-4">
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
