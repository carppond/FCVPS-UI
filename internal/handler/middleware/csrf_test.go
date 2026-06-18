package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func csrfReq(method, host, origin, proto string, withCookie bool) *http.Request {
	// http:// base so r.TLS is nil; HTTPS is signalled via X-Forwarded-Proto
	// (mirrors nginx terminating TLS and proxying plain HTTP to the hub).
	r := httptest.NewRequest(method, "http://"+host+"/api/x", nil)
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if proto != "" {
		r.Header.Set("X-Forwarded-Proto", proto)
	}
	if withCookie {
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "t"})
	}
	return r
}

func runCSRF(r *http.Request) int {
	rr := httptest.NewRecorder()
	CSRFOriginCheck()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rr, r)
	return rr.Code
}

func TestCSRF_SafeMethodAlwaysPasses(t *testing.T) {
	if c := runCSRF(csrfReq("GET", "h.com", "https://evil.com", "https", true)); c != 200 {
		t.Errorf("GET must pass, got %d", c)
	}
}

func TestCSRF_NoCookieSkips(t *testing.T) {
	// Bearer/mobile (no cookie) — never CSRF-checked even cross-origin.
	if c := runCSRF(csrfReq("POST", "h.com", "https://evil.com", "https", false)); c != 200 {
		t.Errorf("no-cookie POST must pass, got %d", c)
	}
}

func TestCSRF_HTTPSkips(t *testing.T) {
	if c := runCSRF(csrfReq("POST", "h.com", "https://evil.com", "", true)); c != 200 {
		t.Errorf("non-HTTPS must skip CSRF, got %d", c)
	}
}

func TestCSRF_SameHostDifferentPortPasses(t *testing.T) {
	// The regression: Origin carries a port, Host (nginx $host) does not.
	if c := runCSRF(csrfReq("POST", "h.com", "https://h.com:8443", "https", true)); c != 200 {
		t.Errorf("same host, different port must pass, got %d", c)
	}
}

func TestCSRF_CrossOriginBlocked(t *testing.T) {
	if c := runCSRF(csrfReq("POST", "h.com", "https://evil.com", "https", true)); c != 403 {
		t.Errorf("cross-origin POST must be blocked, got %d", c)
	}
}

func TestCSRF_MissingOriginAllowed(t *testing.T) {
	if c := runCSRF(csrfReq("POST", "h.com", "", "https", true)); c != 200 {
		t.Errorf("missing Origin must pass (non-browser), got %d", c)
	}
}
