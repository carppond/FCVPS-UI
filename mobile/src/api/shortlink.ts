import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { ShortLink, CreateShortLinkRequest } from "../types/api";

export function useShortLinksQuery() {
  return useQuery({
    queryKey: ["shortlink", "list"],
    queryFn: () => apiFetch<ShortLink[]>("/api/shortlinks"),
  });
}

export function useCreateShortLink() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateShortLinkRequest) =>
      apiFetch<ShortLink>("/api/shortlinks", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["shortlink"] });
    },
  });
}

export function useDeleteShortLink() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ fileCode, userCode }: { fileCode: string; userCode: string }) =>
      apiFetch<void>(`/api/shortlinks/${fileCode}/${userCode}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["shortlink"] });
    },
  });
}
