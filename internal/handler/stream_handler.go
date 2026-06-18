package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// SSEHeartbeatInterval bounds how long the handler waits before emitting a
// comment frame to keep idle proxies (nginx, cloudflare) from disconnecting.
const SSEHeartbeatInterval = 15 * time.Second

// StreamHandler implements GET /api/notify/stream. Authentication is via
// the standard Authorization header OR a `?token=<bearer>` query parameter
// (browser EventSource cannot set headers). The handler subscribes to the
// per-user EventBus and forwards each SSEEvent as a text/event-stream frame.
type StreamHandler struct {
	tokens *auth.TokenStore
	bus    *notify.EventBus
	logger *slog.Logger
}

// NewStreamHandler wires the handler. tokens must be non-nil; bus may be
// nil (the handler then immediately returns 503).
func NewStreamHandler(tokens *auth.TokenStore, bus *notify.EventBus, logger *slog.Logger) *StreamHandler {
	return &StreamHandler{tokens: tokens, bus: bus, logger: logger}
}

// Stream implements GET /api/notify/stream. The response stays open until
// (a) the client closes the connection, (b) the server context is cancelled
// or (c) the configured TTL elapses (the client should auto-reconnect via
// EventSource).
func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	if h.bus == nil {
		util.RespondError(w, types.ErrInternalUnknown, "event bus unavailable", nil, traceID)
		return
	}
	user, ok := h.authenticate(r)
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "authentication required", nil, traceID)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		util.RespondError(w, types.ErrInternalUnknown, "streaming unsupported", nil, traceID)
		return
	}
	// Bug-7 (review-round1): SSE is a long-lived response so the global
	// http.Server.WriteTimeout (60s in cmd/server/main.go) would otherwise
	// kill the connection. Clear the per-response write deadline so
	// heartbeats can extend indefinitely; the bus + client ctx still drive
	// graceful teardown.
	if rc := http.NewResponseController(w); rc != nil {
		// Tolerate the "not supported" error on test ResponseWriters.
		_ = rc.SetWriteDeadline(time.Time{})
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)

	events, cancel := h.bus.Subscribe(user.ID)
	defer cancel()

	// Initial frame so the client knows the connection is live.
	if err := writeSSEEvent(w, "system", map[string]any{
		"kind":     "hello",
		"trace_id": traceID,
		"now":      time.Now().UnixMilli(),
	}); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(SSEHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, ev.Kind, ev.Payload); err != nil {
				if h.logger != nil {
					h.logger.Debug(
						"sse write failed",
						slog.String("err", err.Error()),
						slog.String("user_id", user.ID),
					)
				}
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// authenticate accepts either the standard Bearer header or a ?token=
// query parameter (used by browser EventSource clients).
func (h *StreamHandler) authenticate(r *http.Request) (*storage.UserRecord, bool) {
	if h.tokens == nil {
		return nil, false
	}
	token := ""
	if c, err := r.Cookie(auth.SessionCookieName); err == nil && c.Value != "" {
		token = c.Value // web: httpOnly cookie auto-sent on same-origin EventSource
	} else if v := r.Header.Get("Authorization"); len(v) > 7 && v[:7] == "Bearer " {
		token = v[7:]
	} else if q := r.URL.Query().Get("token"); q != "" {
		token = q
	}
	if token == "" {
		return nil, false
	}
	result, err := h.tokens.Lookup(r.Context(), token)
	if err != nil {
		if errors.Is(err, auth.ErrSessionNotFound) || errors.Is(err, auth.ErrAccountDisabled) {
			return nil, false
		}
		if h.logger != nil {
			h.logger.Warn("sse auth lookup",
				slog.String("err", err.Error()))
		}
		return nil, false
	}
	if result == nil || result.User == nil {
		return nil, false
	}
	return result.User, true
}

// writeSSEEvent emits a `event:` + `data:` frame. Payload is JSON-encoded;
// each newline in the encoded form starts a new `data:` line so SSE parsers
// reassemble it as a single event.
func writeSSEEvent(w http.ResponseWriter, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sse payload: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", kind); err != nil {
		return err
	}
	// SSE: each line of the data field must be prefixed with "data: ".
	// We emit a single line because json.Marshal never produces newlines.
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}
