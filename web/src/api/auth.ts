import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  LoginRequest,
  LoginResponse,
  PendingTOTPResponse,
  VerifyTOTPRequest,
  VerifyRecoveryRequest,
} from "@/types/api";

// ─── Login response variant ──────────────────────────────────────────────────
// Per tech-lead-plan §1.2, the login endpoint always returns 200 with one of
// two payload variants. We discriminate via `totp_required` (present + true on
// the pending variant) so callers can branch with a type-safe `kind` field.

export type LoginResult =
  | { kind: "ok"; payload: LoginResponse }
  | { kind: "totp_required"; payload: PendingTOTPResponse };

type LoginRawResponse = Partial<LoginResponse> & Partial<PendingTOTPResponse>;

function isPendingTotp(
  raw: LoginRawResponse,
): raw is PendingTOTPResponse {
  return raw.totp_required === true && typeof raw.pending_token === "string";
}

function isLoginOk(raw: LoginRawResponse): raw is LoginResponse {
  return typeof raw.access_token === "string" && raw.user !== undefined;
}

async function postLogin(payload: LoginRequest): Promise<LoginResult> {
  const raw = await apiFetch<LoginRawResponse>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify(payload),
  });

  if (isPendingTotp(raw)) {
    return { kind: "totp_required", payload: raw };
  }
  if (isLoginOk(raw)) {
    return { kind: "ok", payload: raw };
  }
  // Defensive: the contract guarantees one of the two shapes; if neither is
  // satisfied we surface a synthetic error rather than silently returning ok.
  throw new Error("Malformed login response");
}

/** POST /api/auth/login — returns a discriminated union (ok | totp_required). */
export function useLoginMutation() {
  return useMutation({ mutationFn: postLogin });
}

/** POST /api/auth/verify-totp — exchanges pending_token + code for an access_token. */
export function useVerifyTotpMutation() {
  return useMutation({
    mutationFn: (payload: VerifyTOTPRequest) =>
      apiFetch<LoginResponse>("/api/auth/verify-totp", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
  });
}

/** POST /api/auth/verify-recovery — exchanges pending_token + recovery code for access_token. */
export function useVerifyRecoveryMutation() {
  return useMutation({
    mutationFn: (payload: VerifyRecoveryRequest) =>
      apiFetch<LoginResponse>("/api/auth/verify-recovery", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
  });
}

/** POST /api/auth/logout — server-side token invalidation. */
export function useLogoutMutation() {
  return useMutation({
    mutationFn: () =>
      apiFetch<null>("/api/auth/logout", { method: "POST" }),
    onSuccess: () => {
      // Drop any cached personal data so a re-login session starts clean.
      queryClient.removeQueries({ queryKey: queryKeys.user.all() });
    },
  });
}
