import { useDroppable } from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { useTranslation } from "react-i18next";
import { ChevronDown, Cog } from "lucide-react";
import { cn } from "@/lib/cn";
import { OperatorNode } from "@/components/pipeline/operator-node";
import { parseLibraryDraggableId } from "@/components/pipeline/operator-library";
import { usePipelineEditorStore } from "@/stores/pipeline-editor-store";
import type { PipelineOperator } from "@/types/api";

/** Stable droppable id for "drop a library card onto an empty canvas". */
export const CANVAS_DROPPABLE_ID = "pipeline-canvas-drop" as const;

interface PipelineCanvasProps {
  className?: string;
}

/**
 * Center column of the editor.
 *
 *  - Renders the operator chain as a vertical sortable list (`@dnd-kit/sortable`).
 *  - Relies on a `DndContext` mounted by the parent (route component); this
 *    keeps the library and canvas droppables in the same provider so a
 *    library card can drop onto the canvas, and an existing operator can
 *    be reordered, with a single `onDragEnd`.
 *  - Header / footer show input / output node counts; when no debug-trace
 *    has run yet (T-21 will populate this), placeholder text is shown.
 */
export function PipelineCanvas({ className }: PipelineCanvasProps) {
  const { t } = useTranslation(["pipeline"]);
  const operators = usePipelineEditorStore((s) => s.ast.operators);
  const selectedId = usePipelineEditorStore((s) => s.selectedOperatorId);
  const debugTrace = usePipelineEditorStore((s) => s.debugTrace);

  const removeOperator = usePipelineEditorStore((s) => s.removeOperator);
  const selectOperator = usePipelineEditorStore((s) => s.selectOperator);

  const { setNodeRef, isOver } = useDroppable({ id: CANVAS_DROPPABLE_ID });
  const isEmpty = operators.length === 0;

  return (
    <section
      data-testid="pipeline-canvas"
      ref={setNodeRef}
      className={cn(
        "flex h-full min-h-0 flex-1 flex-col gap-3 overflow-y-auto",
        "bg-[var(--color-bg)] p-4",
        isOver && "bg-[var(--color-surface-hover)]",
        className,
      )}
    >
      <CanvasInputNode label={t("pipeline:editor.canvas_input_node")} />
      <CanvasArrow
        label={t("pipeline:editor.canvas_run_to_view")}
        count={null}
      />

      {isEmpty ? (
        <CanvasEmptyState
          title={t("pipeline:editor.canvas_empty_title")}
          description={t("pipeline:editor.canvas_empty_description")}
        />
      ) : (
        <SortableContext
          items={operators.map((op) => op.id)}
          strategy={verticalListSortingStrategy}
        >
          <ol
            className="flex flex-col gap-3"
            data-testid="canvas-operator-list"
          >
            {operators.map((op, idx) => {
              const step = debugTrace?.steps?.[idx];
              return (
                <li key={op.id} className="flex flex-col gap-2">
                  <OperatorNode
                    operator={op}
                    selected={selectedId === op.id}
                    onSelect={selectOperator}
                    onRemove={removeOperator}
                  />
                  {idx < operators.length - 1 && (
                    <CanvasArrow
                      label={null}
                      count={
                        step
                          ? `${step.input_count} → ${step.output_count}`
                          : null
                      }
                    />
                  )}
                </li>
              );
            })}
          </ol>
        </SortableContext>
      )}

      <CanvasArrow
        label={null}
        count={
          debugTrace?.output_count != null
            ? String(debugTrace.output_count)
            : null
        }
      />
      <CanvasOutputNode
        label={t("pipeline:editor.canvas_output_node")}
        count={debugTrace?.output_count ?? null}
      />
    </section>
  );
}

/**
 * Helper invoked by the route component's shared `onDragEnd`. Encapsulates
 * the "library drop → addOperator" / "canvas reorder → reorderOperators"
 * decision so the route stays declarative.
 *
 * Returns the operator id to focus (newly added or moved) so the route can
 * route that into `selectOperator` if desired.
 */
