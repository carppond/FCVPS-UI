import { createFileRoute, Outlet } from "@tanstack/react-router";
import { AppShell } from "@/components/layout/app-shell";
import { useAuthStore } from "@/stores/auth-store";

export const Route = createFileRoute("/_authed")({
  // TODO(T-6): Implement real auth guard that checks token validity and expiry.
  // Currently passes through to allow rendering the empty AppShell.
  beforeLoad: () => {
    const token = useAuthStore.getState().token;
    // Placeholder: guard will redirect to login when token is absent.
    // throw redirect({ to: "/_public/login" });
    void token;
  },
  component: AuthedLayout,
});

function AuthedLayout() {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}
