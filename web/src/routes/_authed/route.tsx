import { createFileRoute, Outlet } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { AppShell } from "@/components/layout/app-shell";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api-client";
import { requireAuth } from "@/lib/auth-guard";
import { queryClient } from "@/lib/query-client";

/**
 * `_authed` layout route — every child route requires a valid session.
 *
 * `beforeLoad` runs the auth guard (see `@/lib/auth-guard.ts`):
 *  1. No token → redirect to `/login?next=<current>`.
 *  2. Token present → `ensureQueryData('/api/me')` verifies it server-side.
 *  3. 401 from `/api/me` → clear session + redirect (with `next`).
 *  4. Network / 5xx error → re-thrown so `errorComponent` shows the offline
 *     placeholder with a retry button (per docs §2.4 — do NOT force logout).
 *
 * The verified `User` is returned and made available to nested route loaders /
 * components via TanStack Router's route context (`route.useRouteContext()`).
 */
export const Route = createFileRoute("/_authed")({
  beforeLoad: async ({ location }) => {
    const user = await requireAuth(queryClient, location.href);
    return { user };
  },
  component: AuthedLayout,
  errorComponent: AuthedErrorComponent,
});

function AuthedLayout() {
  return (
    <AppShell>
      <Outlet />
    </AppShell>
  );
}

/**
 * Shown when the guard re-throws a non-401 error (typically a network failure
 * during `/api/me`). Per §2.4 we keep the user "logged in" locally and offer a
 * retry — the most common case is a flaky connection on page refresh.
 */
function AuthedErrorComponent({
  error,
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  const { t } = useTranslation(["auth", "common"]);

  const isOffline =
    error instanceof ApiError &&
    (error.status === 0 || error.code === "INTERNAL_NETWORK");
  const isAuthError =
    error instanceof ApiError &&
    typeof error.code === "string" &&
    error.code.startsWith("AUTH_");

  // Only the offline / auth flavours get the friendly "session lost" copy —
  // every other error is a child-component crash that the user (and us)
  // need to see literally. Showing "couldn't load your session" for a
  // proxy-group form crash was actively misleading.
  const showFriendly = isOffline || isAuthError;
  const heading = isOffline
    ? t("auth:guard.offline_title")
    : isAuthError
      ? t("auth:guard.error_title")
      : t("auth:guard.error_title");
  const body = isOffline
    ? t("auth:guard.offline_description")
    : isAuthError
      ? t("auth:guard.error_description")
      : null;

  return (
    <div className="flex min-h-screen items-center justify-center bg-[var(--color-bg)] p-6">
      <div className="w-full max-w-2xl">
        <h2 className="text-[var(--font-size-xl)] font-semibold text-[var(--color-text-primary)]">
          {heading}
        </h2>
        {body ? (
          <p className="mt-2 text-[var(--font-size-sm)] text-[var(--color-text-tertiary)]">
            {body}
          </p>
        ) : null}
        {!showFriendly ? (
          <pre className="mt-4 max-h-80 overflow-auto rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface)] p-3 text-left text-[var(--font-size-xs)] font-mono text-[var(--color-text-secondary)]">
            {error.message || String(error)}
            {error.stack ? "\n\n" + error.stack : ""}
          </pre>
        ) : null}
        <div className="mt-6">
          <Button onClick={reset}>{t("common:actions.retry")}</Button>
        </div>
      </div>
    </div>
  );
}
