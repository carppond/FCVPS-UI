import * as React from "react";
import { useTranslation } from "react-i18next";
import {
  DndContext,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { toast } from "@/components/ui/toast";
import { useApiError } from "@/hooks/use-api-error";
import { cn } from "@/lib/cn";
import {
  useDeleteRuleMutation,
  useReorderRulesMutation,
  useUpdateRuleMutation,
} from "@/api/rule";
import type { CustomRule, RuleType } from "@/types/api";

interface RuleListProps {
  rules: CustomRule[] | undefined;
  isLoading: boolean;
  isError: boolean;
  errorMessage?: string;
  onRetry?: () => void;
  selectedId: string | null;
  onSelect: (rule: CustomRule | null) => void;
  onNew: () => void;
  className?: string;
}

/**
 * Vertically scrollable list of rules. Uses @dnd-kit/sortable for drag-based
 * reordering (vertical strategy) and a checkbox toggle for the enabled flag.
 *
 * Each row is a card-style entry with the rule name, type / mode badges and a
 * delete icon. Selection state is owned by the parent so the form panel can
 * stay in sync.
 */
export function RuleList({
  rules,
  isLoading,
  isError,
  errorMessage,
  onRetry,
  selectedId,
  onSelect,
  onNew,
  className,
}: RuleListProps) {
  const { t } = useTranslation(["rule", "common"]);

  if (isLoading) {
    return (
      <div className={cn("flex flex-col gap-2 p-4", className)} aria-hidden>
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-14 w-full rounded-[var(--radius-md)]" />
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <div className={cn("p-4", className)}>
        <ErrorState
          message={errorMessage ?? t("rule:error.load_failed")}
          onRetry={onRetry}
          retryLabel={t("common:actions.retry")}
        />
      </div>
    );
  }

  if (!rules || rules.length === 0) {
    return (
      <div className={cn("flex flex-1 items-center justify-center p-4", className)}>
        <EmptyState
          title={t("rule:list.empty_title")}
          description={t("rule:list.empty_description")}
          ctaLabel={t("rule:list.add_rule")}
          onCta={onNew}
        />
      </div>
    );
  }

  return (
    <DraggableRuleList
      rules={rules}
      selectedId={selectedId}
      onSelect={onSelect}
      className={className}
    />
  );
}

interface DraggableRuleListProps {
  rules: CustomRule[];
  selectedId: string | null;
  onSelect: (rule: CustomRule | null) => void;
  className?: string;
}

function DraggableRuleList({
  rules,
  selectedId,
  onSelect,
  className,
}: DraggableRuleListProps) {
  const { t } = useTranslation(["rule"]);
  const { handle: handleError } = useApiError();
  const reorderMutation = useReorderRulesMutation();

  // Maintain a local copy of order so the drag preview feels immediate; the
  // server response invalidates the query which restores canonical order.
  const [items, setItems] = React.useState(rules);
  React.useEffect(() => {
    setItems(rules);
  }, [rules]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
  );

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = items.findIndex((r) => r.id === active.id);
    const newIndex = items.findIndex((r) => r.id === over.id);
    if (oldIndex < 0 || newIndex < 0) return;
    const next = arrayMove(items, oldIndex, newIndex);
    setItems(next);
    // Re-assign sort values in 100-step increments (matches handler default).
    const orders = next.map((r, idx) => ({ id: r.id, sort: (idx + 1) * 100 }));
    void reorderMutation
      .mutateAsync({ orders })
      .then(() => toast.success(t("rule:toast.reorder_ok")))
      .catch((err) => handleError(err));
  };

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragEnd={onDragEnd}
    >
      <SortableContext
        items={items.map((r) => r.id)}
        strategy={verticalListSortingStrategy}
      >
        <ul
          className={cn("flex flex-col gap-2 p-4", className)}
          data-testid="rule-list"
        >
          {items.map((rule) => (
            <SortableRuleRow
              key={rule.id}
              rule={rule}
              selected={rule.id === selectedId}
              onSelect={onSelect}
            />
          ))}
        </ul>
      </SortableContext>
    </DndContext>
  );
}

interface SortableRuleRowProps {
  rule: CustomRule;
  selected: boolean;
  onSelect: (rule: CustomRule | null) => void;
}

function SortableRuleRow({ rule, selected, onSelect }: SortableRuleRowProps) {
  const { t } = useTranslation(["rule", "common"]);
  const { handle: handleError } = useApiError();
  const updateMutation = useUpdateRuleMutation();
  const deleteMutation = useDeleteRuleMutation();

  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: rule.id });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const toggleEnabled = async (e: React.MouseEvent | React.ChangeEvent) => {
    e.stopPropagation();
    try {
      await updateMutation.mutateAsync({
        id: rule.id,
        payload: { enabled: !rule.enabled },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const removeRule = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm(t("rule:delete.dialog_description", { name: rule.name }))) {
      return;
    }
    try {
      await deleteMutation.mutateAsync(rule.id);
      toast.success(t("rule:toast.delete_ok"));
      if (selected) onSelect(null);
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <li ref={setNodeRef} style={style}>
      <div
        role="button"
        tabIndex={0}
        onClick={() => onSelect(rule)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onSelect(rule);
          }
        }}
        data-testid={`rule-row-${rule.id}`}
        className={cn(
          "group relative flex w-full items-center gap-2 rounded-[var(--radius-md)] border p-3 text-left",
          "transition-colors duration-[var(--duration-fast)] cursor-pointer",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
          selected
            ? "border-[var(--color-primary)] bg-[var(--color-surface-hover)]"
            : "border-[var(--color-border)] bg-[var(--color-surface)] hover:border-[var(--color-border-strong)] hover:bg-[var(--color-surface-hover)]",
        )}
      >
        {selected && (
          <span
            aria-hidden
            className="absolute left-0 top-2 bottom-2 w-0.5 rounded-full bg-[var(--color-primary)]"
          />
        )}
        <span
          {...attributes}
          {...listeners}
          className="shrink-0 cursor-grab text-[var(--color-text-tertiary)] active:cursor-grabbing"
          aria-label={t("rule:list.drag_hint")}
          onClick={(e) => e.stopPropagation()}
        >
          <GripVertical className="h-4 w-4" />
        </span>
        <div className="flex flex-1 flex-col gap-1.5 overflow-hidden">
          <div className="flex items-center gap-2">
            <span
              className={cn(
                "truncate text-[var(--font-size-sm)] font-medium",
                rule.enabled
                  ? "text-[var(--color-text-primary)]"
                  : "text-[var(--color-text-tertiary)] line-through",
              )}
            >
              {rule.name}
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="outline">{typeLabel(t, rule.type)}</Badge>
            <Badge variant="secondary">{t(`rule:modes.${rule.mode}`)}</Badge>
          </div>
        </div>
        <input
          type="checkbox"
          checked={rule.enabled}
          onChange={toggleEnabled}
          onClick={(e) => e.stopPropagation()}
          className="h-4 w-4 shrink-0 rounded border-[var(--color-border-strong)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]"
          aria-label={t("rule:form.enabled_label")}
        />
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={removeRule}
          aria-label={t("rule:delete.confirm")}
          className="shrink-0 text-[var(--color-text-tertiary)] opacity-0 transition-opacity duration-[var(--duration-fast)] group-hover:opacity-100 hover:text-[var(--color-error)]"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
    </li>
  );
}

function typeLabel(t: (key: string) => string, type: RuleType): string {
  // i18n keys contain a "-" so the t() helper needs the safe lookup form.
  if (type === "rule-providers") {
    return t("rule:types.rule-providers");
  }
  return t(`rule:types.${type}`);
}
