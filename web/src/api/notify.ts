/**
 * M-NOTIFY API client (T-25).
 *
 * Mirrors the Go handler surface declared in internal/handler/notify_handler.go
 * and the routes section §M-NOTIFY of docs/04-api-contract.md.
 *
 * Endpoints covered:
 *   GET    /api/notify/channels
 *   POST   /api/notify/channels
 *   GET    /api/notify/channels/:id           (derived locally — list cache)
 *   PATCH  /api/notify/channels/:id
 *   DELETE /api/notify/channels/:id
 *   POST   /api/notify/channels/:id/test
 *   GET    /api/notify/channel-kinds          (per-kind config schema)
 *   GET    /api/notify/event-types
 *   GET    /api/notify/events                 (delivery history, paged)
 *
 * The `channel-kinds` payload is treated as the source of truth for what
 * fields each kind needs — the dynamic form below renders strictly from it.
 */
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { queryClient } from "@/lib/query-client";
import { queryKeys } from "@/lib/query-keys";
import type {
  ChannelConfig,
  ChannelKind,
  CreateChannelRequest,
  EventStatus,
  EventType,
  NotificationChannel,
  NotificationEvent,
  PagedResponse,
  UpdateChannelRequest,
} from "@/types/api";

// ─── Supplemental DTOs (richer than the contract one-liner) ─────────────────

/**
 * Field-level schema descriptor for a per-kind config form.
 *
 * The backend's GET /api/notify/channel-kinds returns one entry per kind
 * with a list of these — the frontend renders inputs strictly from this
 * list so adding a new kind never needs a frontend change.
 */
export type ChannelKindFieldType =
  | "string"
  | "password"
  | "number"
  | "boolean"
  | "string[]"
  | "select"
  | "map";

export interface ChannelKindField {
  /** JSON key the field writes into `ChannelConfig`. */
  name: string;
  /** Form input rendering hint. */
  type: ChannelKindFieldType;
  required: boolean;
  /** Optional list of choices for `type === "select"`. */
  options?: Array<{ value: string; label?: string }>;
  /** Default value applied when the form is first rendered. */
  default?: unknown;
  /** i18n key suffix relative to `notify:kinds.<kind>.fields.<name>`. */
  label?: string;
  /** i18n key suffix relative to `notify:kinds.<kind>.fields.<name>.help`. */
  help?: string;
  placeholder?: string;
}

export interface ChannelKindDescriptor {
  kind: ChannelKind;
  display_name: string;
  fields: ChannelKindField[];
}

// ─── Query params ───────────────────────────────────────────────────────────

export interface ListEventsParams {
  page?: number;
  pageSize?: number;
  status?: EventStatus | "";
  channelId?: string;
  eventType?: EventType | "";
  from?: number; // unix ms
  to?: number;
}

function buildEventsQuery(params: ListEventsParams): string {
  const search = new URLSearchParams();
  if (params.page !== undefined) search.set("page", String(params.page));
  if (params.pageSize !== undefined)
    search.set("page_size", String(params.pageSize));
  if (params.status) search.set("status", params.status);
  if (params.channelId) search.set("channel_id", params.channelId);
  if (params.eventType) search.set("event_type", params.eventType);
  if (params.from !== undefined) search.set("from", String(params.from));
  if (params.to !== undefined) search.set("to", String(params.to));
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

// ─── Channel queries ────────────────────────────────────────────────────────

/** GET /api/notify/channels — list of the current user's configured channels.
 *  Backend returns PagedResponse<NotificationChannel>; we flatten to .items
 *  so callers can treat the cached value as a plain array (the page metadata
 *  isn't used by any caller yet — re-add a paged variant when it is). */
export function useChannels() {
  return useQuery({
    queryKey: queryKeys.notify.channels(),
    queryFn: async () => {
      const paged = await apiFetch<{
        items: NotificationChannel[];
        total: number;
        page: number;
        page_size: number;
      }>("/api/notify/channels");
      return paged.items ?? [];
    },
  });
}

/**
 * Single-channel detail. Backend returns the full channel object in the list
 * response so we derive the cache from there when possible; otherwise we fall
 * back to fetching the list.
 */
export function useChannel(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.notify.channel(id ?? ""),
    queryFn: async () => {
      // Try the list cache first to avoid a round-trip — there is no dedicated
      // /api/notify/channels/:id endpoint per the API contract.
      const cached = queryClient.getQueryData<NotificationChannel[]>(
        queryKeys.notify.channels(),
      );
      const hit = cached?.find((c) => c.id === id);
      if (hit) return hit;
      const paged = await apiFetch<{
        items: NotificationChannel[];
        total: number;
        page: number;
        page_size: number;
      }>("/api/notify/channels");
      const all = paged.items ?? [];
      queryClient.setQueryData(queryKeys.notify.channels(), all);
      const found = all.find((c) => c.id === id);
      if (!found) throw new Error(`channel not found: ${id}`);
      return found;
    },
    enabled: Boolean(id),
  });
}

