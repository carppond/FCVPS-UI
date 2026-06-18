package handler

import (
	"net/http"
	"time"

	"shiguang-vps/internal/auth"
)

// setSessionCookie writes the access token into an httpOnly cookie so the web
// client never has to store it in JavaScript-reachable storage (localStorage),
// where a browser extension or XSS could read it. Native clients ignore this
// and keep using the Authorization: Bearer header from the response body.
//
//   - HttpOnly: not readable by document.cookie / extensions' page context.
//   - Secure: only over HTTPS (skipped on plain-HTTP dev so localhost works).
//   - SameSite=Lax: blocks cross-site POST/fetch from carrying the cookie,
//     which (with the API using POST/PUT/… for all mutations) defeats CSRF.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie (logout).
func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// isHTTPS reports whether the request reached us over TLS, honouring the
// fronting proxy's X-Forwarded-Proto (nginx terminates TLS, talks HTTP to hub).
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}
