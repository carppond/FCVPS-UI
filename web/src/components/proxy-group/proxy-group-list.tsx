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
  useDeleteProxyGroup,
  useReorderProxyGroups,
} from "@/api/proxy-group";
import type { ProxyGroupCategory } from "@/types/api";

interface ProxyGroupListProps {
  items: ProxyGroupCategory[] | undefined;
  isLoading: boolean;
  isError: boolean;
  errorMessage?: string;
  onRetry?: () => void;
  onEdit: (g: ProxyGroupCategory) => void;
  onNew: () => void;
  className?: string;
}

/**
 * Sortable list of proxy groups. Each row exposes a drag handle that triggers
 * an optimistic local reorder; the server-side reorder endpoint is invoked
 * after the user drops, with the full id array used to recompute sort_order.
 */
export function ProxyGroupList({
  items,
  isLoading,
  isError,
  errorMessage,
  onRetry,
  onEdit,
  onNew,
  className,
}: ProxyGroupListProps) {
  const { t } = useTranslation(["proxy-group", "common"]);

  if (isLoading) return <ListSkeleton className={className} />;

  if (isError) {
    return (
      <div className={cn(className)}>
        <ErrorState
          message={errorMessage ?? t("proxy-group:error.load_failed")}
          onRetry={onRetry}
          retryLabel={t("common:actions.retry")}
        />
      </div>
    );
  }

  if (!items || items.length === 0) {
    return (
      <div className={cn("flex flex-1 items-center justify-center", className)}>
        <EmptyState
          title={t("proxy-group:list.empty_title")}
          description={t("proxy-group:list.empty_description")}
          ctaLabel={t("proxy-group:list.add")}
          onCta={onNew}
        />
      </div>
    );
  }

  return <DraggableList items={items} onEdit={onEdit} className={className} />;
}

interface DraggableListProps {
  items: ProxyGroupCategory[];
  onEdit: (g: ProxyGroupCategory) => void;
  className?: string;
}

function DraggableList({ items, onEdit, className }: DraggableListProps) {
  const { t } = useTranslation(["proxy-group", "common"]);
  const { handle: handleError } = useApiError();
  const reorderMutation = useReorderProxyGroups();

  const [local, setLocal] = React.useState(items);
  React.useEffect(() => {
    setLocal(items);
  }, [items]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
  );

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = local.findIndex((g) => g.id === active.id);
    const newIndex = local.findIndex((g) => g.id === over.id);
    if (oldIndex < 0 || newIndex < 0) return;
    const next = arrayMove(local, oldIndex, newIndex);
    setLocal(next);
    void reorderMutation
      .mutateAsync({ ids: next.map((g) => g.id) })
      .then(() => toast.success(t("proxy-group:toast.reorder_ok")))
      .catch((err) => handleError(err));
  };

  return (
    <div
      className={cn(
        "overflow-hidden rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)]",
        className,
      )}
      data-testid="proxy-group-list"
    >
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={onDragEnd}
      >
        <SortableContext
          items={local.map((g) => g.id)}
          strategy={verticalListSortingStrategy}
        >
          <ul className="divide-y divide-[var(--color-border)]">
            {local.map((g) => (
              <SortableGroupRow key={g.id} group={g} onEdit={onEdit} />
            ))}
          </ul>
        </SortableContext>
      </DndContext>
    </div>
  );
}

interface SortableGroupRowProps {
  group: ProxyGroupCategory;
  onEdit: (g: ProxyGroupCategory) => void;
}

function SortableGroupRow({ group, onEdit }: SortableGroupRowProps) {
  const { t } = useTranslation(["proxy-group", "common"]);
  const { handle: handleError } = useApiError();
  const deleteMutation = useDeleteProxyGroup();

  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: group.id });

  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const onDelete = async () => {
    if (!confirm(t("proxy-group:delete.confirm", { name: group.name }))) return;
    try {
      await deleteMutation.mutateAsync(group.id);
      toast.success(t("proxy-group:toast.delete_ok"));
    } catch (err) {
      handleError(err);
    }
  };

  const memberCount = group.member_proxies?.length ?? 0;
  const subGroupCount = group.member_groups?.length ?? 0;

  return (
    <li
      ref={setNodeRef}
      style={style}
      data-testid={`proxy-group-row-${group.id}`}
      className={cn(
        "flex items-center gap-3 px-4 py-3",
        "hover:bg-[var(--color-surface-hover)] transition-colors duration-[var(--duration-fast)]",
      )}
    >
      <span
        {...attributes}
        {...listeners}
        className="inline-flex cursor-grab text-[var(--color-text-tertiary)] active:cursor-grabbing"
        aria-label={t("proxy-group:list.drag_hint")}
      >
        <GripVertical className="h-4 w-4" />
      </span>

      <span className="flex h-8 w-8 shrink-0 items-center justify-center text-base">
        {group.icon || "·"}
      </span>

      <button
        type="button"
        onClick={() => onEdit(group)}
        className={cn(
          "flex-1 min-w-0 text-left font-medium text-[var(--color-text-primary)]",
          "hover:text-[var(--color-primary)] transition-colors duration-[var(--duration-fast)]",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] rounded-[var(--radius-sm)]",
        )}
      >
        <span className="block truncate">{group.name}</span>
      </button>

      <Badge variant="outline">{group.type}</Badge>

      <span className="hidden sm:inline text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
        {group.include_all
          ? group.filter
            ? t("proxy-group:list.filter_label", { filter: group.filter })
            : t("proxy-group:list.include_all_label")
          : memberCount > 0
            ? t("proxy-group:list.members_count", { count: memberCount })
            : subGroupCount > 0
              ? t("proxy-group:list.groups_count", { count: subGroupCount })
              : "—"}
      </span>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            aria-label={t("common:aria.actions")}
          >
            <MoreHorizontal className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => onEdit(group)}>
            <Pencil className="mr-2 h-3.5 w-3.5" />
            {t("proxy-group:actions.edit")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onSelect={() => void onDelete()}
            className="text-[var(--color-error)] data-[highlighted]:text-[var(--color-error)]"
          >
            <Trash2 className="mr-2 h-3.5 w-3.5" />
            {t("proxy-group:actions.delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </li>
  );
}

function ListSkeleton({ className }: { className?: string }) {
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
            <Skeleton className="h-7 w-7 rounded-[var(--radius-md)]" />
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-5 w-16" />
            <Skeleton className="h-4 w-20" />
          </div>
        ))}
      </div>
    </div>
  );
}
