import * as React from "react";
import { cn } from "@/lib/cn";

interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  /**
   * 0..100 — the percentage to render. Values are clamped; `undefined` flips
   * the bar into an indeterminate animation (no known total). The Tailwind
   * design tokens (radius / colours / motion) come from globals.css so this
   * component stays purely structural.
   */
  value?: number;
  /** Optional aria label so the bar is announced to assistive tech. */
  label?: string;
}

/**
 * Linear progress bar matching the Minimalism / Swiss visual language used
 * elsewhere in the admin shell. Determinate mode renders a fill at `value`%;
 * indeterminate mode renders an animated stripe so the OTA "verifying" stage
 * still feels alive even though there is no per-byte tracker.
 */
function Progress({ value, label, className, ...props }: ProgressProps) {
  const determinate = typeof value === "number" && !Number.isNaN(value);
  const pct = determinate ? Math.max(0, Math.min(100, value!)) : 0;
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={determinate ? pct : undefined}
      aria-label={label}
      className={cn(
        "relative h-2 w-full overflow-hidden rounded-[var(--radius-full)] bg-[var(--color-surface-hover)]",
        className,
      )}
      {...props}
    >
      {determinate ? (
        <div
          className="h-full rounded-[var(--radius-full)] bg-[var(--color-primary)] transition-[width] duration-[var(--duration-normal)]"
          style={{ width: `${pct}%` }}
        />
      ) : (
        <div
          className={cn(
            "absolute inset-y-0 left-0 h-full w-1/3 rounded-[var(--radius-full)] bg-[var(--color-primary)]",
            "animate-[progress-indeterminate_1.4s_ease-in-out_infinite]",
          )}
        />
      )}
    </div>
  );
}

export { Progress };
