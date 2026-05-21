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
import { GripVertical, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
  /** Open the edit dialog for a rule. */
  onEdit: (rule: CustomRule) => void;
  /** Open the "new rule" dialog (used by the empty state CTA). */
  onNew: () => void;
  className?: string;
}

/**
 * Single-column data table for rules.
 *
 * Each row carries a drag handle, type/mode badges, the name (click to edit),
 * an enable toggle, and a "⋯" overflow menu for edit/delete. The list owns
 * its own optimistic drag-reorder; the parent only cares about which rule
 * the user wants to edit / create.
 */
export function RuleList({
  rules,
  isLoading,
  isError,
  errorMessage,
  onRetry,
  onEdit,
  onNew,
  className,
}: RuleListProps) {
  const { t } = useTranslation(["rule", "common"]);

  if (isLoading) return <RuleTableSkeleton className={className} />;

  if (isError) {
    return (
      <div className={cn(className)}>
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
      <div className={cn("flex flex-1 items-center justify-center", className)}>
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
    <DraggableRuleTable
      rules={rules}
      onEdit={onEdit}
      className={className}
    />
  );
}

interface DraggableRuleTableProps {
  rules: CustomRule[];
  onEdit: (rule: CustomRule) => void;
  className?: string;
}

function DraggableRuleTable({ rules, onEdit, className }: DraggableRuleTableProps) {
  const { t } = useTranslation(["rule", "common"]);
  const { handle: handleError } = useApiError();
  const reorderMutation = useReorderRulesMutation();

  // Local copy of order so the drag preview feels immediate; the server
  // response invalidates the query which restores canonical order.
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
    const orders = next.map((r, idx) => ({ id: r.id, sort: (idx + 1) * 100 }));
    void reorderMutation
      .mutateAsync({ orders })
      .then(() => toast.success(t("rule:toast.reorder_ok")))
      .catch((err) => handleError(err));
  };

  return (
    <div
      className={cn(
        "overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]",
        className,
      )}
    >
      <table className="w-full text-[var(--font-size-sm)]" data-testid="rule-list">
        <thead className="border-b border-[var(--color-border)] text-[var(--color-text-tertiary)]">
          <tr>
            <Th className="w-10" />
            <Th className="w-24">{t("rule:form.type_label")}</Th>
            <Th className="w-24">{t("rule:form.mode_label")}</Th>
            <Th>{t("rule:form.name_label")}</Th>
            <Th className="w-24 text-center">{t("rule:form.enabled_label")}</Th>
            <Th className="w-12 text-right">
              <span className="sr-only">{t("common:actions.edit")}</span>
            </Th>
          </tr>
        </thead>
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={onDragEnd}
        >
          <SortableContext
            items={items.map((r) => r.id)}
            strategy={verticalListSortingStrategy}
          >
            <tbody>
              {items.map((rule) => (
                <SortableRuleRow
                  key={rule.id}
                  rule={rule}
                  onEdit={onEdit}
                />
              ))}
            </tbody>
          </SortableContext>
        </DndContext>
      </table>
    </div>
  );
}

interface SortableRuleRowProps {
  rule: CustomRule;
  onEdit: (rule: CustomRule) => void;
}

function SortableRuleRow({ rule, onEdit }: SortableRuleRowProps) {
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

  const toggleEnabled = async (e: React.ChangeEvent<HTMLInputElement>) => {
    try {
      await updateMutation.mutateAsync({
        id: rule.id,
        payload: { enabled: e.target.checked },
      });
    } catch (err) {
      handleError(err);
    }
  };

  const removeRule = async () => {
    if (!confirm(t("rule:delete.dialog_description", { name: rule.name }))) {
      return;
    }
    try {
      await deleteMutation.mutateAsync(rule.id);
      toast.success(t("rule:toast.delete_ok"));
    } catch (err) {
      handleError(err);
    }
  };

  return (
    <tr
      ref={setNodeRef}
      style={style}
      data-testid={`rule-row-${rule.id}`}
      className={cn(
        "border-b border-[var(--color-border)] last:border-0",
        "hover:bg-[var(--color-surface-hover)] transition-colors duration-[var(--duration-fast)]",
      )}
    >
      <Td className="w-10">
        <span
          {...attributes}
          {...listeners}
          className="inline-flex cursor-grab text-[var(--color-text-tertiary)] active:cursor-grabbing"
          aria-label={t("rule:list.drag_hint")}
        >
          <GripVertical className="h-4 w-4" />
        </span>
      </Td>
      <Td>
        <Badge variant="outline">{typeLabel(t, rule.type)}</Badge>
      </Td>
      <Td>
        <Badge variant="secondary">{t(`rule:modes.${rule.mode}`)}</Badge>
      </Td>
      <Td>
        <button
          type="button"
          onClick={() => onEdit(rule)}
          className={cn(
            "truncate text-left font-medium",
            "hover:text-[var(--color-primary)] transition-colors duration-[var(--duration-fast)]",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] rounded-[var(--radius-sm)]",
            rule.enabled
              ? "text-[var(--color-text-primary)]"
              : "text-[var(--color-text-tertiary)] line-through",
          )}
        >
          {rule.name}
        </button>
      </Td>
      <Td className="text-center">
        <input
          type="checkbox"
          checked={rule.enabled}
          onChange={toggleEnabled}
          className="h-4 w-4 rounded border-[var(--color-border-strong)] text-[var(--color-primary)] focus:ring-[var(--color-primary)]"
          aria-label={t("rule:form.enabled_label")}
        />
      </Td>
      <Td className="text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("common:actions.edit")}
            >
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => onEdit(rule)}>
              <Pencil className="mr-2 h-3.5 w-3.5" />
              {t("common:actions.edit")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onSelect={() => void removeRule()}
              className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
            >
              <Trash2 className="mr-2 h-3.5 w-3.5" />
              {t("rule:delete.confirm")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </Td>
    </tr>
  );
}

function Th({ children, className }: { children?: React.ReactNode; className?: string }) {
  return (
    <th
      className={cn(
        "px-4 py-2.5 text-left text-[var(--font-size-xs)] font-medium uppercase tracking-wide",
        className,
      )}
    >
      {children}
    </th>
  );
}

function Td({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <td
      className={cn(
        "px-4 py-3 align-middle text-[var(--color-text-primary)]",
        className,
      )}
    >
      {children}
    </td>
  );
}

function RuleTableSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4",
        className,
      )}
      aria-hidden
    >
      <div className="flex flex-col gap-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center gap-3">
            <Skeleton className="h-4 w-4" />
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-4 w-10" />
          </div>
        ))}
      </div>
    </div>
  );
}

function typeLabel(t: (key: string) => string, type: RuleType): string {
  // i18n keys contain a "-" so the t() helper needs the safe lookup form.
  if (type === "rule-providers") {
    return t("rule:types.rule-providers");
  }
  return t(`rule:types.${type}`);
}
