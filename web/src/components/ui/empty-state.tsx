import * as React from "react";
import { cn } from "@/lib/cn";
import { Button } from "@/components/ui/button";

interface EmptyStateProps {
  /** Optional icon element to display above the title. */
  icon?: React.ReactNode;
  title: string;
  description?: string;
  /** Optional call-to-action label. */
  ctaLabel?: string;
  /** Callback for the CTA button. */
  onCta?: () => void;
  className?: string;
}

/**
 * Centered empty-state placeholder with optional icon, title,
 * description, and a call-to-action button.
 */
export function EmptyState({ icon, title, description, ctaLabel, onCta, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 py-16 text-center",
        className,
      )}
    >
      {icon && (
        <div className="text-[var(--color-text-disabled)] [&>svg]:h-12 [&>svg]:w-12">{icon}</div>
      )}
      <h3 className="text-[var(--font-size-lg)] font-semibold text-[var(--color-text-primary)]">
        {title}
      </h3>
      {description && (
        <p className="max-w-sm text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
          {description}
        </p>
      )}
      {ctaLabel && onCta && (
        <Button onClick={onCta} className="mt-2">
          {ctaLabel}
        </Button>
      )}
    </div>
  );
}
