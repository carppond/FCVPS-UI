import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  ChangePasswordRequest,
  CreateUserRequest,
  DisableTOTPRequest,
  EnableTOTPRequest,
  EnableTOTPResponse,
  PagedResponse,
  ResetPasswordResponse,
  TOTPSetupResponse,
  UpdateMeRequest,
  UpdateUserRequest,
  User,
} from "@/types/api";

// ─── Me ──────────────────────────────────────────────────────────────────────

/** GET /api/me — current user profile (used for guard hydration + profile page). */
export function useMeQuery(enabled = true) {
  return useQuery({
    queryKey: queryKeys.user.me(),
    queryFn: () => apiFetch<User>("/api/me"),
    enabled,
  });
}

export function useUpdateProfileMutation() {
  return useMutation({
    mutationFn: (payload: UpdateMeRequest) =>
      apiFetch<User>("/api/me", {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: (user) => {
      queryClient.setQueryData(queryKeys.user.me(), user);
    },
  });
}

export function useChangePasswordMutation() {
  return useMutation({
    mutationFn: (payload: ChangePasswordRequest) =>
      apiFetch<null>("/api/me/password", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
  });
}

// ─── 2FA ─────────────────────────────────────────────────────────────────────

/**
 * GET /api/me/totp/setup — fetches a fresh secret + otpauth URI.
 *
 * Disabled by default so callers explicitly opt-in (the setup endpoint has
 * server-side side effects — see contract §M-USER.10).
 */
export function useTotpSetupQuery(enabled: boolean) {
  return useQuery({
    queryKey: ["user", "totp", "setup"],
    queryFn: () => apiFetch<TOTPSetupResponse>("/api/me/totp/setup"),
    enabled,
    staleTime: 0,
    gcTime: 0,
  });
}

export function useConfirmTotpMutation() {
  return useMutation({
    mutationFn: (payload: EnableTOTPRequest) =>
      apiFetch<EnableTOTPResponse>("/api/me/totp/enable", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.user.me() });
    },
  });
}

export function useDisableTotpMutation() {
  return useMutation({
    mutationFn: (payload: DisableTOTPRequest) =>
      apiFetch<null>("/api/me/totp/disable", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.user.me() });
    },
  });
}

export function useRegenRecoveryMutation() {
  return useMutation({
    mutationFn: (password: string) =>
      apiFetch<EnableTOTPResponse>("/api/me/totp/recovery-codes", {
        method: "POST",
        body: JSON.stringify({ password }),
      }),
  });
}

// ─── Admin: user management ──────────────────────────────────────────────────

export interface ListUsersParams {
  page?: number;
  pageSize?: number;
  keyword?: string;
}

function buildUsersQuery(params: ListUsersParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.keyword) search.set("keyword", params.keyword);
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

export function useListUsersQuery(params: ListUsersParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.user.list(), params],
    queryFn: () =>
      apiFetch<PagedResponse<User>>(`/api/admin/users${buildUsersQuery(params)}`),
  });
}

export function useCreateUserMutation() {
  return useMutation({
    mutationFn: (payload: CreateUserRequest) =>
      apiFetch<User>("/api/admin/users", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.user.list() });
    },
  });
}

export function useUpdateUserMutation() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: UpdateUserRequest;
    }) =>
      apiFetch<User>(`/api/admin/users/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.user.list() });
    },
  });
}

export function useDeleteUserMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/admin/users/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.user.list() });
    },
  });
}

export function useResetUserPasswordMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<ResetPasswordResponse>(
        `/api/admin/users/${id}/reset-password`,
        { method: "POST" },
      ),
  });
}

export function useForceDisable2FAMutation() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/admin/users/${id}/disable-2fa`, {
        method: "POST",
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.user.list() });
    },
  });
}
