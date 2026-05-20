import type { APIResponse } from "@/types/api";
import { prefixedPath } from "@/lib/silent-prefix";

// ── Error types ──────────────────────────────────────────────────────────────

export class NotFoundError extends Error {
  constructor(path: string) {
    super(`Not found: ${path}`);
    this.name = "NotFoundError";
  }
}

/**
 * Normalised API error. Every non-2xx response — and every network / parse
 * failure — is wrapped in this type so callers can rely on `code` being a
 * stable i18n key.
 *
 *  - `code` mirrors the backend `code` field when present; otherwise it is
 *    derived from the HTTP status (e.g. `HTTP_500`) or set to a synthetic
 *    `INTERNAL_*` value for transport-level failures.
 *  - `status` is the HTTP status code (0 for network failures).
 *  - `details` / `requestId` come from the backend body when available.
 */
export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
    public readonly details?: unknown,
    public readonly requestId?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// ── Auth store accessor ──────────────────────────────────────────────────────

/** Lazily import auth store to avoid circular deps at module init time. */
async function getAuthStore() {
  const { useAuthStore } = await import("@/stores/auth-store");
  return useAuthStore.getState();
}

// ── Core fetch wrapper ───────────────────────────────────────────────────────

/**
 * Typed fetch wrapper that:
 *  - Prepends silent-mode URL prefix
 *  - Injects Authorization: Bearer <token>
 *  - Maps 401 → clearSession (the /login redirect is handled globally in
 *    query-client so it works for both queries and mutations and can attach
 *    a `next` query param)
 *  - Maps 404 → NotFoundError
 *  - Wraps any other failure in ApiError with a stable `code` field
 */
export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const store = await getAuthStore();
  const url = prefixedPath(path);

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> | undefined),
  };

  if (store.token) {
    headers["Authorization"] = `Bearer ${store.token}`;
  }

  let response: Response;
  try {
    response = await fetch(url, { ...options, headers });
  } catch (err) {
    // Network failure / fetch abort / DNS error — no HTTP status available.
    throw new ApiError(
      "INTERNAL_NETWORK",
      err instanceof Error ? err.message : "Network request failed",
      0,
    );
  }

  if (response.status === 401) {
    // Clear local credentials; the global QueryCache.onError will redirect.
    store.clearSession();
    throw new ApiError("AUTH_UNAUTHORIZED", "Unauthorized", 401);
  }

  if (response.status === 404) {
    throw new NotFoundError(path);
  }

  let body: APIResponse<T> | null = null;
  try {
    body = (await response.json()) as APIResponse<T>;
  } catch {
    // Body wasn't valid JSON. For 2xx that's a contract violation; for non-2xx
    // we still want a usable error code derived from the status.
    if (!response.ok) {
      throw new ApiError(
        `HTTP_${response.status}`,
        `Request failed with status ${response.status}`,
        response.status,
      );
    }
    throw new ApiError(
      "INTERNAL_PARSE",
      `Invalid JSON response from ${path}`,
      response.status,
    );
  }

  if (!response.ok || body.code) {
    throw new ApiError(
      body.code ?? `HTTP_${response.status}`,
      body.message ?? "An unexpected error occurred",
      response.status,
      body.details,
      body.request_id,
    );
  }

  return body.data as T;
}
