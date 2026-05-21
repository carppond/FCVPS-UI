package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/handler"
	"shiguang-vps/internal/ota"
	"shiguang-vps/internal/storage"
)

func newOTATestDB(t *testing.T) *storage.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestOTAHandler_StatusReturnsCurrentVersion exercises the happy path of the
// Status endpoint without touching GitHub.
func TestOTAHandler_StatusReturnsCurrentVersion(t *testing.T) {
	t.Parallel()
	db := newOTATestDB(t)
	svc, err := ota.NewService(ota.ServiceConfig{
		DB:             db,
		CurrentVersion: "v1.0.0",
		BinaryPath:     filepath.Join(t.TempDir(), "x"),
		Shutdown:       func() {},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	h := handler.NewOTAHandler(svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/ota/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data struct {
			CurrentVersion string `json:"current_version"`
			HasUpdate      bool   `json:"has_update"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.CurrentVersion != "v1.0.0" {
		t.Fatalf("current_version = %q", resp.Data.CurrentVersion)
	}
	if resp.Data.HasUpdate {
		t.Fatalf("HasUpdate true without a check")
	}
}

// TestOTAHandler_CheckHitsGitHub spins up a mock release server and asserts
// the handler reports HasUpdate=true with the right tag.
func TestOTAHandler_CheckHitsGitHub(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/name/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     "v2.0.0",
			"html_url":     "https://example.com/v2",
			"published_at": "2024-05-01T00:00:00Z",
			"assets":       []any{},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	db := newOTATestDB(t)
	svc, err := ota.NewService(ota.ServiceConfig{
		DB:             db,
		CurrentVersion: "v1.0.0",
		GitHubRepo:     "owner/name",
		APIBase:        srv.URL,
		BinaryPath:     filepath.Join(t.TempDir(), "x"),
		Shutdown:       func() {},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	h := handler.NewOTAHandler(svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/ota/check", nil)
	rr := httptest.NewRecorder()
	h.Check(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data struct {
			HasUpdate     bool   `json:"has_update"`
			LatestVersion string `json:"latest_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Data.HasUpdate {
		t.Fatalf("HasUpdate=false body=%s", rr.Body.String())
	}
	if resp.Data.LatestVersion != "v2.0.0" {
		t.Fatalf("latest_version = %q", resp.Data.LatestVersion)
	}
}

// TestOTAHandler_ApplyWithoutUpdateReturns400 confirms we reject the no-op
// apply rather than starting an unnecessary download.
func TestOTAHandler_ApplyWithoutUpdateReturns400(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/name/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     "v1.0.0",
			"html_url":     "https://example.com",
			"published_at": "2024-05-01T00:00:00Z",
			"assets":       []any{},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	db := newOTATestDB(t)
	svc, err := ota.NewService(ota.ServiceConfig{
		DB:             db,
		CurrentVersion: "v1.0.0",
		GitHubRepo:     "owner/name",
		APIBase:        srv.URL,
		BinaryPath:     filepath.Join(t.TempDir(), "x"),
		Shutdown:       func() {},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	h := handler.NewOTAHandler(svc, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/ota/apply", nil)
	rr := httptest.NewRecorder()
	h.Apply(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
