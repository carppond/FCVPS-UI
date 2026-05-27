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
      apiFetch<UserPublicProfile>("/api/me", {
        method: "PUT",
        body: JSON.stringify(data),
      }),
  });
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (data: ChangePasswordRequest) =>
      apiFetch<void>("/api/me/password", {
        method: "PUT",
        body: JSON.stringify(data),
      }),
  });
}