export interface CanvasDragResolution {
  kind: "add" | "reorder" | "noop";
  /** For `add`: the operator id newly inserted (after the store applies it). */
  newOperatorId?: string;
}

export function resolveCanvasDragEnd(args: {
  activeId: string;
  overId: string | null;
  operators: PipelineOperator[];
  store: {
    addOperator: (
      type: ReturnType<typeof parseLibraryDraggableId> & string,
      insertIndex?: number,
    ) => string;
    reorderOperators: (fromIndex: number, toIndex: number) => void;
  };
}): CanvasDragResolution {
  const { activeId, overId, operators, store } = args;
  if (!overId) return { kind: "noop" };

  const libraryType = parseLibraryDraggableId(activeId);
  if (libraryType) {
    // Drop onto an existing operator → insert after it; drop on the empty
    // canvas droppable id → append.
    const targetIdx = operators.findIndex((op) => op.id === overId);
    const insertIndex =
      targetIdx >= 0 ? targetIdx + 1 : undefined; /* append */
    const id = store.addOperator(libraryType, insertIndex);
    return { kind: "add", newOperatorId: id };
  }

  if (activeId === overId) return { kind: "noop" };
  const fromIndex = operators.findIndex((op) => op.id === activeId);
  const toIndex = operators.findIndex((op) => op.id === overId);
  if (fromIndex < 0 || toIndex < 0) return { kind: "noop" };
  const moved = arrayMove(operators, fromIndex, toIndex);
  const newIndex = moved.findIndex((op) => op.id === activeId);
  store.reorderOperators(fromIndex, newIndex);
  return { kind: "reorder" };
}

// ── Internal building blocks ────────────────────────────────────────────────

function CanvasInputNode({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-3 self-center rounded-[var(--radius-md)] border border-dashed border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)] px-4 py-2">
      <Cog className="h-4 w-4 text-[var(--color-text-tertiary)]" aria-hidden />
      <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-secondary)]">
        {label}
      </span>
    </div>
  );
}

function CanvasOutputNode({
  label,
  count,
}: {
  label: string;
  count: number | null;
}) {
  return (
    <div className="flex items-center gap-3 self-center rounded-[var(--radius-md)] border border-dashed border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)] px-4 py-2">
      <Cog className="h-4 w-4 text-[var(--color-text-tertiary)]" aria-hidden />
      <span className="text-[var(--font-size-sm)] font-medium text-[var(--color-text-secondary)]">
        {label}
      </span>
      {count != null && (
        <span className="font-mono text-[var(--font-size-xs)] tabular-nums text-[var(--color-text-tertiary)]">
          {count}
        </span>
      )}
    </div>
  );
}

function CanvasArrow({
  label,
  count,
}: {
  label: string | null;
  count: string | null;
}) {
  return (
    <div className="flex flex-col items-center gap-1 text-[var(--color-text-tertiary)]">
      <ChevronDown className="h-3.5 w-3.5" aria-hidden />
      {count != null && (
        <span className="font-mono text-[var(--font-size-xs)] tabular-nums">
          {count}
        </span>
      )}
      {label && !count && (
        <span className="text-[var(--font-size-xs)]">{label}</span>
      )}
    </div>
  );
}

function CanvasEmptyState({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div
      data-testid="canvas-empty"
      className={cn(
        "mx-auto my-6 flex w-full max-w-md flex-col items-center justify-center gap-2 rounded-[var(--radius-lg)]",
        "border border-dashed border-[var(--color-border-strong)] bg-[var(--color-bg-elevated)] px-6 py-12 text-center",
      )}
    >
      <h3 className="text-[var(--font-size-sm)] font-semibold text-[var(--color-text-primary)]">
        {title}
      </h3>
      <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        {description}
      </p>
    </div>
  );
}
