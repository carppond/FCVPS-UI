package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/handler"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func openTestDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	db, err := storage.Open(config.DatabaseConfig{
		DataDir:       dir,
		Filename:      "test.db",
		BusyTimeoutMs: 1000,
		MaxOpenWrite:  1,
		MaxOpenRead:   2,
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("close test db: %v", err)
		}
		_ = os.RemoveAll(dir)
	})
	return db
}

func TestHealthz_OK(t *testing.T) {
	db := openTestDB(t)
	deps := &handler.Deps{
		DB:      db,
		Logger:  newTestLogger(),
		Version: "test-1.2.3",
	}
	_ = handler.NewRouter(deps)
	srv := deps.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get(middleware.TraceIDHeader); got == "" {
		t.Fatalf("missing %s header", middleware.TraceIDHeader)
	}
	var body handler.HealthStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (raw=%s)", err, rr.Body.String())
	}
	if body.Status != "ok" {
		t.Fatalf("status field = %q, want ok", body.Status)
	}
	if body.Version != "test-1.2.3" {
		t.Fatalf("version = %q", body.Version)
	}
}

func TestHealthz_NoDB(t *testing.T) {
	deps := &handler.Deps{Logger: newTestLogger()}
	_ = handler.NewRouter(deps)
	srv := deps.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("nil-db healthz should still 200; got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestRouter_UnknownPath_404(t *testing.T) {
	deps := &handler.Deps{Logger: newTestLogger()}
	_ = handler.NewRouter(deps)
	srv := deps.Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestRouter_GlobalRateLimit_BlocksAfterBurst(t *testing.T) {
	// 1 req/s, burst 3. 5 quick requests ⇒ 3 pass, 2 blocked.
	deps := &handler.Deps{
		Logger:          newTestLogger(),
		GlobalRateLimit: ratelimit.New(1, 3, 100),
	}
	_ = handler.NewRouter(deps)
	srv := deps.Handler()

	pass, blocked := 0, 0
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		switch rr.Code {
		case http.StatusOK:
			pass++
		case http.StatusTooManyRequests:
			blocked++
			if rr.Header().Get("Retry-After") == "" {
				t.Fatalf("Retry-After header missing on 429 response")
			}
		default:
			t.Fatalf("unexpected status %d", rr.Code)
		}
	}
	if pass != 3 || blocked != 2 {
		t.Fatalf("expected 3 pass / 2 blocked, got %d / %d", pass, blocked)
	}
}

func TestRouter_SilentMode_RoutesPrefixed(t *testing.T) {
	const prefix = "00112233445566778899aabbccddeeff"
	deps := &handler.Deps{
		Logger:       newTestLogger(),
		SilentPrefix: prefix,
	}
	_ = handler.NewRouter(deps)
	srv := deps.Handler()

	// Whitelisted /healthz still works WITHOUT the prefix.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("whitelisted /healthz blocked: %d", rr.Code)
	}

	// Unknown path under correct prefix is stripped → falls to mux 404, NOT nginx 404.
	req = httptest.NewRequest(http.MethodGet, "/_app/"+prefix+"/api/missing", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected mux 404 for unknown path, got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "nginx") {
		t.Fatalf("mux 404 should NOT be the nginx clone; body=%s", rr.Body.String())
	}

	// Unknown path WITHOUT the prefix → nginx mimic 404 + Server header.
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from silent mode; got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "nginx/1.18.0") {
		t.Fatalf("expected nginx mimic body; got %q", rr.Body.String())
	}
	if got := rr.Header().Get("Server"); got != "nginx/1.18.0" {
		t.Fatalf("Server header = %q, want nginx/1.18.0", got)
	}
}

func TestRouter_RecoverMiddleware_PanicReturns500(t *testing.T) {
	deps := &handler.Deps{Logger: newTestLogger()}
	// Build the chain manually so we can register a panicking handler.
	rec := middleware.Recover(newTestLogger(), false)
	h := rec(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ERR_INTERNAL_UNKNOWN") {
		t.Fatalf("expected canonical error envelope; got %s", rr.Body.String())
	}
	// Touch deps so the `unused` linter does not complain.
	_ = deps
}

func TestRouter_StartShutdown_NoCrash(t *testing.T) {
	deps := &handler.Deps{Logger: newTestLogger(), DB: openTestDB(t)}
	if mux := handler.NewRouter(deps); mux == nil {
		t.Fatalf("router unexpectedly nil")
	}
	deps.Start(context.Background())
	// Give the watcher goroutine a chance to enter its select.
	time.Sleep(10 * time.Millisecond)
	deps.Shutdown()
}
