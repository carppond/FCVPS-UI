import * as React from "react";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useTranslation } from "react-i18next";
import { GripVertical, Trash2 } from "lucide-react";
import { cn } from "@/lib/cn";
import { Badge } from "@/components/ui/badge";
import { getOperatorMeta } from "@/components/pipeline/operator-meta";
import type { PipelineOperator } from "@/types/api";

interface OperatorNodeProps {
  operator: PipelineOperator;
  /** Whether the node is currently selected (highlighted with primary ring). */
  selected: boolean;
  /** Whether the operator passes local validation (drives the badge variant). */
  valid?: boolean;
  /** Click on the body — selects the operator and opens the param panel (T-21). */
  onSelect: (id: string) => void;
  /** Click on the trash icon. */
  onRemove: (id: string) => void;
}

/**
 * A single sortable operator card on the canvas.
 *
 *  - Wraps `useSortable` from `@dnd-kit/sortable`; the *drag handle* is the
 *    left grip icon — the rest of the card stays clickable so users can
 *    select / inspect an operator without accidentally starting a drag.
 *  - Selected state: `ring-2 ring-primary` to satisfy the visual contract
 *    (`bg-elevated + 选中态 ring-2 ring-primary`).
 */
export function OperatorNode({
  operator,
  selected,
  valid = true,
  onSelect,
  onRemove,
}: OperatorNodeProps) {
  const { t } = useTranslation(["pipeline"]);
  const meta = getOperatorMeta(operator.type);
  const Icon = meta.iconComponent;

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: operator.id });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      data-testid={`operator-node-${operator.id}`}
      data-operator-id={operator.id}
      data-operator-type={operator.type}
      data-selected={selected}
      className={cn(
        "group relative flex items-center gap-3 rounded-[var(--radius-lg)] border",
        "bg-[var(--color-bg-elevated)] px-3 py-3",
        "transition-colors duration-[var(--duration-fast)]",
        selected
          ? "border-[var(--color-primary)] ring-2 ring-[var(--color-primary)] ring-offset-0"
          : "border-[var(--color-border)] hover:border-[var(--color-border-strong)]",
        isDragging && "z-10 opacity-70 shadow-md",
      )}
    >
      {/* Drag handle */}
      <button
        type="button"
        aria-label={t("pipeline:editor.drag_handle")}
        data-testid={`operator-node-handle-${operator.id}`}
        className={cn(
          "flex h-7 w-5 shrink-0 items-center justify-center text-[var(--color-text-tertiary)]",
          "cursor-grab active:cursor-grabbing",
          "hover:text-[var(--color-text-secondary)]",
        )}
        {...listeners}
        {...attributes}
      >
        <GripVertical className="h-4 w-4" />
      </button>

      {/* Icon + main body (clickable to select) */}
      <button
        type="button"
        onClick={() => onSelect(operator.id)}
        data-testid={`operator-node-body-${operator.id}`}
        className={cn(
          "flex min-w-0 flex-1 items-center gap-3 text-left",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
          "rounded-[var(--radius-sm)]",
        )}
      >
        <span
          className={cn(
            "flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-sm)]",
            "bg-[var(--color-surface)] text-[var(--color-primary)]",
          )}
          aria-hidden
        >
          <Icon className="h-4 w-4" />
        </span>

        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-center gap-2">
            <span className="truncate text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
              {t(meta.nameKey)}
            </span>
            <span className="text-[var(--font-size-xs)] font-mono text-[var(--color-text-tertiary)]">
              {operator.type}
            </span>
          </span>
          <span className="truncate text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t(meta.descriptionKey)}
          </span>
        </span>

        <Badge variant={valid ? "outline" : "destructive"}>
          {valid
            ? t("pipeline:editor.node_status_valid")
            : t("pipeline:editor.node_status_error")}
        </Badge>
      </button>

      {/* Remove button — inline flex, not absolute, to avoid overlap with Badge */}
      <button
        type="button"
        aria-label={t("pipeline:editor.remove_operator")}
        data-testid={`operator-node-remove-${operator.id}`}
        onClick={(e) => {
          e.stopPropagation();
          onRemove(operator.id);
        }}
        className={cn(
          "flex h-7 w-7 shrink-0 items-center justify-center rounded-[var(--radius-sm)]",
          "text-[var(--color-text-tertiary)] opacity-0 transition-opacity duration-[var(--duration-fast)]",
          "hover:bg-[var(--color-error-bg)] hover:text-[var(--color-error)]",
          "group-hover:opacity-100 focus-visible:opacity-100",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
        )}
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}
