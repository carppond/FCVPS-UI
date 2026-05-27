import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "../lib/api-client";
import type { PagedResponse, NotificationChannel } from "../types/api";

export function useNotificationChannelsQuery() {
  return useQuery({
    queryKey: ["notify", "channels"],
    queryFn: () =>
      apiFetch<PagedResponse<NotificationChannel>>(
        "/api/notify/channels?page=1&page_size=100",
      ),
  });
}
