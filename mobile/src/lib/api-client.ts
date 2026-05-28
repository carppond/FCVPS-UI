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

// Redact sensitive fields in log output (passwords, tokens) so dev logs
// don't leak credentials via adb logcat / Xcode console.
function redactBody(body: BodyInit | null | undefined): string {
  if (typeof body !== "string") return "";
  return body
    .replace(/"password"\s*:\s*"[^"]*"/g, '"password":"[redacted]"')
    .replace(/"token"\s*:\s*"[^"]*"/g, '"token":"[redacted]"')
    .replace(/"ssh_password"\s*:\s*"[^"]*"/g, '"ssh_password":"[redacted]"')
    .replace(/"ssh_private_key"\s*:\s*"[^"]*"/g, '"ssh_private_key":"[redacted]"');
}

function redactResponse(text: string): string {
  return text
    .replace(/"access_token"\s*:\s*"[^"]*"/g, '"access_token":"[redacted]"')
    .replace(/"token"\s*:\s*"[^"]*"/g, '"token":"[redacted]"')
    .replace(/"share_token"\s*:\s*"[^"]*"/g, '"share_token":"[redacted]"')
    .replace(/"ssh_password"\s*:\s*"[^"]*"/g, '"ssh_password":"[redacted]"')
    .replace(/"ssh_private_key"\s*:\s*"[^"]*"/g, '"ssh_private_key":"[redacted]"');
}

const debug = (...args: unknown[]) => {
  if (__DEV__) console.log(...args);
};

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

  debug(`[API] ${options.method ?? "GET"} ${url}${options.body ? ` body=${redactBody(options.body)}` : ""}`);

  let response: Response;
  try {
    response = await fetch(url, { ...options, headers });
  } catch (err) {
    const errObj = err as any;
    debug(`[API] ❌ NETWORK ERROR: ${errObj?.message ?? err}`);
    throw new ApiError(
      "NETWORK_ERROR",
      err instanceof Error ? err.message : "Network request failed",
      0,
    );
  }

  const responseText = await response.clone().text();
  debug(`[API] ${response.status} ${path}`);
  debug(`[API] Response: ${redactResponse(responseText).substring(0, 500)}`);

  if (response.status === 401) {
    clearSession();
    throw new ApiError("AUTH_UNAUTHORIZED", "Unauthorized", 401);
  }

  let body: APIResponse<T> | null = null;
  try {
    body = (await response.json()) as APIResponse<T>;
  } catch {
    if (!response.ok) {
      debug(`[API] ❌ HTTP ${response.status} (no JSON body)`);
      throw new ApiError(`HTTP_${response.status}`, `Request failed (${response.status})`, response.status);
    }
    throw new ApiError("PARSE_ERROR", "Invalid JSON response", response.status);
  }

  if (!response.ok || body.code) {
    debug(`[API] ❌ ${body.code ?? response.status}: ${body.message}`);
    throw new ApiError(
      body.code ?? `HTTP_${response.status}`,
      body.message ?? "An unexpected error occurred",
      response.status,
    );
  }

  return body.data as T;
}
