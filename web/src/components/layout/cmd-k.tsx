import { useEffect, useState } from "react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";

/**
 * Command palette placeholder.
 * Opens on Cmd+K / Ctrl+K; full implementation due in T-9.
 *
 * TODO(T-9): Replace with full command palette using cmdk.
 */
export function CmdK() {
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setIsOpen((prev) => !prev);
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, []);

  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogContent className="max-w-md">
        <DialogTitle className="sr-only">Command Palette</DialogTitle>
        <p className="py-8 text-center text-[var(--color-text-tertiary)] text-sm">
          {/* TODO(T-9): Command palette implementation */}
          Command palette coming soon (T-9)
        </p>
      </DialogContent>
    </Dialog>
  );
}
