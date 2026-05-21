import * as React from "react";
import { X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/cn";

/**
 * Generic chip list input used by dedupe / future multi-value forms.
 *
 *  - Press Enter to commit the current input value as a chip.
 *  - Click the × to remove a chip (a11y label uses i18n key
 *    `pipeline:param_form.dedupe.remove_field`).
 *  - Duplicate values are silently ignored so the on-disk AST stays clean.
 */
export interface ChipListProps {
  value: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  /** i18n key used to build the per-chip aria-label; `{{field}}` interpolated. */
  removeLabelKey?: string;
  /** Optional id wired to a label-for. */
  id?: string;
  className?: string;
}

export function ChipList({
  value,
  onChange,
  placeholder,
  removeLabelKey = "pipeline:param_form.dedupe.remove_field",
  id,
  className,
}: ChipListProps) {
  const { t } = useTranslation(["pipeline"]);
  const [draft, setDraft] = React.useState("");

  const commit = React.useCallback(() => {
    const trimmed = draft.trim();
    if (!trimmed) return;
    if (value.includes(trimmed)) {
      setDraft("");
      return;
    }
    onChange([...value, trimmed]);
    setDraft("");
  }, [draft, value, onChange]);

  const remove = (idx: number) => {
    const next = value.slice();
    next.splice(idx, 1);
    onChange(next);
  };

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      {value.length > 0 && (
        <div className="flex flex-wrap gap-1.5" data-testid="chip-list">
          {value.map((chip, idx) => (
            <span
              key={`${chip}-${idx}`}
              data-testid={`chip-${chip}`}
              className={cn(
                "inline-flex items-center gap-1 rounded-[var(--radius-sm)]",
                "border border-[var(--color-border)] bg-[var(--color-surface)]",
                "px-2 py-0.5 text-[var(--font-size-xs)] text-[var(--color-text-primary)]",
              )}
            >
              <span className="font-mono">{chip}</span>
              <button
                type="button"
                aria-label={t(removeLabelKey, { field: chip })}
                onClick={() => remove(idx)}
                className="text-[var(--color-text-tertiary)] hover:text-[var(--color-error)]"
              >
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
      )}
      <Input
        id={id}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            commit();
          } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
            remove(value.length - 1);
          }
        }}
        onBlur={commit}
        placeholder={placeholder}
        data-testid="chip-list-input"
        className="h-8 text-[var(--font-size-sm)]"
      />
    </div>
  );
}
