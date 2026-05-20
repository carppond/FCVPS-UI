package middleware

import (
	"math"
	"net/http"
	"strconv"

	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// RateLimit returns a middleware that enforces the global per-IP throughput
// bound. Each request consumes one token from the bucket identified by the
// client IP. Denied requests receive a 429 response with the canonical error
// envelope, a Retry-After header (rounded up to the nearest whole second)
// and the project's trace ID echoed back to the client.
//
// The limiter parameter MAY be nil — in that case the middleware is a no-op
// (useful for tests). For route-level limits (e.g. /api/auth/login 5/hour),
// the calling task constructs a dedicated *ratelimit.Limiter and wraps just
// that handler with RateLimit again.
func RateLimit(limiter *ratelimit.Limiter) Middleware {
	if limiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := RemoteIPFromContext(r.Context())
			if key == "" {
				// Trace middleware did not run (or path bypassed it); fall
				// back to RemoteAddr so we still get a stable key.
				key = r.RemoteAddr
			}
			ok, retry := limiter.Allow(key)
			if ok {
				next.ServeHTTP(w, r)
				return
			}
			seconds := int(math.Ceil(retry.Seconds()))
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			util.RespondError(w, types.ErrAuthRateLimited,
				"rate limit exceeded",
				map[string]any{"retry_after_seconds": seconds},
				TraceIDFromContext(r.Context()))
		})
	}
}
