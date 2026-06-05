import { useQuery, useMutation } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  UserPublicProfile,
  UpdateMeRequest,
  ChangePasswordRequest,
} from "../types/api";

export function useProfileQuery() {
  return useQuery({
    queryKey: ["user", "me"],
    queryFn: () => apiFetch<UserPublicProfile>("/api/me"),
  });
}

export function useUpdateProfile() {
  return useMutation({
    mutationFn: (data: UpdateMeRequest) =>
      // 合同 §:PATCH /api/me(PUT 会 405)
      apiFetch<UserPublicProfile>("/api/me", {
        method: "PATCH",
        body: JSON.stringify(data),
      }),
  });
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (data: ChangePasswordRequest) =>
      // 合同 §:POST /api/me/password(PUT 会 405)
      apiFetch<void>("/api/me/password", {
        method: "POST",
        body: JSON.stringify(data),
      }),
  });
}
