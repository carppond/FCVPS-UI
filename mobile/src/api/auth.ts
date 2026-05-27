import { useMutation } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import { useAuthStore } from "../stores/auth-store";
import type { LoginRequest, LoginResponse, UserPublicProfile } from "../types/api";

export function useLoginMutation() {
  const setAuth = useAuthStore((s) => s.setAuth);
  return useMutation({
    mutationFn: (data: LoginRequest) =>
      apiFetch<LoginResponse>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: (res) => {
      setAuth(res.access_token, res.user);
    },
  });
}

export function useMeQuery() {
  const token = useAuthStore((s) => s.token);
  return {
    queryKey: ["user", "me"],
    queryFn: () => apiFetch<UserPublicProfile>("/api/me"),
    enabled: !!token,
  };
}
