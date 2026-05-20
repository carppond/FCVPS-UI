package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"shiguang-vps/internal/util"
)

// TraceIDHeader is the HTTP header used to surface the per-request trace ID
// to clients. The value is also stored on the request context so handlers
// can correlate downstream log lines.
const TraceIDHeader = "X-Trace-Id"

// RequestLog returns a middleware that:
//   - generates a UUID v7 trace ID, writes it to the X-Trace-Id response
//     header and stashes it on the request context (CtxKeyTraceID);
//   - resolves the client IP from RemoteAddr / X-Forwarded-For;
//   - wraps the ResponseWriter in a StatusRecorder so the chosen status code
//     is observable after the handler returns;
//   - emits a single info-level slog record after the handler completes.
//
// The middleware deliberately does NOT log request or response bodies — the
// payloads may contain credentials, totp codes, share tokens, etc. (see
// docs/00-coding-standards.md §12).
//
// `now` defaults to time.Now when nil is passed.
func RequestLog(logger *slog.Logger, now func() time.Time) Middleware {
	if now == nil {
		now = time.Now
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := util.UUIDv7()
			remoteIP := resolveClientIP(r)

			ctx := r.Context()
			ctx = context.WithValue(ctx, CtxKeyTraceID, traceID)
			ctx = context.WithValue(ctx, CtxKeyRemoteIP, remoteIP)
			r = r.WithContext(ctx)

			recorder := util.NewStatusRecorder(w)
			recorder.Header().Set(TraceIDHeader, traceID)

			start := now()
			next.ServeHTTP(recorder, r)
			duration := now().Sub(start)

			if logger != nil {
				attrs := []any{
					slog.String("trace_id", traceID),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", recorder.Status),
					slog.Int64("duration_ms", duration.Milliseconds()),
					slog.String("remote_ip", remoteIP),
					slog.Int64("bytes", recorder.BytesSent),
				}
				if uid := UserIDFromContext(ctx); uid != "" {
					attrs = append(attrs, slog.String("user_id", uid))
				}
				logger.LogAttrs(ctx, slog.LevelInfo, "http request", toLogAttrs(attrs)...)
			}
		})
	}
}

// toLogAttrs converts a heterogeneous []any of slog.Attr items into the typed
// slice required by LogAttrs without forcing every caller to declare the type
// inline. Non-Attr values are skipped defensively.
func toLogAttrs(in []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(in))
	for _, v := range in {
		if a, ok := v.(slog.Attr); ok {
			out = append(out, a)
		}
	}
	return out
}

// resolveClientIP returns the most-likely real client IP given the inbound
// request. It honours the first entry of X-Forwarded-For when present,
// otherwise falls back to RemoteAddr (host portion).
//
// The X-Forwarded-For trust decision is left to the deployment doc: any hub
// running directly on the public internet should be put behind a TLS-
// terminating proxy that normalises the header.
func resolveClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// XFF is "client, proxy1, proxy2"; the client is the leftmost entry.
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if r.RemoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
