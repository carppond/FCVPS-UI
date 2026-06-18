package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// sessionCookieName must match auth.SessionCookieName. Duplicated here to avoid
// an import cycle (the auth package imports this middleware package).
const sessionCookieName = "sg_session"

// CSRFOriginCheck rejects state-changing requests (POST/PUT/PATCH/DELETE) that
// carry the session cookie but come from a foreign Origin.
//
// Cookie auth is implicit (the browser attaches it automatically), so it is
// CSRF-exploitable in a way Bearer-header auth is not. SameSite=Lax already
// blocks most cross-site cookie sends; this is belt-and-suspenders:
//
//   - Only enforced for unsafe methods.
//   - Only when the session cookie is present (Bearer-only / mobile requests,
//     which set no cookie, are never affected).
//   - A missing Origin is allowed (non-browser clients don't send one); only a
//     PRESENT, mismatched Origin is rejected.
func CSRFOriginCheck() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			if _, err := r.Cookie(sessionCookieName); err != nil {
				next.ServeHTTP(w, r) // not cookie-authenticated → not CSRF-prone
				return
			}
			// Only enforce over HTTPS (production behind nginx, which forwards
			// the real Host). On plain-HTTP dev the Vite proxy rewrites Host
			// (changeOrigin), so an Origin/Host compare would false-positive;
			// SameSite=Lax already protects there.
			if !isHTTPSRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin != "" && !originMatchesHost(origin, r.Host) {
				http.Error(w, "cross-origin request blocked", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isHTTPSRequest mirrors the handler-package isHTTPS (kept local to avoid an
// import cycle): TLS terminated here, or nginx signalled it via the header.
func isHTTPSRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// IsSameOriginRequest reports whether a WebSocket/EventSource upgrade may be
// treated as same-origin. Allows: no Origin (native/mobile clients), the
// opaque "null" origin (e.g. a WebView html-string document), or an Origin
// whose hostname matches the request host. Rejects only a present, foreign
// browser Origin — defense-in-depth against cross-site WebSocket hijacking now
// that browser WS connections can authenticate via the httpOnly cookie.
func IsSameOriginRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		return true
	}
	return originMatchesHost(origin, r.Host)
}

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// originMatchesHost reports whether the Origin header's hostname equals the
// request's hostname. Compares HOSTNAME ONLY (ignoring port): a cross-site CSRF
// attacker is on a different hostname, while a legitimate same-host request may
// differ only by port (non-standard HTTPS port, or nginx $host without a port
// vs a browser Origin that carries one) — matching on port would 403 valid
// writes (tcping / sync / …). SameSite=Lax remains the primary CSRF defense.
func originMatchesHost(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return false
	}
	return strings.EqualFold(u.Hostname(), hostnameOnly(host))
}

// hostnameOnly strips an optional :port from a Host header value.
func hostnameOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
