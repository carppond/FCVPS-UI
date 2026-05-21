import * as React from "react";
import { useTranslation } from "react-i18next";
import { Trash2 } from "lucide-react";
import { cn } from "@/lib/cn";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  EmptyState,
} from "@/components/ui/empty-state";
import { getOperatorMeta } from "@/components/pipeline/operator-meta";
import { FilterForm } from "@/components/pipeline/param-forms/filter";
import { MapForm } from "@/components/pipeline/param-forms/map";
import { SortForm } from "@/components/pipeline/param-forms/sort";
import { DedupeForm } from "@/components/pipeline/param-forms/dedupe";
import { RegexRenameForm } from "@/components/pipeline/param-forms/regex-rename";
import { OutputForm } from "@/components/pipeline/param-forms/output";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";
import type { PipelineOperator } from "@/types/api";

interface ParamPanelProps {
  className?: string;
}

/**
 * Right-rail parameter panel.
 *
 *  - Header: operator icon + name + `type` badge.
 *  - Body: per-type form (filter / map / sort / dedupe / regex_rename /
 *    output). Form components are dumb — they receive `operator` + an
 *    `onChange` callback and never read the store directly so they stay
 *    trivially testable.
 *  - Footer: destructive "remove operator" button.
 */
export function ParamPanel({ className }: ParamPanelProps) {
  const { t } = useTranslation(["pipeline", "common"]);

  const selectedId = usePipelineEditorStore((s) => s.selectedOperatorId);
  const operators = usePipelineEditorStore((s) => s.ast.operators);
  const updateOperatorParams = usePipelineEditorStore(
    (s) => s.updateOperatorParams,
  );
  const removeOperator = usePipelineEditorStore((s) => s.removeOperator);

  const operator = React.useMemo(
    () => operators.find((op) => op.id === selectedId) ?? null,
    [operators, selectedId],
  );

  return (
    <aside
      data-testid="param-panel"
      className={cn(
        "flex h-full w-80 shrink-0 flex-col border-l border-[var(--color-border)]",
        "bg-[var(--color-bg-elevated)]",
        className,
      )}
    >
      <header className="flex items-center justify-between gap-2 border-b border-[var(--color-border)] px-4 py-3">
        <h2 className="text-[var(--font-size-sm)] font-semibold uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("pipeline:editor.param_panel_title")}
        </h2>
      </header>

      {operator ? (
        <ParamPanelBody
          operator={operator}
          onChange={(params) => updateOperatorParams(operator.id, params)}
          onRemove={() => {
            if (window.confirm(t("pipeline:param_form.common.delete_confirm"))) {
              removeOperator(operator.id);
            }
          }}
        />
      ) : (
        <div className="flex flex-1 items-center justify-center p-4">
          <EmptyState
            title={t("pipeline:param_form.common.no_selection_title")}
            description={t(
              "pipeline:param_form.common.no_selection_description",
            )}
          />
        </div>
      )}
    </aside>
  );
}

interface ParamPanelBodyProps {
  operator: PipelineOperator;
  onChange: (params: PipelineOperator["params"]) => void;
  onRemove: () => void;
}

function ParamPanelBody({ operator, onChange, onRemove }: ParamPanelBodyProps) {
  const { t } = useTranslation(["pipeline"]);
  const meta = getOperatorMeta(operator.type);
  const Icon = meta.iconComponent;

  return (
    <>
      <div
        data-testid="param-panel-header"
        className="flex items-center gap-3 border-b border-[var(--color-border)] px-4 py-3"
      >
        <span
          aria-hidden
          className={cn(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--radius-sm)]",
            "bg-[var(--color-surface)] text-[var(--color-primary)]",
          )}
        >
          <Icon className="h-4 w-4" />
        </span>
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
            {t(meta.nameKey)}
          </span>
          <span className="truncate text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t(meta.descriptionKey)}
          </span>
        </div>
        <Badge variant="outline" data-testid="param-panel-type-badge">
          {operator.type}
        </Badge>
      </div>

      <div className="flex flex-1 flex-col gap-4 overflow-y-auto px-4 py-4">
        {renderForm(operator, onChange)}
      </div>

      <footer className="flex items-center justify-end border-t border-[var(--color-border)] px-4 py-3">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onRemove}
          data-testid="param-panel-delete"
          className="text-[var(--color-error)] hover:bg-[var(--color-error-bg)]"
        >
          <Trash2 className="mr-1 h-3.5 w-3.5" />
          {t("pipeline:param_form.common.delete_operator")}
        </Button>
      </footer>
    </>
  );
}

function renderForm(
  operator: PipelineOperator,
  onChange: (params: PipelineOperator["params"]) => void,
): React.ReactNode {
  switch (operator.type) {
    case "filter":
      return <FilterForm operator={operator} onChange={onChange} />;
    case "map":
      return <MapForm operator={operator} onChange={onChange} />;
    case "sort":
      return <SortForm operator={operator} onChange={onChange} />;
    case "dedupe":
      return <DedupeForm operator={operator} onChange={onChange} />;
    case "regex_rename":
      return <RegexRenameForm operator={operator} onChange={onChange} />;
    case "output":
      return <OutputForm operator={operator} onChange={onChange} />;
  }
}
