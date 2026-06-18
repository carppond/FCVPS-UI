package middleware

import "net/http"

// SecurityHeaders sets defensive HTTP response headers on every hub response.
//
// These are defense-in-depth for the API surface (the SPA's own CSP is set by
// the fronting nginx, which serves index.html). They do NOT stop a malicious
// browser extension — an extension runs with page-level privileges and is
// exempt from page CSP — but they do limit clickjacking, MIME sniffing,
// referrer leakage, and remote-script injection via XSS.
//
//   - X-Content-Type-Options: nosniff — never MIME-sniff a response body.
//   - X-Frame-Options: DENY + frame-ancestors 'none' — no framing (clickjack).
//   - Referrer-Policy: no-referrer — never leak the (token-bearing) URL.
//   - Content-Security-Policy: default-src 'none' — API/JSON responses pull no
//     resources, so the strictest policy is safe and blocks any injected load.
func SecurityHeaders() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			next.ServeHTTP(w, r)
		})
	}
}
