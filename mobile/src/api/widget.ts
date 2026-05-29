import { apiFetch } from "../lib/api-client";

// Widget (home-screen traffic) API.
//
// The mint endpoint returns a scoped, read-only token that the native widget
// extension uses to fetch /api/widget/traffic WITHOUT carrying the full
// session token. The plaintext is returned exactly once — the app must hand it
// straight to the App Group shared container (see lib/widget-bridge).

export interface WidgetTokenResponse {
  token: string;
}

export interface WidgetTokenStatus {
  enabled: boolean;
}

/** POST /api/widget/token — mint (or rotate) the read-only widget token. */
export function mintWidgetToken(): Promise<WidgetTokenResponse> {
  return apiFetch<WidgetTokenResponse>("/api/widget/token", { method: "POST" });
}

/** DELETE /api/widget/token — revoke the token (disable the widget). */
export function revokeWidgetToken(): Promise<WidgetTokenStatus> {
  return apiFetch<WidgetTokenStatus>("/api/widget/token", { method: "DELETE" });
}

/** GET /api/widget/token — whether the caller currently has a widget token. */
export function getWidgetTokenStatus(): Promise<WidgetTokenStatus> {
  return apiFetch<WidgetTokenStatus>("/api/widget/token");
}
