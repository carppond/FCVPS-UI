import type { APIResponse } from "@/types/api";
import { prefixedPath } from "@/lib/silent-prefix";

// ── Error types ──────────────────────────────────────────────────────────────

export class NotFoundError extends Error {
  constructor(path: string) {
    super(`Not found: ${path}`);
    this.name = "NotFoundError";
  }
}

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
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
 *  - Maps 401 → clearSession + redirect to /login
 *  - Maps 404 → NotFoundError
 *  - Maps other errors → ApiError with `code` string for i18n rendering
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

  const response = await fetch(url, { ...options, headers });

  if (response.status === 401) {
    store.clearSession();
    window.location.href = "/login";
    throw new ApiError("AUTH_UNAUTHORIZED", "Unauthorized", 401);
  }

  if (response.status === 404) {
    throw new NotFoundError(path);
  }

  let body: APIResponse<T>;
  try {
    body = (await response.json()) as APIResponse<T>;
  } catch {
    throw new ApiError("INTERNAL", `Invalid JSON response from ${path}`, response.status);
  }

  if (!response.ok || body.code) {
    throw new ApiError(
      body.code ?? "INTERNAL",
      body.message ?? "An unexpected error occurred",
      response.status,
    );
  }

  return body.data as T;
}
