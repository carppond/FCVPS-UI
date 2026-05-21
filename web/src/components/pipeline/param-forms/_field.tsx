import * as React from "react";
import { cn } from "@/lib/cn";
import { Label } from "@/components/ui/label";

interface FieldProps {
  label: string;
  htmlFor?: string;
  help?: string;
  error?: string;
  children: React.ReactNode;
  className?: string;
}

/**
 * Shared field wrapper for all param forms — label + control + optional help
 * text and error message. Keeps spacing consistent (gap-1.5) so every form
 * stays on the same vertical rhythm.
 */
export function Field({
  label,
  htmlFor,
  help,
  error,
  children,
  className,
}: FieldProps) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {error ? (
        <p
          role="alert"
          className="text-[var(--font-size-xs)] text-[var(--color-error)]"
        >
          {error}
        </p>
      ) : (
        help && (
          <p className="text-[var(--font-size-xs)] text-[var(--color-text-tertiary)]">
            {help}
          </p>
        )
      )}
    </div>
  );
}

/**
 * Native <select> styled to match the project's design tokens. Avoids pulling
 * in the radix Select primitive for these compact param forms.
 */
export const Select = React.forwardRef<
  HTMLSelectElement,
  React.SelectHTMLAttributes<HTMLSelectElement>
>(({ className, children, ...props }, ref) => (
  <select
    ref={ref}
    className={cn(
      "flex h-9 w-full rounded-[var(--radius-md)] border border-[var(--color-border-strong)]",
      "bg-[var(--color-surface)] px-3 py-1 text-[var(--font-size-sm)] text-[var(--color-text-primary)]",
      "shadow-[var(--shadow-inset)]",
      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]",
      "disabled:cursor-not-allowed disabled:opacity-50",
      className,
    )}
    {...props}
  >
    {children}
  </select>
));
Select.displayName = "Select";
