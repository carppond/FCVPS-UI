import { createFileRoute, redirect } from "@tanstack/react-router";
import { useAuthStore } from "@/stores/auth-store";

/**
 * Root index "/" — pure redirect.
 *
 * Authenticated users go to `/dashboard`; everyone else lands on `/login`.
 * No UI is ever rendered here, so there is no user-facing copy to translate.
 */
export const Route = createFileRoute("/")({
  beforeLoad: () => {
    const { token, user } = useAuthStore.getState();
    if (token && user) {
      throw redirect({ to: "/dashboard" });
    }
    throw redirect({ to: "/login" });
  },
});
