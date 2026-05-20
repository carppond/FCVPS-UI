import { AlertCircle } from "lucide-react";
import { cn } from "@/lib/cn";
import { Button } from "@/components/ui/button";

interface ErrorStateProps {
  /** Error message to display. */
  message: string;
  /** Optional retry callback. */
  onRetry?: () => void;
  /** Label for the retry button (default: "Retry"). */
  retryLabel?: string;
  className?: string;
}

/**
 * Error banner displayed when a data-fetching operation fails.
 * Provides an optional retry button.
 */
export function ErrorState({ message, onRetry, retryLabel = "Retry", className }: ErrorStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-[var(--radius-lg)]",
        "border border-[var(--color-error-bg)] bg-[var(--color-error-bg)] p-6 text-center",
        className,
      )}
    >
      <AlertCircle className="h-8 w-8 text-[var(--color-error)]" />
      <p className="text-[var(--font-size-sm)] text-[var(--color-text-primary)]">{message}</p>
      {onRetry && (
        <Button variant="outline" size="sm" onClick={onRetry}>
          {retryLabel}
        </Button>
      )}
    </div>
  );
}
