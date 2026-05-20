// Package middleware hosts HTTP middlewares that wrap the project's handlers.
//
// Each file in this package exports a single middleware constructor. Compose
// them via Chain so the order is explicit at the call site (router.go).
package middleware

import (
	"context"
	"net/http"
)

// Middleware is the canonical signature for every HTTP wrapper in the
// project. We deliberately keep it the same shape as alice.Constructor so we
// can drop the helper later without touching call sites.
type Middleware func(http.Handler) http.Handler

// Chain composes middlewares so that mw[0] runs first (outermost), mw[1]
// next, etc. The returned handler invokes final after every wrapper.
//
//	Chain(recover, log, ratelimit)(handler) ≡
//	    recover(log(ratelimit(handler)))
//
// Passing zero middlewares returns final unchanged.
func Chain(final http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		mw := mws[i]
		if mw == nil {
			continue
		}
		final = mw(final)
	}
	return final
}

// ctxKey is an unexported type used for all context keys defined by this
// package; using a private type avoids accidental collision with other
// packages.
type ctxKey string

const (
	// CtxKeyTraceID stores the per-request trace identifier (UUID v7 hex).
	CtxKeyTraceID ctxKey = "trace_id"
	// CtxKeyUserID stores the authenticated user ID (empty for anonymous).
	CtxKeyUserID ctxKey = "user_id"
	// CtxKeyRemoteIP stores the resolved client IP (X-Forwarded-For aware).
	CtxKeyRemoteIP ctxKey = "remote_ip"
)

// TraceIDFromContext extracts the trace ID set by the RequestLog middleware.
// Returns "" when none was set (e.g. outside the HTTP pipeline).
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(CtxKeyTraceID).(string); ok {
		return v
	}
	return ""
}

// UserIDFromContext returns the authenticated user ID stored in ctx by the
// auth middleware (added in a later task). Empty for anonymous requests.
func UserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(CtxKeyUserID).(string); ok {
		return v
	}
	return ""
}

// RemoteIPFromContext returns the client IP stored by RequestLog.
func RemoteIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(CtxKeyRemoteIP).(string); ok {
		return v
	}
	return ""
}
