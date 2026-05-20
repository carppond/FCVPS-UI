import {
  QueryCache,
  MutationCache,
  QueryClient,
} from "@tanstack/react-query";
import i18n from "@/lib/i18n";
import { toast } from "@/components/ui/toast";
import { ApiError } from "@/lib/api-client";
import { formatApiError } from "@/hooks/use-api-error";

// ── retry policy ─────────────────────────────────────────────────────────────

const MAX_QUERY_RETRIES = 2;
const RETRY_BASE_DELAY_MS = 1_000;
const RETRY_MAX_DELAY_MS = 10_000;

/**
 * Decide whether a failed query should be retried.
 *
 *  - 4xx (400-499) responses are never retried (validation, auth, not-found):
 *    retrying the same input would just repeat the same client error.
 *  - 5xx and network/timeout errors are retried up to MAX_QUERY_RETRIES times
 *    with exponential backoff (see retryDelay below).
 */
function shouldRetryQuery(failureCount: number, error: unknown): boolean {
  if (failureCount >= MAX_QUERY_RETRIES) return false;

  if (error instanceof ApiError) {
    // 4xx → no retry; 5xx → retry.
    if (error.status >= 400 && error.status < 500) return false;
    return true;
  }

  // Non-ApiError (network failure, fetch abort, JSON parse) → retry.
  return true;
}

/**
 * Exponential backoff: 1s, 2s, 4s, capped at 10s.
 * attemptIndex is 0-based, so delays are 1s (attempt 0) → 2s → 4s → ...
 */
function exponentialBackoff(attemptIndex: number): number {
  const delay = RETRY_BASE_DELAY_MS * 2 ** attemptIndex;
  return Math.min(delay, RETRY_MAX_DELAY_MS);
}

// ── global error handling ────────────────────────────────────────────────────

/**
 * Redirect to /login on 401, preserving the current path as `next` query param
 * so the auth-store/login flow can return the user where they came from.
 *
 * The Bearer token cleanup itself already runs inside apiFetch; this is the
 * UX-level redirect that we want to fire regardless of whether the 401 came
 * from a query or mutation.
 */
function handle401Redirect(): void {
  const current = window.location.pathname + window.location.search;
  // Avoid bouncing /login → /login → ... when the 401 itself happened on /login.
  if (window.location.pathname.startsWith("/login")) return;
  const next = encodeURIComponent(current);
  window.location.href = `/login?next=${next}`;
}

/**
 * Show a toast for "internal" errors. Validation / not-found / 4xx errors are
 * usually surfaced inline by the calling component, so we only auto-toast for
 * INTERNAL_* and network-level failures here.
 */
function shouldToastError(error: unknown): boolean {
  if (error instanceof ApiError) {
    return error.code.startsWith("INTERNAL");
  }
  // Non-ApiError = network/parse failure: toast it.
  return true;
}

function globalErrorHandler(error: unknown): void {
  if (error instanceof ApiError && error.status === 401) {
    handle401Redirect();
    return;
  }

  if (shouldToastError(error)) {
    // i18next.t has a heavily-overloaded generic signature; the structural
    // shape we use (string key + optional string default) is compatible but
    // TS can't prove it. The runtime contract is verified by use-api-error's
    // tests, so we narrow via `unknown` here.
    const t = i18n.t.bind(i18n) as unknown as Parameters<
      typeof formatApiError
    >[1];
    const message = formatApiError(error, t);
    toast.error(message);
  }
}

// ── exported singleton ───────────────────────────────────────────────────────

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000,
      gcTime: 5 * 60 * 1000,
      refetchOnWindowFocus: true,
      refetchOnReconnect: true,
      retry: shouldRetryQuery,
      retryDelay: exponentialBackoff,
    },
    mutations: {
      retry: 0,
    },
  },
  queryCache: new QueryCache({
    onError: (error) => globalErrorHandler(error),
  }),
  mutationCache: new MutationCache({
    onError: (error) => globalErrorHandler(error),
  }),
});
