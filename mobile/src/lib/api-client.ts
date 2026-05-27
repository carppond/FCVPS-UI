import { useAuthStore } from "../stores/auth-store";
import type { APIResponse } from "../types/api";

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

export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { token, serverUrl, clearSession } = useAuthStore.getState();
  const url = `${serverUrl}${path}`;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> | undefined),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  console.log(`[API] ${options.method ?? "GET"} ${url}${options.body ? ` body=${options.body}` : ""}`);

  let response: Response;
  try {
    response = await fetch(url, { ...options, headers });
  } catch (err) {
    const errObj = err as any;
    console.log(`[API] ❌ NETWORK ERROR: ${errObj?.message ?? err}`);
    console.log(`[API] ❌ Error type: ${errObj?.constructor?.name}`);
    console.log(`[API] ❌ Error cause: ${JSON.stringify(errObj?.cause ?? "none")}`);
    throw new ApiError(
      "NETWORK_ERROR",
      err instanceof Error ? err.message : "Network request failed",
      0,
    );
  }

  const responseText = await response.clone().text();
  console.log(`[API] ${response.status} ${path}`);
  console.log(`[API] Response: ${responseText.substring(0, 500)}`);

  if (response.status === 401) {
    clearSession();
    throw new ApiError("AUTH_UNAUTHORIZED", "Unauthorized", 401);
  }

  let body: APIResponse<T> | null = null;
  try {
    body = (await response.json()) as APIResponse<T>;
  } catch {
    if (!response.ok) {
      console.log(`[API] ❌ HTTP ${response.status} (no JSON body)`);
      throw new ApiError(`HTTP_${response.status}`, `Request failed (${response.status})`, response.status);
    }
    throw new ApiError("PARSE_ERROR", "Invalid JSON response", response.status);
  }

  if (!response.ok || body.code) {
    console.log(`[API] ❌ ${body.code ?? response.status}: ${body.message}`);
    throw new ApiError(
      body.code ?? `HTTP_${response.status}`,
      body.message ?? "An unexpected error occurred",
      response.status,
    );
  }

  return body.data as T;
}
