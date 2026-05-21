import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  Globe,
  Upload,
  PencilLine,
  Check,
  MapPin,
  Shield,
  Ban,
  SkipForward,
} from "lucide-react";
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
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/toast";
import { cn } from "@/lib/cn";
import { useApiError } from "@/hooks/use-api-error";
import {
  useCreateSubscriptionMutation,
  useUploadSubscriptionMutation,
} from "@/api/subscription";
import {
  useCreateRuleMutation,
  useRuleTemplatesQuery,
  useRulesQuery,
} from "@/api/rule";
import { SubUpload } from "./sub-upload";
import { SubTagInput } from "./sub-tag-input";
import type { RuleTemplate, SubType } from "@/types/api";

interface SubCreateWizardProps {
  open: boolean;
  onClose: () => void;
  /** Optional callback when the new subscription is persisted. */
  onCreated?: (subscriptionId: string) => void;
}

type SourceChoice = SubType;
type WizardStep = 1 | 2 | 3 | 4;

/** Sentinel for the "skip" radio option in step 4. */
const TEMPLATE_SKIP = "__skip__" as const;
/** Default template id pre-selected on step 4 (most common need). */
const DEFAULT_TEMPLATE_ID = "cn-direct-foreign-proxy";

type TemplateChoice = string; // template.id or TEMPLATE_SKIP

interface WizardState {
  step: WizardStep;
  source: SourceChoice;
  name: string;
  sourceUrl: string;
  ua: string;
  file: File | null;
  tags: string[];
  syncInterval: number; // seconds; 0 = manual only
  templateChoice: TemplateChoice;
}

const SYNC_INTERVAL_OPTIONS = [
  { value: 3600, key: "h1" },
  { value: 21600, key: "h6" }, // default per PRD
  { value: 43200, key: "h12" },
  { value: 86400, key: "h24" },
  { value: 0, key: "manual" },
] as const;

/** Lucide icon used per built-in template id; falls back to Shield. */
function templateIcon(id: string): React.ReactNode {
  if (id === "cn-direct-foreign-proxy")
    return <MapPin className="h-5 w-5" />;
  if (id === "global-proxy") return <Shield className="h-5 w-5" />;
  if (id === "ad-block") return <Ban className="h-5 w-5" />;
  return <Shield className="h-5 w-5" />;
}

/** Count non-empty, non-comment rule lines in a template's content blob. */
function countRuleLines(content: string): number {
  return content
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !line.startsWith("#")).length;
}

function initialState(): WizardState {
  return {
    step: 1,
    source: "url",
    name: "",
    sourceUrl: "",
    ua: "",
    file: null,
    tags: [],
    syncInterval: 21600,
    templateChoice: DEFAULT_TEMPLATE_ID,
  };
}

