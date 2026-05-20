import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/_public")({
  component: PublicLayout,
});

/** Public layout — no sidebar, centered container for auth pages. */
function PublicLayout() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-bg)]">
      <Outlet />
    </div>
  );
}
