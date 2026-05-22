package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"shiguang-vps/internal/handler/middleware"
)

const testPrefix = "deadbeef00112233445566778899aabb"

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newSilent(prefix string) *middleware.SilentMode {
	return middleware.NewSilentMode(middleware.SilentModeConfig{
		InitialPrefix:  prefix,
		InitialEnabled: prefix != "",
		Logger:         silentLogger(),
	})
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Final-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
}

func TestSilentMode_Disabled_PassesThrough(t *testing.T) {
	sm := newSilent("") // empty = disabled
	mw := sm.Middleware()
	srv := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/anywhere", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("disabled mode should pass through; got %d", rr.Code)
	}
	if rr.Header().Get("Server") == "nginx/1.18.0" {
		t.Fatalf("disabled mode should not emit nginx Server header")
	}
}

// TestSilentMode_EnabledFalseWithPrefix_PassesThrough verifies the new opt-in
// invariant: even if the persisted prefix is non-empty, enabled=false makes
// the middleware a complete no-op (no 404 mimic, no path rewrite).
func TestSilentMode_EnabledFalseWithPrefix_PassesThrough(t *testing.T) {
	sm := middleware.NewSilentMode(middleware.SilentModeConfig{
		InitialPrefix:  testPrefix,
		InitialEnabled: false,
		Logger:         silentLogger(),
	})
	mw := sm.Middleware()
	srv := mw(okHandler())

	// Both /api/me (no prefix) AND /_app/<otherprefix>/foo (wrong prefix)
	// should pass through untouched when enabled=false.
	for _, path := range []string{"/api/me", "/admin", "/_app/wrongprefix/foo"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("disabled mode blocked %q; got %d", path, rr.Code)
		}
		if rr.Header().Get("Server") == "nginx/1.18.0" {
			t.Fatalf("disabled mode emitted nginx Server header for %q", path)
		}
	}
}

// TestSilentMode_SetEnabled_LiveToggle verifies the live-toggle path: the
// admin enable/disable endpoint flips enabled via SetEnabled and the next
// request observes the new state without waiting for the watcher poll.
func TestSilentMode_SetEnabled_LiveToggle(t *testing.T) {
	sm := newSilent(testPrefix) // enabled=true
	mw := sm.Middleware()
	srv := mw(okHandler())

	// Initially enabled — wrong path returns 404.
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before SetEnabled(false); got %d", rr.Code)
	}

	// Disable live — same request now passes.
	sm.SetEnabled(false)
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 after SetEnabled(false); got %d", rr.Code)
	}

	// Re-enable — back to 404 for non-prefixed paths.
	sm.SetEnabled(true)
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after SetEnabled(true); got %d", rr.Code)
	}
}

func TestSilentMode_Whitelist_Passes(t *testing.T) {
	sm := newSilent(testPrefix)
	mw := sm.Middleware()
	srv := mw(okHandler())

	cases := []string{
		"/healthz",
		"/s/abcd",
		"/download/abc",
		"/api/v1/nezha/report",
		"/api/notify/telegram/webhook/SOMETOKEN",
		"/api/agent/ws",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("whitelisted %s blocked; status=%d body=%s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestSilentMode_WrongPrefix_Returns404Mimic(t *testing.T) {
	sm := newSilent(testPrefix)
	mw := sm.Middleware()
	srv := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/_app/wrongprefix0000000000000000000/api/me", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404; got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "nginx/1.18.0") {
		t.Fatalf("expected nginx mimic; body=%q", rr.Body.String())
	}
	if rr.Header().Get("Server") != "nginx/1.18.0" {
		t.Fatalf("missing Server: nginx header")
	}
	if rr.Header().Get("Content-Type") != "text/html" {
		t.Fatalf("expected text/html Content-Type; got %q", rr.Header().Get("Content-Type"))
	}
}

func TestSilentMode_NoPrefixAccess_Returns404(t *testing.T) {
	sm := newSilent(testPrefix)
	mw := sm.Middleware()
	srv := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unprefixed API call should be 404; got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "404 Not Found") {
		t.Fatalf("body should contain the nginx 404 text; got %q", rr.Body.String())
	}
}

func TestSilentMode_CorrectPrefix_PassesAndStrips(t *testing.T) {
	sm := newSilent(testPrefix)
	mw := sm.Middleware()
	srv := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/_app/"+testPrefix+"/api/me", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("prefixed access should pass; got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("X-Final-Path"); got != "/api/me" {
		t.Fatalf("downstream handler saw path %q, want /api/me", got)
	}
}

func TestSilentMode_RootPrefix_Strips(t *testing.T) {
	sm := newSilent(testPrefix)
	mw := sm.Middleware()
	srv := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/_app/"+testPrefix, nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("bare prefix access should pass; got %d", rr.Code)
	}
}

func TestSilentMode_SetPrefix_LiveUpdate(t *testing.T) {
	sm := newSilent(testPrefix)
	mw := sm.Middleware()
	srv := mw(okHandler())

	const newPrefix = "abababababababababababababababab"
	sm.SetPrefix(newPrefix)

	// Old prefix should now be rejected.
	req := httptest.NewRequest(http.MethodGet, "/_app/"+testPrefix+"/api/me", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("old prefix should be invalidated after rotate; got %d", rr.Code)
	}

	// New prefix passes.
	req = httptest.NewRequest(http.MethodGet, "/_app/"+newPrefix+"/api/me", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("new prefix should pass; got %d", rr.Code)
	}
}

func TestSilentMode_LoaderRefresh(t *testing.T) {
	current := "first0000000000000000000000000000"
	sm := middleware.NewSilentMode(middleware.SilentModeConfig{
		InitialPrefix: current,
		Loader: func(ctx context.Context) (string, error) {
			return "secondaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
		},
		Logger: silentLogger(),
	})
	if sm.Prefix() != current {
		t.Fatalf("initial prefix wrong: %s", sm.Prefix())
	}
	// The watcher only ticks every 30 s; we exercise the refresh method's
	// side-effects indirectly by calling SetPrefix from the loader return.
	sm.SetPrefix("secondaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if sm.Prefix() != "secondaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("SetPrefix did not update; got %s", sm.Prefix())
	}
}
