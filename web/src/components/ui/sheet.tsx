import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";

/**
 * Sheet — side drawer built on Radix Dialog.
 *
 * Used for transient detail panes (rule preview, etc.) where a full-blown
 * modal would over-constrain the layout. Default side is `right`; `left` is
 * available for nav drawers.
 *
 * Keep the API surface in lock-step with `dialog.tsx` so consumers can swap
 * the two with minimal churn.
 */
const Sheet = DialogPrimitive.Root;
const SheetTrigger = DialogPrimitive.Trigger;
const SheetClose = DialogPrimitive.Close;
const SheetPortal = DialogPrimitive.Portal;

const SheetOverlay = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Overlay
    ref={ref}
    className={cn(
      "fixed inset-0 z-[var(--z-overlay)] bg-black/60",
      "data-[state=open]:animate-in data-[state=closed]:animate-out",
      "data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0",
      className,
    )}
    {...props}
  />
));
SheetOverlay.displayName = "SheetOverlay";

interface SheetContentProps
  extends React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> {
  side?: "right" | "left";
}

const SheetContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  SheetContentProps
>(({ side = "right", className, children, ...props }, ref) => (
  <SheetPortal>
    <SheetOverlay />
    <DialogPrimitive.Content
      ref={ref}
      className={cn(
        "fixed inset-y-0 z-[var(--z-modal)] flex flex-col gap-4",
        "bg-[var(--color-bg-elevated)] shadow-[var(--shadow-lg)]",
        "transition ease-in-out duration-[var(--duration-normal)]",
        "data-[state=open]:animate-in data-[state=closed]:animate-out",
        side === "right" && [
          "right-0 w-full sm:max-w-2xl border-l border-[var(--color-border)]",
          "data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right",
        ],
        side === "left" && [
          "left-0 w-full sm:max-w-md border-r border-[var(--color-border)]",
          "data-[state=closed]:slide-out-to-left data-[state=open]:slide-in-from-left",
        ],
        className,
      )}
      {...props}
    >
      {children}
      <DialogPrimitive.Close
        className={cn(
          "absolute right-4 top-4 rounded-[var(--radius-sm)] opacity-70",
          "hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)]",
          "disabled:pointer-events-none",
        )}
      >
        <X className="h-4 w-4 text-[var(--color-text-tertiary)]" />
        <span className="sr-only">Close</span>
      </DialogPrimitive.Close>
    </DialogPrimitive.Content>
  </SheetPortal>
));
SheetContent.displayName = "SheetContent";

const SheetHeader = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      "flex flex-col gap-1.5 border-b border-[var(--color-border)]",
      "px-6 py-4 text-left",
      className,
    )}
    {...props}
  />
);
SheetHeader.displayName = "SheetHeader";

const SheetFooter = ({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div
    className={cn(
      "flex flex-col-reverse gap-2 border-t border-[var(--color-border)]",
      "px-6 py-3 sm:flex-row sm:justify-end",
      className,
    )}
    {...props}
  />
);
SheetFooter.displayName = "SheetFooter";

const SheetTitle = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Title
    ref={ref}
    className={cn(
      "text-[var(--font-size-lg)] font-semibold leading-[var(--line-height-tight)]",
      "text-[var(--color-text-primary)]",
      className,
    )}
    {...props}
  />
));
SheetTitle.displayName = "SheetTitle";

const SheetDescription = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => (
  <DialogPrimitive.Description
    ref={ref}
    className={cn(
      "text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]",
      className,
    )}
    {...props}
  />
));
SheetDescription.displayName = "SheetDescription";

export {
  Sheet,
  SheetTrigger,
  SheetClose,
  SheetPortal,
  SheetOverlay,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetDescription,
};
