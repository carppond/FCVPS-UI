import { cn } from "@/lib/cn";

interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {}

/** Placeholder skeleton with a pulse animation. */
function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div
      className={cn("animate-pulse rounded-[var(--radius-md)] bg-[var(--color-surface-hover)]", className)}
      {...props}
    />
  );
}

export { Skeleton };
