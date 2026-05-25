import * as React from "react";
import { cn } from "@/lib/cn";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

const Input = React.forwardRef<HTMLInputElement, InputProps>(({ className, type, ...props }, ref) => {
  return (
    <input
      type={type}
      className={cn(
        "flex h-9 w-full rounded-[var(--radius-md)] border border-[var(--color-border-strong)]",
        "bg-[var(--color-surface-hover)] px-3 py-1 text-[var(--font-size-sm)] text-[var(--color-text-primary)]",
        "backdrop-blur-md",
        "placeholder:text-[var(--color-text-tertiary)]",
        "transition-colors duration-[var(--duration-fast)]",
        "focus-visible:border-[var(--color-primary)] focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-[var(--color-primary)]/15 focus-visible:ring-offset-0",
        "disabled:cursor-not-allowed disabled:opacity-50",
        "file:border-0 file:bg-transparent file:text-sm file:font-medium",
        className,
      )}
      ref={ref}
      {...props}
    />
  );
});
Input.displayName = "Input";

export { Input };
