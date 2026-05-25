import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/cn";

const badgeVariants = cva(
  [
    "inline-flex items-center rounded-[var(--radius-sm)] px-[9px] py-[3px]",
    "text-[11px] leading-[1.4] font-medium transition-colors duration-[var(--duration-fast)]",
    "focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:ring-offset-2",
  ],
  {
    variants: {
      variant: {
        default: "bg-[var(--color-primary)] text-[var(--color-primary-foreground)]",
        secondary: "bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)]",
        destructive: "bg-[var(--color-error-bg)] text-[var(--color-error)]",
        success: "bg-[var(--color-success-bg)] text-[var(--color-success)]",
        warning: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
        info: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
        outline:
          "border border-[var(--color-border-strong)] text-[var(--color-text-secondary)] bg-transparent",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { Badge, badgeVariants };
