import { createFileRoute, Outlet, redirect } from "@tanstack/react-router";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Public layout — no sidebar, centered container for auth pages.
 *
 * If the user already has a valid local session, bounce them straight to the
 * dashboard so they don't see the login form. The full token-validity check
 * is performed by the `_authed` guard once they land on a protected route.
 */
export const Route = createFileRoute("/_public")({
  beforeLoad: () => {
    const { token, user } = useAuthStore.getState();
    if (token && user) {
      throw redirect({ to: "/dashboard" });
    }
  },
  component: PublicLayout,
});

function PublicLayout() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-bg)] p-4">
      <Outlet />
    </div>
  );
}
