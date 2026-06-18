package middleware

import (
	"net/http"
	"net/url"
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

func isSafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// originMatchesHost reports whether the Origin header's host equals the
// request Host (the value nginx forwards via proxy_set_header Host $host).
func originMatchesHost(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Host == host
}
