import * as React from "react";
import { useDraggable } from "@dnd-kit/core";
import { useTranslation } from "react-i18next";
import { Search } from "lucide-react";
import { cn } from "@/lib/cn";
import { Input } from "@/components/ui/input";
import {
  OPERATOR_META,
  type OperatorMeta,
} from "@/components/pipeline/operator-meta";
import type { OperatorType } from "@/types/api";

/**
 * Stable id-prefix for draggable library items. The canvas uses this to
 * distinguish "spawn a new operator" drags from "reorder existing operator"
 * drags (which carry the operator instance id as the draggable id).
 */
export const LIBRARY_DRAGGABLE_PREFIX = "library:" as const;

/** Synthetic id used by `useDraggable` for an operator type. */
export function libraryDraggableId(type: OperatorType): string {
  return `${LIBRARY_DRAGGABLE_PREFIX}${type}`;
}

/** Recover the OperatorType from a library draggable id (or null). */
export function parseLibraryDraggableId(id: string | null): OperatorType | null {
  if (!id || !id.startsWith(LIBRARY_DRAGGABLE_PREFIX)) return null;
  return id.slice(LIBRARY_DRAGGABLE_PREFIX.length) as OperatorType;
}

interface OperatorLibraryProps {
  /** Optional: clicking a card adds the operator directly (keyboard-first UX). */
  onClickAdd?: (type: OperatorType) => void;
  className?: string;
}

/**
 * Left-rail palette of the 6 operator types.
 *  - Each card is draggable via @dnd-kit/core.
 *  - A search input filters cards by translated name / description.
 *  - Cards expose a click handler so the canvas can support
 *    "click-to-append" in addition to drag-and-drop (a11y).
 */
export function OperatorLibrary({ onClickAdd, className }: OperatorLibraryProps) {
  const { t } = useTranslation(["pipeline"]);
  const [query, setQuery] = React.useState("");

  const visible = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return OPERATOR_META;
    return OPERATOR_META.filter((meta) => {
      const name = t(meta.nameKey).toLowerCase();
      const desc = t(meta.descriptionKey).toLowerCase();
      return (
        meta.id.includes(q) || name.includes(q) || desc.includes(q)
      );
    });
  }, [query, t]);

  return (
    <aside
      data-testid="operator-library"
      className={cn(
        "flex h-full w-60 shrink-0 flex-col gap-3 border-r border-[var(--color-border)]",
        "bg-[var(--color-bg-elevated)] p-3",
        className,
      )}
    >
      <div className="flex items-center justify-between">
        <h2 className="text-[var(--font-size-sm)] font-semibold uppercase tracking-wide text-[var(--color-text-tertiary)]">
          {t("pipeline:editor.library_title")}
        </h2>
      </div>

      <div className="relative">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-[var(--color-text-tertiary)]"
          aria-hidden
        />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("pipeline:editor.library_search")}
          className="h-8 pl-8 text-[var(--font-size-sm)]"
          data-testid="operator-library-search"
        />
      </div>

      <div className="flex flex-col gap-2 overflow-y-auto pr-1">
        {visible.map((meta) => (
          <LibraryCard key={meta.id} meta={meta} onClick={onClickAdd} />
        ))}
        {visible.length === 0 && (
          <p className="px-2 py-6 text-center text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {t("common:no_data", { defaultValue: "—" })}
          </p>
        )}
      </div>
    </aside>
  );
}

interface LibraryCardProps {
  meta: OperatorMeta;
  onClick?: (type: OperatorType) => void;
}

function LibraryCard({ meta, onClick }: LibraryCardProps) {
  const { t } = useTranslation(["pipeline"]);
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: libraryDraggableId(meta.id),
    data: { kind: "library", operatorType: meta.id },
  });

  const Icon = meta.iconComponent;

  return (
    <button
      ref={setNodeRef}
      type="button"
      data-testid={`operator-library-card-${meta.id}`}
      data-operator-type={meta.id}
      onClick={() => onClick?.(meta.id)}
      className={cn(
        "group flex w-full items-start gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)]",
        "bg-[var(--color-surface)] px-3 py-2 text-left",
        "transition-colors duration-[var(--duration-fast)]",
        "hover:border-[var(--color-primary)] hover:bg-[var(--color-surface-hover)]",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
        "cursor-grab active:cursor-grabbing",
        isDragging && "opacity-50",
      )}
      {...listeners}
      {...attributes}
    >
      <span
        className={cn(
          "flex h-7 w-7 shrink-0 items-center justify-center rounded-[var(--radius-sm)]",
          "bg-[var(--color-bg-elevated)] text-[var(--color-primary)]",
          "group-hover:bg-[var(--color-surface-hover)]",
        )}
        aria-hidden
      >
        <Icon className="h-4 w-4" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)]">
          {t(meta.nameKey)}
        </span>
        <span className="line-clamp-2 text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
          {t(meta.descriptionKey)}
        </span>
      </span>
    </button>
  );
}
