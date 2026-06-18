import { redirect } from "@tanstack/react-router";
import type { QueryClient } from "@tanstack/react-query";
import { apiFetch, ApiError } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { useAuthStore } from "@/stores/auth-store";
import type { User } from "@/types/api";

/**
 * Re-export of the "me" query key so route loaders and components share one
 * cache entry. Keep this in sync with the key used by `useMeQuery` in
 * `@/api/user`.
 */
export const meQueryKey = queryKeys.user.me();

/** Fetch `/api/me` directly (used by the route guard, outside React). */
function fetchMe(): Promise<User> {
  return apiFetch<User>("/api/me");
}

/**
 * Hydrate the auth-store from the access token stored in localStorage.
 *
 * Uses `queryClient.ensureQueryData` so the result is cached and re-used by
 * downstream `useMeQuery` calls without a second network round-trip.
 *
 * Returns the verified `User` on success. Throws a TanStack Router `redirect`
 * to `/login?next=<current>` on:
 *   - missing token (no session at all), or
 *   - 401 from `/api/me` (token expired / revoked).
 *
 * Re-throws any other error untouched so the route's `errorComponent` (or the
 * QueryCache global handler) can decide how to surface it — per §2.4 we do
 * NOT force a logout on network errors.
 */
export async function requireAuth(
  queryClient: QueryClient,
  href: string,
): Promise<User> {
  const store = useAuthStore.getState();

  // Auth is the httpOnly sg_session cookie, which JS cannot read — so there's
  // no local token to short-circuit on. Always verify via /api/me; the browser
  // attaches the cookie automatically, and a 401 means "not logged in".
  try {
    const user = await queryClient.ensureQueryData<User>({
      queryKey: meQueryKey,
      queryFn: fetchMe,
      // staleTime 0 ensures that after an explicit hydration request we always
      // hit the server to verify the session is still valid.
      staleTime: 0,
    });
    // Keep auth-store user in sync with the verified `/api/me` payload.
    if (!store.user || store.user.id !== user.id) {
      store.setSession(user);
    }
    return user;
  } catch (err) {
    // 401 → clear session + bounce to login with `next` so the user lands back
    // on the protected route after re-authenticating.
    if (err instanceof ApiError && err.status === 401) {
      // apiFetch already cleared the auth-store; remove the stale cached
      // /api/me entry too so a future login starts fresh.
      queryClient.removeQueries({ queryKey: meQueryKey });
      throw redirect({
        to: "/login",
        search: { next: href },
      });
    }
    // Re-throw network / 5xx errors — handled by errorComponent + global toast.
    throw err;
  }
}
