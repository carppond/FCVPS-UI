package middleware_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"shiguang-vps/internal/handler/middleware"
)

// captureIP runs RequestLog with the given trusted proxies and returns the IP
// the middleware resolved onto the request context.
func captureIP(t *testing.T, remoteAddr string, headers map[string]string, trusted []*net.IPNet) string {
	t.Helper()
	var got string
	h := middleware.RequestLog(nil, nil, trusted)(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			got = middleware.RemoteIPFromContext(r.Context())
		},
	))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return n
}

// TestResolveClientIP_IgnoresSpoofedXFFFromUntrustedPeer is the regression
// guard for the X-Forwarded-For trust fix: a direct attacker (peer not in the
// trusted set) must NOT be able to spoof their IP via forwarded headers.
func TestResolveClientIP_IgnoresSpoofedXFFFromUntrustedPeer(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "127.0.0.1/32")}
	got := captureIP(t, "203.0.113.9:5555", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	}, trusted)
	if got != "203.0.113.9" {
		t.Fatalf("spoofed forwarded header honoured: got %q, want real peer 203.0.113.9", got)
	}
}

// TestResolveClientIP_HonorsForwardedFromTrustedProxy verifies the legit
// nginx→hub path still surfaces the real client behind a trusted proxy.
func TestResolveClientIP_HonorsForwardedFromTrustedProxy(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "127.0.0.1/32")}
	// X-Real-IP preferred.
	if got := captureIP(t, "127.0.0.1:40000", map[string]string{
		"X-Real-IP":       "9.9.9.9",
		"X-Forwarded-For": "1.1.1.1, 127.0.0.1",
	}, trusted); got != "9.9.9.9" {
		t.Fatalf("trusted proxy: want X-Real-IP 9.9.9.9, got %q", got)
	}
	// Falls back to leftmost XFF when no X-Real-IP.
	if got := captureIP(t, "127.0.0.1:40000", map[string]string{
		"X-Forwarded-For": "8.8.8.8, 127.0.0.1",
	}, trusted); got != "8.8.8.8" {
		t.Fatalf("trusted proxy: want XFF leftmost 8.8.8.8, got %q", got)
	}
}

// TestResolveClientIP_NoTrustedProxiesFailsSafe confirms that with no trusted
// proxies configured, forwarded headers are never honoured.
func TestResolveClientIP_NoTrustedProxiesFailsSafe(t *testing.T) {
	got := captureIP(t, "127.0.0.1:40000", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
	}, nil)
	if got != "127.0.0.1" {
		t.Fatalf("fail-safe: want real peer 127.0.0.1, got %q", got)
	}
}