export function SubCreateWizard({
  open,
  onClose,
  onCreated,
}: SubCreateWizardProps) {
  const { t } = useTranslation(["subscription", "common"]);
  const { handle: handleError } = useApiError();
  const [state, setState] = React.useState<WizardState>(initialState());

  const createMutation = useCreateSubscriptionMutation();
  const uploadMutation = useUploadSubscriptionMutation();
  const createRuleMutation = useCreateRuleMutation();
  const templatesQuery = useRuleTemplatesQuery();
  // Cheap "do you already have rules?" probe — pageSize=1 so payload is tiny.
  const existingRulesQuery = useRulesQuery({ page: 1, pageSize: 1 });

  // Reset state every time the dialog opens so reopening is a clean slate.
  React.useEffect(() => {
    if (open) setState(initialState());
  }, [open]);

  const [importingRules, setImportingRules] = React.useState(false);
  const isPending =
    createMutation.isPending || uploadMutation.isPending || importingRules;

  const set = (patch: Partial<WizardState>) =>
    setState((prev) => ({ ...prev, ...patch }));

  const goNext = () => {
    if (state.step === 1) {
      set({ step: 2 });
      return;
    }
    if (state.step === 2) {
      // step 2 validation
      if (!state.name.trim()) {
        toast.error(t("subscription:error.name_required"));
        return;
      }
      if (state.source === "url" && !state.sourceUrl.trim()) {
        toast.error(t("subscription:error.url_required"));
        return;
      }
      if (state.source === "upload" && !state.file) {
        toast.error(t("subscription:error.file_required"));
        return;
      }
      set({ step: 3 });
      return;
    }
    if (state.step === 3) {
      set({ step: 4 });
    }
  };

  const goBack = () =>
    set({ step: Math.max(1, state.step - 1) as WizardStep });

  /**
   * Import the chosen template as one (or more) CustomRule records. Returns
   * the number of rules that failed so the caller can surface a warning toast.
   *
   * Backend templates ship as a single multi-line `content` blob — we create
   * one rule per template, mode=prepend so it takes precedence at render time.
   */
  const importTemplate = async (
    template: RuleTemplate,
    indexBase: number,
  ): Promise<number> => {
    let failed = 0;
    try {
      await createRuleMutation.mutateAsync({
        name: template.name,
        type: "rules",
        mode: "prepend",
        content: template.content,
        enabled: true,
        sort: indexBase * 100,
      });
    } catch {
      failed += 1;
    }
    return failed;
  };

  const submit = async () => {
    try {
      let subId: string;
      if (state.source === "upload" && state.file) {
        const sub = await uploadMutation.mutateAsync({
          name: state.name.trim(),
          file: state.file,
          tags: state.tags,
          syncInterval: state.syncInterval || undefined,
        });
        toast.success(
          t("subscription:detail.sync_success", {
            added: sub.node_count,
            removed: 0,
          }),
        );
        subId = sub.id;
      } else {
        const sub = await createMutation.mutateAsync({
          name: state.name.trim(),
          type: state.source,
          source_url:
            state.source === "url" ? state.sourceUrl.trim() : undefined,
          ua:
            state.source === "url" ? state.ua.trim() || undefined : undefined,
          sync_interval: state.syncInterval || undefined,
          tags: state.tags,
        });
        subId = sub.id;
      }

      // Apply the chosen template (if any) — failures here are non-blocking;
      // the subscription is already persisted.
      if (state.templateChoice !== TEMPLATE_SKIP) {
        const tpl = templatesQuery.data?.find(
          (it) => it.id === state.templateChoice,
        );
        if (tpl) {
          setImportingRules(true);
          const failed = await importTemplate(tpl, 0);
          setImportingRules(false);
          if (failed > 0) {
            toast.warning(
              t("subscription:wizard.step4.partial_failure", { failed }),
            );
          }
        }
      }

      onCreated?.(subId);
      onClose();
    } catch (err) {
      setImportingRules(false);
      handleError(err);
    }
  };

  const finishDisabled =
    isPending ||
    (state.templateChoice !== TEMPLATE_SKIP && templatesQuery.isLoading);

  return (
    <Dialog open={open} onOpenChange={(o) => !o && !isPending && onClose()}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("subscription:wizard.title")}</DialogTitle>
        </DialogHeader>

        <StepIndicator current={state.step} />

        {state.step === 1 && (
          <StepSource
            value={state.source}
            onChange={(s) => set({ source: s })}
          />
        )}

        {state.step === 2 && <StepDetails state={state} onChange={set} />}

        {state.step === 3 && (
          <StepTagsInterval
            tags={state.tags}
            syncInterval={state.syncInterval}
            onTagsChange={(tags) => set({ tags })}
            onIntervalChange={(syncInterval) => set({ syncInterval })}
          />
        )}

        {state.step === 4 && (
          <StepTemplate
            value={state.templateChoice}
            templates={templatesQuery.data}
            isLoading={templatesQuery.isLoading}
            existingCount={existingRulesQuery.data?.total ?? 0}
            onChange={(templateChoice) => set({ templateChoice })}
          />
        )}

        <DialogFooter className="mt-2">
          <Button
            type="button"
            variant="outline"
            onClick={state.step === 1 ? onClose : goBack}
            disabled={isPending}
          >
            {state.step === 1
              ? t("common:actions.cancel")
              : t("subscription:wizard.back")}
          </Button>
          {state.step < 4 ? (
            <Button type="button" onClick={goNext} disabled={isPending}>
              {t("subscription:wizard.next")}
            </Button>
          ) : (
            <Button type="button" onClick={submit} disabled={finishDisabled}>
              {t("subscription:wizard.finish")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ── Step indicator ──────────────────────────────────────────────────────────

function StepIndicator({ current }: { current: WizardStep }) {
  const { t } = useTranslation(["subscription", "common"]);
  const labels = [
    t("subscription:wizard.step1"),
    t("subscription:wizard.step2"),
    t("subscription:wizard.step3"),
    t("subscription:wizard.step4.indicator"),
  ];
  return (
    <ol
      className="flex items-center gap-2 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]"
      aria-label={t("common:aria.wizard_steps")}
    >
      {labels.map((label, idx) => {
        const stepNum = (idx + 1) as WizardStep;
        const isActive = stepNum === current;
        const isDone = stepNum < current;
        return (
          <li
            key={label}
            data-testid={`wizard-step-${stepNum}`}
            className={cn(
              "flex items-center gap-2",
              idx < labels.length - 1 && "flex-1",
            )}
          >
            <span
              className={cn(
                "flex h-5 w-5 items-center justify-center rounded-full text-[var(--font-size-xs)]",
                isActive &&
                  "bg-[var(--color-primary)] text-[var(--color-primary-foreground)]",
                isDone &&
                  "bg-[var(--color-success)] text-[var(--color-primary-foreground)]",
                !isActive &&
                  !isDone &&
                  "border border-[var(--color-border-strong)] text-[var(--color-text-tertiary)]",
              )}
            >
              {isDone ? <Check className="h-3 w-3" /> : stepNum}
            </span>
            <span
              className={cn(
                isActive
                  ? "font-medium text-[var(--color-text-primary)]"
                  : undefined,
              )}
            >
              {label}
            </span>
            {idx < labels.length - 1 && (
              <span className="mx-2 h-px flex-1 bg-[var(--color-border)]" />
            )}
          </li>
        );
      })}
    </ol>
  );
}

// ── Step 1: source ──────────────────────────────────────────────────────────

interface StepSourceProps {
  value: SourceChoice;
  onChange: (next: SourceChoice) => void;
}

function StepSource({ value, onChange }: StepSourceProps) {
  const { t } = useTranslation("subscription");
  const choices: {
    id: SourceChoice;
    icon: React.ReactNode;
    title: string;
    desc: string;
  }[] = [
    {
      id: "url",
      icon: <Globe className="h-5 w-5" />,
      title: t("subscription:wizard.source.url_title"),
      desc: t("subscription:wizard.source.url_desc"),
    },
    {
      id: "upload",
      icon: <Upload className="h-5 w-5" />,
      title: t("subscription:wizard.source.upload_title"),
      desc: t("subscription:wizard.source.upload_desc"),
    },
    {
      id: "manual",
      icon: <PencilLine className="h-5 w-5" />,
      title: t("subscription:wizard.source.manual_title"),
      desc: t("subscription:wizard.source.manual_desc"),
    },
  ];

  return (
    <div
      role="radiogroup"
      aria-label={t("subscription:wizard.step1")}
      className="flex flex-col gap-2"
    >
      {choices.map((c) => {
        const selected = value === c.id;
        return (
          <button
            key={c.id}
            type="button"
            role="radio"
            aria-checked={selected}
            data-testid={`wizard-source-${c.id}`}
            onClick={() => onChange(c.id)}
            className={cn(
              "flex items-start gap-3 rounded-[var(--radius-lg)] border p-3 text-left",
              "transition-colors duration-[var(--duration-fast)]",
              selected
                ? "border-[var(--color-primary)] bg-[var(--color-surface-hover)]"
                : "border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-hover)]",
            )}
          >
            <span
              className={cn(
                "mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--radius-md)]",
                selected
                  ? "bg-[var(--color-primary)] text-[var(--color-primary-foreground)]"
                  : "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]",
              )}
            >
              {c.icon}
            </span>
            <span className="flex flex-col gap-0.5">
              <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
                {c.title}
              </span>
              <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
                {c.desc}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

// ── Step 2: source-specific form ────────────────────────────────────────────

interface StepDetailsProps {
  state: WizardState;
  onChange: (patch: Partial<WizardState>) => void;
}

function StepDetails({ state, onChange }: StepDetailsProps) {
  const { t } = useTranslation("subscription");
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor="sub-name">
          {t("subscription:wizard.form.name_label")}
        </Label>
        <Input
          id="sub-name"
          value={state.name}
          onChange={(e) => onChange({ name: e.target.value })}
          placeholder={t("subscription:wizard.form.name_placeholder")}
        />
      </div>

      {state.source === "url" && (
        <>
          <div className="flex flex-col gap-2">
            <Label htmlFor="sub-url">
              {t("subscription:wizard.form.source_url_label")}
            </Label>
            <Input
              id="sub-url"
              value={state.sourceUrl}
              onChange={(e) => onChange({ sourceUrl: e.target.value })}
              placeholder={t("subscription:wizard.form.source_url_placeholder")}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="sub-ua">
              {t("subscription:wizard.form.ua_label")}
            </Label>
            <Input
              id="sub-ua"
              value={state.ua}
              onChange={(e) => onChange({ ua: e.target.value })}
              placeholder={t("subscription:wizard.form.ua_placeholder")}
            />
            <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
              {t("subscription:wizard.form.ua_hint")}
            </p>
          </div>
        </>
      )}

      {state.source === "upload" && (
        <SubUpload file={state.file} onChange={(f) => onChange({ file: f })} />
      )}

      {state.source === "manual" && (
        <p className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("subscription:wizard.source.manual_desc")}
        </p>
      )}
    </div>
  );
}

// ── Step 3: tags + sync interval ────────────────────────────────────────────

interface StepTagsIntervalProps {
  tags: string[];
  syncInterval: number;
  onTagsChange: (tags: string[]) => void;
  onIntervalChange: (interval: number) => void;
}

function StepTagsInterval({
  tags,
  syncInterval,
  onTagsChange,
  onIntervalChange,
}: StepTagsIntervalProps) {
  const { t } = useTranslation("subscription");
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label htmlFor="sub-tags">{t("subscription:wizard.tags.label")}</Label>
        <SubTagInput id="sub-tags" value={tags} onChange={onTagsChange} />
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="sub-interval">
          {t("subscription:wizard.sync_interval.label")}
        </Label>
        <select
          id="sub-interval"
          value={syncInterval}
          onChange={(e) => onIntervalChange(Number(e.target.value))}
          className="h-9 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] px-3 text-[var(--font-size-sm)] text-[var(--color-text-primary)]"
        >
          {SYNC_INTERVAL_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {t(`subscription:wizard.sync_interval.${opt.key}`)}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}

// ── Step 4: rule template (optional) ────────────────────────────────────────

interface StepTemplateProps {
  value: TemplateChoice;
  templates: RuleTemplate[] | undefined;
  isLoading: boolean;
  existingCount: number;
  onChange: (next: TemplateChoice) => void;
}

function StepTemplate({
  value,
  templates,
  isLoading,
  existingCount,
  onChange,
}: StepTemplateProps) {
  const { t } = useTranslation("subscription");

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-1">
        <h3 className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
          {t("subscription:wizard.step4.title")}
        </h3>
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("subscription:wizard.step4.subtitle")}
        </p>
      </div>

      {existingCount > 0 && (
        <div
          role="status"
          className="rounded-[var(--radius-md)] border border-[var(--color-warning)] bg-[var(--color-warning)]/10 px-3 py-2 text-[var(--font-size-xs)] text-[var(--color-text-primary)]"
        >
          {t("subscription:wizard.step4.existing_warning", {
            count: existingCount,
          })}
        </div>
      )}

      <div
        role="radiogroup"
        aria-label={t("subscription:wizard.step4.title")}
        className="flex flex-col gap-2"
      >
        {isLoading && <TemplateCardsSkeleton />}

        {!isLoading &&
          templates?.map((tpl) => {
            const selected = value === tpl.id;
            const count = countRuleLines(tpl.content);
            return (
              <TemplateOption
                key={tpl.id}
                id={tpl.id}
                selected={selected}
                icon={templateIcon(tpl.id)}
                title={tpl.name}
                desc={tpl.description}
                meta={t("subscription:wizard.step4.rules_count", { count })}
                onClick={() => onChange(tpl.id)}
              />
            );
          })}

        {!isLoading && (
          <TemplateOption
            id={TEMPLATE_SKIP}
            selected={value === TEMPLATE_SKIP}
            icon={<SkipForward className="h-5 w-5" />}
            title={t("subscription:wizard.step4.skip")}
            desc={t("subscription:wizard.step4.skip_desc")}
            onClick={() => onChange(TEMPLATE_SKIP)}
          />
        )}
      </div>
    </div>
  );
}

interface TemplateOptionProps {
  id: string;
  selected: boolean;
  icon: React.ReactNode;
  title: string;
  desc: string;
  meta?: string;
  onClick: () => void;
}

function TemplateOption({
  id,
  selected,
  icon,
  title,
  desc,
  meta,
  onClick,
}: TemplateOptionProps) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      data-testid={`wizard-template-${id}`}
      onClick={onClick}
      className={cn(
        "flex items-start gap-3 rounded-[var(--radius-lg)] border p-3 text-left",
        "transition-colors duration-[var(--duration-fast)]",
        selected
          ? "border-[var(--color-primary)] bg-[var(--color-primary)]/10"
          : "border-[var(--color-border)] bg-[var(--color-bg-elevated)] hover:bg-[var(--color-surface-hover)]",
      )}
    >
      <span
        className={cn(
          "mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--radius-md)]",
          selected
            ? "bg-[var(--color-primary)] text-[var(--color-primary-foreground)]"
            : "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]",
        )}
      >
        {icon}
      </span>
      <span className="flex flex-col gap-0.5">
        <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
          {title}
        </span>
        <span className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {desc}
        </span>
        {meta && (
          <span className="mt-1 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {meta}
          </span>
        )}
      </span>
    </button>
  );
}

function TemplateCardsSkeleton() {
  return (
    <div className="flex flex-col gap-2" aria-hidden>
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="flex items-start gap-3 rounded-[var(--radius-lg)] border border-[var(--color-border)] p-3"
        >
          <Skeleton className="h-9 w-9 rounded-[var(--radius-md)]" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-3.5 w-40" />
            <Skeleton className="h-3 w-64" />
          </div>
        </div>
      ))}
    </div>
  );
}
