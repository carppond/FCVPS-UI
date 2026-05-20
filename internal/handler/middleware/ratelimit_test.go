package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/ratelimit"
)

func TestRateLimit_Allows100AndDenies101st(t *testing.T) {
	// 1 req/s steady, 100 burst (matches the project default).
	limiter := ratelimit.New(1, 100, 1024)
	mw := middleware.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}),
		middleware.RequestLog(nil, nil),
		middleware.RateLimit(limiter),
	)

	const ip = "203.0.113.42:5555"
	allowed := 0
	denied := 0
	var firstDenyRetry string
	for i := 0; i < 105; i++ {
		req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		switch rr.Code {
		case http.StatusOK:
			allowed++
		case http.StatusTooManyRequests:
			denied++
			if firstDenyRetry == "" {
				firstDenyRetry = rr.Header().Get("Retry-After")
			}
		default:
			t.Fatalf("unexpected status %d at i=%d", rr.Code, i)
		}
	}
	if allowed != 100 {
		t.Fatalf("expected 100 passes, got %d (denied=%d)", allowed, denied)
	}
	if denied != 5 {
		t.Fatalf("expected 5 denials, got %d", denied)
	}
	if firstDenyRetry == "" {
		t.Fatalf("Retry-After header missing on denial")
	}
	if n, err := strconv.Atoi(firstDenyRetry); err != nil || n < 1 {
		t.Fatalf("Retry-After should be positive integer; got %q", firstDenyRetry)
	}
}

func TestRateLimit_IsolatesIPs(t *testing.T) {
	limiter := ratelimit.New(1, 2, 1024)
	mw := middleware.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}),
		middleware.RequestLog(nil, nil),
		middleware.RateLimit(limiter),
	)

	for i, ip := range []string{"10.0.0.1:1", "10.0.0.2:2", "10.0.0.3:3"} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("ip %d (%s) blocked unexpectedly; status=%d", i, ip, rr.Code)
		}
	}
}

func TestRateLimit_NilLimiter_PassesThrough(t *testing.T) {
	mw := middleware.RateLimit(nil)
	srv := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusTeapot {
		t.Fatalf("nil limiter should be no-op; got %d", rr.Code)
	}
}
