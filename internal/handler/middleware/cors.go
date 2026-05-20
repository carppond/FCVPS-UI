package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that emits Access-Control-Allow-* headers for the
// configured allow-list and short-circuits preflight (OPTIONS) requests with a
// 204.
//
// IMPORTANT: per docs/05-tech-lead-plan.md T-3 and architecture §5.3 this
// middleware is NOT mounted by NewRouter in the default build. CORS handling
// belongs to the front fronting proxy (nginx/Caddy) in production. The helper
// exists so single-binary deployments can opt in by composing it themselves.
//
// An empty allowedOrigins list disables CORS entirely (returns the next
// handler unchanged). Use "*" to permit any origin (NOT recommended when
// credentials are involved).
func CORS(allowedOrigins []string) Middleware {
	if len(allowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	origins := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAll = true
		}
		origins[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := origins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods",
					"GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					"Content-Type, Authorization, X-Trace-Id")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
