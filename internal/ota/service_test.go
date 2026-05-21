package ota_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"shiguang-vps/internal/notify"
	"shiguang-vps/internal/ota"
)

// TestService_Apply_EndToEnd drives the full check → download → verify → swap
// pipeline against an httptest server and asserts the SSE bus fires the
// expected ota_progress events.
func TestService_Apply_EndToEnd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binPath := filepath.Join(dir, "shiguang-vps")
	if err := os.WriteFile(binPath, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}

	// Build the release artefact + SHA-256 sidecar.
	binBody := []byte("NEW_BINARY_FROM_RELEASE_" + runtime.GOOS + "_" + runtime.GOARCH)
	sum := sha256.Sum256(binBody)
	hashHex := hex.EncodeToString(sum[:])
	binName := fmt.Sprintf("shiguang-vps-%s-%s", runtime.GOOS, runtime.GOARCH)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/name/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		// Filled in by the closure below once srv is constructed.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     "v9.9.9",
			"name":         "release",
			"body":         "release notes",
			"html_url":     "https://example.com/release/v9.9.9",
			"published_at": "2024-05-01T00:00:00Z",
			"assets": []map[string]any{
				{"name": binName, "browser_download_url": fmt.Sprintf("%s/bin", releaseBase(r)), "size": len(binBody)},
				{"name": binName + ".sha256", "browser_download_url": fmt.Sprintf("%s/sha", releaseBase(r))},
			},
		})
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binBody)))
		_, _ = w.Write(binBody)
	})
	mux.HandleFunc("/sha", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hashHex + "  " + binName + "\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	bus := notify.NewEventBus()
	events, cancel := bus.Subscribe("admin-uid")
	t.Cleanup(cancel)

	var shutdownOnce sync.Once
	shutdownDone := make(chan struct{})
	db := newTestDB(t)
	svc, err := ota.NewService(ota.ServiceConfig{
		DB:             db,
		BinaryPath:     binPath,
		CurrentVersion: "v1.0.0",
		GitHubRepo:     "owner/name",
		APIBase:        srv.URL,
		EventBus:       bus,
		Shutdown:       func() { shutdownOnce.Do(func() { close(shutdownDone) }) },
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	info, err := svc.CheckNow(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !info.HasUpdate {
		t.Fatalf("has_update=false")
	}

	if err := svc.Apply(context.Background(), info); err != nil {
		t.Fatalf("apply: %v", err)
	}

	got, _ := os.ReadFile(binPath)
	if string(got) != string(binBody) {
		t.Fatalf("binary not swapped: %q", got)
	}

	// Drain SSE events with a deadline so a misbehaving bus doesn't hang.
	deadline := time.After(2 * time.Second)
	stages := map[string]bool{}
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				goto done
			}
			payload, _ := ev.Payload.(map[string]any)
			if stage, ok := payload["stage"].(string); ok {
				stages[stage] = true
			}
			if stages["done"] {
				goto done
			}
		case <-deadline:
			goto done
		}
	}
done:
	for _, s := range []string{"downloading", "verifying", "restarting", "done"} {
		if !stages[s] {
			t.Errorf("expected ota_progress stage %q, stages=%v", s, stages)
		}
	}

	select {
	case <-shutdownDone:
	case <-time.After(3 * time.Second):
		t.Fatalf("shutdown not triggered")
	}

	history := svc.History()
	if len(history) == 0 || history[0].Status != "success" {
		t.Fatalf("history: %+v", history)
	}
}

// TestService_Apply_RejectsSHA256Mismatch ensures a tampered binary is never
// swapped — the most important security invariant of the OTA pipeline.
func TestService_Apply_RejectsSHA256Mismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	binPath := filepath.Join(dir, "shiguang-vps")
	if err := os.WriteFile(binPath, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed binary: %v", err)
	}

	binBody := []byte("MALICIOUS")
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	binName := fmt.Sprintf("shiguang-vps-%s-%s", runtime.GOOS, runtime.GOARCH)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/name/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     "v9.9.9",
			"name":         "release",
			"html_url":     "https://example.com",
			"published_at": "2024-05-01T00:00:00Z",
			"assets": []map[string]any{
				{"name": binName, "browser_download_url": fmt.Sprintf("%s/bin", releaseBase(r)), "size": len(binBody)},
				{"name": binName + ".sha256", "browser_download_url": fmt.Sprintf("%s/sha", releaseBase(r))},
			},
		})
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(binBody)
	})
	mux.HandleFunc("/sha", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(wrongHash))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	db := newTestDB(t)
	svc, err := ota.NewService(ota.ServiceConfig{
		DB:             db,
		BinaryPath:     binPath,
		CurrentVersion: "v1.0.0",
		GitHubRepo:     "owner/name",
		APIBase:        srv.URL,
		Shutdown:       func() {},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	info, err := svc.CheckNow(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if err := svc.Apply(context.Background(), info); err == nil {
		t.Fatalf("expected sha256 mismatch error")
	}
	got, _ := os.ReadFile(binPath)
	if string(got) != "OLD" {
		t.Fatalf("binary swapped despite mismatch: %q", got)
	}
	// Failed history entry recorded.
	hist := svc.History()
	if len(hist) == 0 || hist[0].Status != "failed" {
		t.Fatalf("history: %+v", hist)
	}
}

// TestService_CurrentVersionReturnsConfigured ensures the handler can render
// the running binary's tag even when the daily checker has not yet run.
func TestService_CurrentVersionReturnsConfigured(t *testing.T) {
	t.Parallel()
	svc, err := ota.NewService(ota.ServiceConfig{
		DB:             newTestDB(t),
		CurrentVersion: "v1.2.3",
		BinaryPath:     filepath.Join(t.TempDir(), "x"),
		Shutdown:       func() {},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if got := svc.CurrentVersion(); got != "v1.2.3" {
		t.Fatalf("CurrentVersion = %q", got)
	}
}

// releaseBase derives the public scheme://host of the running httptest server
// from the incoming request — used so the asset URLs in the JSON payload
// point back to the same test server without us having to thread a reference.
func releaseBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	return scheme + "://" + host
}