/** GET /api/notify/channel-kinds — per-kind field schema, cached aggressively. */
export function useChannelKinds() {
  return useQuery({
    queryKey: [...queryKeys.notify.all(), "kinds"],
    queryFn: () =>
      apiFetch<ChannelKindDescriptor[]>("/api/notify/channel-kinds"),
    // The schema rarely changes within a session.
    staleTime: 10 * 60 * 1000,
  });
}

/** GET /api/notify/event-types — canonical list of subscribable events. */
export function useEventTypes() {
  return useQuery({
    queryKey: [...queryKeys.notify.all(), "event-types"],
    queryFn: () => apiFetch<EventType[]>("/api/notify/event-types"),
    staleTime: 10 * 60 * 1000,
  });
}

// ─── Channel mutations ──────────────────────────────────────────────────────

/** POST /api/notify/channels — create a new channel. */
export function useCreateChannel() {
  return useMutation({
    mutationFn: (payload: CreateChannelRequest) =>
      apiFetch<NotificationChannel>("/api/notify/channels", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notify.channels() });
    },
  });
}

/** PATCH /api/notify/channels/:id — partial update. */
export function useUpdateChannel() {
  return useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: UpdateChannelRequest;
    }) =>
      apiFetch<NotificationChannel>(`/api/notify/channels/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: (ch) => {
      queryClient.setQueryData(queryKeys.notify.channel(ch.id), ch);
      queryClient.invalidateQueries({ queryKey: queryKeys.notify.channels() });
    },
  });
}

/** DELETE /api/notify/channels/:id. */
export function useDeleteChannel() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<null>(`/api/notify/channels/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.notify.channels() });
    },
  });
}

// ─── Test send ──────────────────────────────────────────────────────────────

export interface ChannelTestResult {
  ok: boolean;
  duration_ms?: number;
  error?: string;
  message?: string;
}

/** POST /api/notify/channels/:id/test — dispatches a synthetic event. */
export function useTestChannel() {
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<ChannelTestResult>(`/api/notify/channels/${id}/test`, {
        method: "POST",
        body: JSON.stringify({}),
      }),
  });
}

// ─── Bulk subscription matrix save ──────────────────────────────────────────

/**
 * Persist the event×channel subscription matrix as a batch of PATCHes.
 *
 * The contract has no dedicated bulk endpoint, so we send one PATCH per
 * channel that changed and invalidate the list cache once at the end. This
 * keeps the API surface stable while still feeling atomic from the UI.
 */
export function useSaveSubscriptionMatrix() {
  return useMutation({
    mutationFn: async (
      updates: Array<{ id: string; event_types: EventType[] }>,
    ) => {
      const results = await Promise.all(
        updates.map((u) =>
          apiFetch<NotificationChannel>(`/api/notify/channels/${u.id}`, {
            method: "PATCH",
            body: JSON.stringify({ event_types: u.event_types }),
          }),
        ),
      );
      return results;
    },
    onSuccess: (channels) => {
      for (const ch of channels) {
        queryClient.setQueryData(queryKeys.notify.channel(ch.id), ch);
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.notify.channels() });
    },
  });
}

// ─── Events / delivery history ──────────────────────────────────────────────

/** GET /api/notify/events — paged delivery history with filtering. */
export function useEvents(params: ListEventsParams = {}) {
  return useQuery({
    queryKey: [...queryKeys.notify.events(), params],
    queryFn: () =>
      apiFetch<PagedResponse<NotificationEvent>>(
        `/api/notify/events${buildEventsQuery(params)}`,
      ),
  });
}

// ─── Telegram bot (T-24) ────────────────────────────────────────────────────

/**
 * GET /api/notify/telegram/status — returns the active webhook token and the
 * chat-IDs registered across the caller's telegram channels.
 *
 * The webhook token is masked for non-admins; admins see the cleartext value
 * because they need it to compose the setWebhook URL.
 */
export interface TGBindingDTO {
  channel_id: string;
  channel_name: string;
  chat_ids: string[];
  enabled: boolean;
}

export interface TGStatusDTO {
  webhook_token: string;
  bindings: TGBindingDTO[];
}

/** GET /api/notify/telegram/status. */
export function useTelegramStatus() {
  return useQuery({
    queryKey: [...queryKeys.notify.all(), "telegram", "status"],
    queryFn: () => apiFetch<TGStatusDTO>("/api/notify/telegram/status"),
  });
}

/** POST /api/notify/telegram/webhook/rotate (admin only). */
export function useRotateTelegramWebhook() {
  return useMutation({
    mutationFn: () =>
      apiFetch<{ webhook_token: string }>(
        "/api/notify/telegram/webhook/rotate",
        { method: "POST", body: JSON.stringify({}) },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [...queryKeys.notify.all(), "telegram", "status"],
      });
    },
  });
}

/** POST /api/notify/telegram/webhook/install (admin only). */
export function useInstallTelegramWebhook() {
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<null>("/api/notify/telegram/webhook/install", {
        method: "POST",
        body: JSON.stringify({ url }),
      }),
  });
}

// ─── Re-exports for callers that need the local types ───────────────────────

export type { ChannelKind, ChannelConfig, EventType, EventStatus };
