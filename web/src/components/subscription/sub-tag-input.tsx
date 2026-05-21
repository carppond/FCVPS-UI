import * as React from "react";
import { X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/cn";
import { useSubscriptionTagSuggestionsQuery } from "@/api/subscription";

interface SubTagInputProps {
  value: string[];
  onChange: (next: string[]) => void;
  /** Suggestions sourced from existing subscriptions. Pass `false` to disable network fetch. */
  enableSuggestions?: boolean;
  placeholderKey?: string;
  className?: string;
  id?: string;
}

/**
 * Chip-style tag input.
 *
 *  - Enter or comma commits the current draft as a tag.
 *  - Backspace on an empty draft removes the last tag.
 *  - X on a chip removes it.
 *  - Suggestions come from `useSubscriptionTagSuggestionsQuery` (cached 5 min).
 */
export function SubTagInput({
  value,
  onChange,
  enableSuggestions = true,
  placeholderKey = "subscription:wizard.tags.placeholder",
  className,
  id,
}: SubTagInputProps) {
  const { t } = useTranslation(["subscription"]);
  const [draft, setDraft] = React.useState("");
  const suggestionsQuery = useSubscriptionTagSuggestionsQuery();

  const suggestions = React.useMemo(() => {
    if (!enableSuggestions) return [] as string[];
    const all = suggestionsQuery.data ?? [];
    const lower = draft.toLowerCase().trim();
    return all
      .filter((tag) => !value.includes(tag))
      .filter((tag) => (lower ? tag.toLowerCase().includes(lower) : true))
      .slice(0, 8);
  }, [enableSuggestions, suggestionsQuery.data, draft, value]);

  const commit = (raw: string) => {
    const next = raw.trim();
    if (!next) return;
    if (value.includes(next)) {
      setDraft("");
      return;
    }
    onChange([...value, next]);
    setDraft("");
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commit(draft);
    } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  };

  const remove = (tag: string) => onChange(value.filter((v) => v !== tag));

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <div className="flex flex-wrap items-center gap-1.5 rounded-[var(--radius-md)] border border-[var(--color-border-strong)] bg-[var(--color-surface)] p-2">
        {value.map((tag) => (
          <Badge
            key={tag}
            variant="outline"
            className="flex items-center gap-1 bg-[var(--color-surface-hover)]"
          >
            <span>{tag}</span>
            <button
              type="button"
              onClick={() => remove(tag)}
              className="rounded-[var(--radius-sm)] p-0.5 hover:bg-[var(--color-bg-elevated)]"
              aria-label={`remove ${tag}`}
            >
              <X className="h-3 w-3" />
            </button>
          </Badge>
        ))}
        <Input
          id={id}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={() => commit(draft)}
          placeholder={t(placeholderKey)}
          className="h-7 flex-1 min-w-[8rem] border-0 bg-transparent px-1 shadow-none focus-visible:ring-0"
        />
      </div>

      {enableSuggestions && suggestions.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {suggestions.map((tag) => (
            <button
              key={tag}
              type="button"
              onClick={() => commit(tag)}
              className={cn(
                "rounded-[var(--radius-sm)] border border-[var(--color-border)]",
                "bg-[var(--color-surface)] px-2 py-0.5 text-[var(--font-size-xs)]",
                "text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-hover)]",
                "transition-colors duration-[var(--duration-fast)]",
              )}
            >
              + {tag}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
