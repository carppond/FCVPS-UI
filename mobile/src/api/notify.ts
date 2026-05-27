import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type {
  PagedResponse,
  NotificationChannel,
  CreateChannelRequest,
  UpdateChannelRequest,
} from "../types/api";

export function useNotificationChannelsQuery() {
  return useQuery({
    queryKey: ["notify", "channels"],
    queryFn: () =>
      apiFetch<PagedResponse<NotificationChannel>>(
        "/api/notify/channels?page=1&page_size=100",
      ),
  });
}

export function useCreateChannel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateChannelRequest) =>
      apiFetch("/api/notify/channels", {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notify"] });
    },
  });
}

export function useUpdateChannel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateChannelRequest }) =>
      apiFetch(`/api/notify/channels/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notify"] });
    },
  });
}

export function useDeleteChannel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch(`/api/notify/channels/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notify"] });
    },
  });
}
