import * as React from "react";
import { prefixedPath } from "@/lib/silent-prefix";

/**
 * Generic SSE handler map. Keys are event names ("agent_status",
 * "agent_metrics", "notification_event", ...) and values are receive callbacks
 * with the already-JSON-parsed payload.
 *
 * Callers should memoise the handlers map (e.g. with `useMemo`) to avoid
 * tearing down and reopening the EventSource on every parent re-render.
 */
export type SSEHandlers = Record<string, (payload: unknown) => void>;

interface EventStreamOptions {
  /** When false, no EventSource is opened. Defaults to true. */
  enabled?: boolean;
}

/**
 * Subscribe to a server-sent-events endpoint with auto-cleanup.
 *
 * The hook injects the current session token via `?token=<jwt>` because
 * EventSource cannot set custom headers — this mirrors what `OtaDialog` does
 * for the OTA progress feed and what the backend SSE handler accepts
 * (Authorization header *or* `?token=` query param per docs §4.2).
 *
 * The EventSource is recreated when the URL, token, or `enabled` flag
 * changes. Handlers are read through a ref so callers can swap callbacks
 * without forcing a reconnect.
 */
export function useEventStream(
  path: string,
  handlers: SSEHandlers,
  options: EventStreamOptions = {},
): void {
  const enabled = options.enabled ?? true;

  // Mirror the latest handlers in a ref so the EventSource listeners can read
  // them without being torn down on every render.
  const handlersRef = React.useRef<SSEHandlers>(handlers);
  React.useEffect(() => {
    handlersRef.current = handlers;
  }, [handlers]);

  React.useEffect(() => {
    if (!enabled) return;
    if (typeof window === "undefined" || typeof EventSource === "undefined") {
      return;
    }
    // No ?token=: the httpOnly sg_session cookie is sent automatically on the
    // same-origin EventSource connection (withCredentials for safety).
    const url = prefixedPath(path);
    let es: EventSource;
    try {
      es = new EventSource(url, { withCredentials: true });
    } catch {
      // Some test environments (jsdom + restricted URLs) can throw — bail out
      // silently so the rest of the page keeps rendering.
      return;
    }

    // Capture the set of event names we registered so cleanup is exact.
    const names = Object.keys(handlersRef.current);
    const listeners: Array<{ name: string; fn: (ev: MessageEvent) => void }> =
      [];
    for (const name of names) {
      const fn = (ev: MessageEvent) => {
        const cb = handlersRef.current[name];
        if (!cb) return;
        try {
          cb(ev.data ? JSON.parse(ev.data) : null);
        } catch {
          // Malformed JSON — swallow; the next event will self-correct.
        }
      };
      es.addEventListener(name, fn as EventListener);
      listeners.push({ name, fn });
    }

    return () => {
      for (const { name, fn } of listeners) {
        es.removeEventListener(name, fn as EventListener);
      }
      es.close();
    };
  }, [path, enabled]);
}
