import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/_authed/dashboard")({
  component: DashboardPlaceholder,
});

/** Dashboard placeholder — full implementation in T-31. */
function DashboardPlaceholder() {
  return (
    <div className="flex items-center justify-center py-24">
      <p className="text-[var(--color-text-tertiary)]">Dashboard placeholder</p>
    </div>
  );
}
