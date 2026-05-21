import * as React from "react";
import { useTranslation } from "react-i18next";
import { Play, Loader2, ChevronDown, ChevronRight } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { EmptyState } from "@/components/ui/empty-state";
import { useApiError } from "@/hooks/use-api-error";
import { useRunPreview } from "@/api/pipeline";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";
import { getOperatorMeta } from "@/components/pipeline/operator-meta";
import { cn } from "@/lib/cn";
import type { OperatorStepResult, PipelineOperator } from "@/types/api";

export interface PreviewPaneProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Pipeline id used in POST /api/pipelines/:id/run. */
  pipelineId: string;
}

/**
 * Debug preview modal.
 *
 *  - Top: subscription-id input + "Run preview" button.
 *  - Body: grid of operator cards — one per step in `RunPipelineResponse`.
 *    Each card shows input/output counts and expandable lists of added /
 *    removed / modified node names. Names are sourced from the user's local
 *    AST when available (steps come back keyed by operator id).
 */
export function PreviewPane({ open, onOpenChange, pipelineId }: PreviewPaneProps) {
  const { t } = useTranslation(["pipeline", "common"]);
  const { handle: handleError } = useApiError();

  const operators = usePipelineEditorStore((s) => s.ast.operators);
  const debugTrace = usePipelineEditorStore((s) => s.debugTrace);
  const setDebugTrace = usePipelineEditorStore((s) => s.setDebugTrace);

  const runPreview = useRunPreview();
  const [subscriptionId, setSubscriptionId] = React.useState("");

  const handleRun = async () => {
    const id = subscriptionId.trim();
    if (!id) return;
    try {
      const resp = await runPreview.mutateAsync({
        id: pipelineId,
        payload: { subscription_id: id, debug: true },
      });
      setDebugTrace(resp);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-w-4xl"
        data-testid="preview-pane"
      >
        <DialogHeader>
          <DialogTitle>{t("pipeline:preview.title")}</DialogTitle>
          <DialogDescription>
            {t("pipeline:preview.no_result_description")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex items-end gap-3">
          <div className="flex flex-1 flex-col gap-1.5">
            <Label htmlFor="preview-subscription">
              {t("pipeline:preview.subscription_label")}
            </Label>
            <Input
              id="preview-subscription"
              data-testid="preview-subscription"
              value={subscriptionId}
              onChange={(e) => setSubscriptionId(e.target.value)}
              placeholder={t("pipeline:preview.subscription_placeholder")}
              spellCheck={false}
              autoComplete="off"
              className="font-mono"
            />
          </div>
          <Button
            type="button"
            size="default"
            onClick={handleRun}
            disabled={runPreview.isPending || !subscriptionId.trim()}
            data-testid="preview-run"
          >
            {runPreview.isPending ? (
              <Loader2 className="mr-1 h-4 w-4 animate-spin" aria-hidden />
            ) : (
              <Play className="mr-1 h-4 w-4" aria-hidden />
            )}
            {runPreview.isPending
              ? t("pipeline:preview.running")
              : t("pipeline:preview.run")}
          </Button>
        </div>

        {debugTrace ? (
          <PreviewResultBody trace={debugTrace} operators={operators} />
        ) : (
          <EmptyState
            title={t("pipeline:preview.no_result_title")}
            description={t("pipeline:preview.no_result_description")}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

interface PreviewResultBodyProps {
  trace: NonNullable<ReturnType<typeof usePipelineEditorStore.getState>["debugTrace"]>;
  operators: PipelineOperator[];
}

function PreviewResultBody({ trace, operators }: PreviewResultBodyProps) {
  const { t } = useTranslation(["pipeline"]);
  return (
    <div className="flex flex-col gap-3" data-testid="preview-result">
      <div className="flex items-center gap-4 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        <span className="font-mono tabular-nums">
          {t("pipeline:preview.total_ms", { ms: trace.total_ms })}
        </span>
        <span className="font-mono tabular-nums">
          {t("pipeline:preview.output_count", { count: trace.output_count })}
        </span>
      </div>

      {!trace.steps || trace.steps.length === 0 ? (
        <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t("pipeline:preview.no_steps")}
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {trace.steps.map((step, idx) => (
            <StepCard
              key={`${step.operator}-${idx}`}
              step={step}
              operator={operators.find((op) => op.id === step.operator) ?? null}
              index={idx}
            />
          ))}
        </div>
      )}
    </div>
  );
}

interface StepCardProps {
  step: OperatorStepResult;
  operator: PipelineOperator | null;
  index: number;
}

function StepCard({ step, operator, index }: StepCardProps) {
  const { t } = useTranslation(["pipeline"]);

  // When we have the local operator we can render its translated name + icon;
  // otherwise (e.g. server inserted an implicit op) fall back to the raw id.
  const meta = operator ? getOperatorMeta(operator.type) : null;
  const Icon = meta?.iconComponent;

  return (
    <article
      data-testid={`preview-step-${index}`}
      className={cn(
        "flex flex-col gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)]",
        "bg-[var(--color-surface)] p-3",
      )}
    >
      <header className="flex items-center gap-2">
        {Icon && (
          <span
            aria-hidden
            className="flex h-6 w-6 items-center justify-center rounded-[var(--radius-sm)] bg-[var(--color-bg-elevated)] text-[var(--color-primary)]"
          >
            <Icon className="h-3.5 w-3.5" />
          </span>
        )}
        <span className="truncate text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
          {meta ? t(meta.nameKey) : step.operator}
        </span>
        <Badge variant="outline" className="ml-auto font-mono">
          #{index + 1}
        </Badge>
      </header>

      <div className="flex items-center justify-between gap-2 text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
        <span className="flex flex-col">
          <span className="text-[var(--color-text-tertiary)]">
            {t("pipeline:preview.input_count")}
          </span>
          <span className="font-mono tabular-nums text-[var(--color-text-primary)]">
            {step.input_count}
          </span>
        </span>
        <ChevronRight className="h-3.5 w-3.5 text-[var(--color-text-tertiary)]" />
        <span className="flex flex-col text-right">
          <span className="text-[var(--color-text-tertiary)]">
            {t("pipeline:preview.output_count_short")}
          </span>
          <span className="font-mono tabular-nums text-[var(--color-text-primary)]">
            {step.output_count}
          </span>
        </span>
      </div>

      <NameList
        labelKey="pipeline:preview.step_added"
        items={step.added}
        tone="success"
      />
      <NameList
        labelKey="pipeline:preview.step_removed"
        items={step.removed}
        tone="error"
      />
      <NameList
        labelKey="pipeline:preview.step_modified"
        items={step.modified}
        tone="warning"
      />

      {step.added.length === 0 &&
        step.removed.length === 0 &&
        step.modified.length === 0 && (
          <p className="text-[var(--font-size-xs)] italic text-[var(--color-text-tertiary)]">
            {t("pipeline:preview.step_none")}
          </p>
        )}
    </article>
  );
}

interface NameListProps {
  labelKey: string;
  items: string[];
  tone: "success" | "error" | "warning";
}

function NameList({ labelKey, items, tone }: NameListProps) {
  const { t } = useTranslation(["pipeline"]);
  const [expanded, setExpanded] = React.useState(false);
  if (items.length === 0) return null;

  const toneClass =
    tone === "success"
      ? "text-[var(--color-success)]"
      : tone === "error"
        ? "text-[var(--color-error)]"
        : "text-[var(--color-warning)]";

  return (
    <details
      onToggle={(e) => setExpanded((e.target as HTMLDetailsElement).open)}
      className="rounded-[var(--radius-sm)] bg-[var(--color-bg-elevated)] p-1.5"
    >
      <summary
        className={cn(
          "flex cursor-pointer items-center gap-1 text-[var(--font-size-xs)]",
          toneClass,
        )}
      >
        {expanded ? (
          <ChevronDown className="h-3 w-3" aria-hidden />
        ) : (
          <ChevronRight className="h-3 w-3" aria-hidden />
        )}
        {t(labelKey, { count: items.length })}
      </summary>
      <ul className="mt-1 max-h-32 list-none space-y-0.5 overflow-y-auto pl-4 font-mono text-[var(--font-size-xs)] text-[var(--color-text-secondary)]">
        {items.map((name) => (
          <li key={name} className="truncate">
            {name}
          </li>
        ))}
      </ul>
    </details>
  );
}
