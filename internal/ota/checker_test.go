package ota_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"shiguang-vps/internal/ota"
)

func TestIsNewer(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		latest, curr   string
		want           bool
	}{
		{"strict newer", "v1.2.4", "v1.2.3", true},
		{"strict newer minor", "v1.3.0", "v1.2.9", true},
		{"strict newer major", "v2.0.0", "v1.9.9", true},
		{"equal", "v1.2.3", "v1.2.3", false},
		{"older", "v1.2.2", "v1.2.3", false},
		{"empty current treated as 0.0.0", "v0.0.1", "", true},
		{"both prefixes", "1.0.1", "v1.0.0", true},
		{"strip pre-release", "v1.2.4-rc1", "v1.2.3", true},
		{"malformed latest", "not-a-version", "v1.2.3", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ota.IsNewer(tc.latest, tc.curr); got != tc.want {
				t.Fatalf("IsNewer(%q,%q) = %v want %v", tc.latest, tc.curr, got, tc.want)
			}
		})
	}
}

func TestNewChecker_RejectsMalformedRepo(t *testing.T) {
	t.Parallel()
	if _, err := ota.NewChecker(ota.CheckerConfig{GitHubRepo: "invalid"}); err == nil {
		t.Fatalf("expected error for repo without slash")
	}
	if _, err := ota.NewChecker(ota.CheckerConfig{GitHubRepo: "/leading"}); err == nil {
		t.Fatalf("expected error for leading slash")
	}
	if _, err := ota.NewChecker(ota.CheckerConfig{GitHubRepo: "trailing/"}); err == nil {
		t.Fatalf("expected error for trailing slash")
	}
	if _, err := ota.NewChecker(ota.CheckerConfig{GitHubRepo: "owner/name"}); err != nil {
		t.Fatalf("unexpected error for valid repo: %v", err)
	}
	// Defaults applied when empty.
	c, err := ota.NewChecker(ota.CheckerConfig{})
	if err != nil || c == nil {
		t.Fatalf("default config rejected: %v", err)
	}
}

func TestChecker_CheckLatest_200WithUpdate(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/name/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		// Accept header should be set to the GitHub-recommended value.
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("unexpected accept header: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     "v1.5.0",
			"name":         "Release 1.5.0",
			"body":         "## What's new\n- foo\n- bar",
			"html_url":     "https://example.com/release/1.5.0",
			"published_at": "2024-05-01T00:00:00Z",
			"assets":       []map[string]any{},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := ota.NewChecker(ota.CheckerConfig{
		GitHubRepo:     "owner/name",
		APIBase:        srv.URL,
		CurrentVersion: "v1.0.0",
	})
	if err != nil {
		t.Fatalf("new checker: %v", err)
	}
	info, err := c.CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("check latest: %v", err)
	}
	if info.TagName != "v1.5.0" {
		t.Errorf("tag_name = %q", info.TagName)
	}
	if !info.HasUpdate {
		t.Errorf("HasUpdate=false; expected true")
	}
	if info.CurrentVersion != "v1.0.0" {
		t.Errorf("CurrentVersion = %q", info.CurrentVersion)
	}
}

func TestChecker_CheckLatest_404IsNoRelease(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/name/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no release", http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, _ := ota.NewChecker(ota.CheckerConfig{GitHubRepo: "owner/name", APIBase: srv.URL})
	_, err := c.CheckLatest(context.Background())
	if !errors.Is(err, ota.ErrNoRelease) {
		t.Fatalf("expected ErrNoRelease, got %v", err)
	}
}

func TestChecker_CheckLatest_500SurfaceStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	c, _ := ota.NewChecker(ota.CheckerConfig{GitHubRepo: "owner/name", APIBase: srv.URL})
	_, err := c.CheckLatest(context.Background())
	if err == nil {
		t.Fatalf("expected error for 502")
	}
	if errors.Is(err, ota.ErrNoRelease) {
		t.Fatalf("5xx must not be ErrNoRelease")
	}
}

func TestReleaseInfo_PickAsset(t *testing.T) {
	t.Parallel()
	info := &ota.ReleaseInfo{
		Assets: []ota.ReleaseAsset{
			{Name: "shiguang-vps-linux-amd64", BrowserDownloadURL: "https://x/bin", Size: 100},
			{Name: "shiguang-vps-linux-amd64.sha256", BrowserDownloadURL: "https://x/sha"},
			{Name: "shiguang-vps-darwin-arm64", BrowserDownloadURL: "https://y/bin"},
		},
	}
	if asset, ok := info.PickAsset("shiguang-vps", "linux", "amd64", ""); !ok || asset.Size != 100 {
		t.Fatalf("PickAsset binary failed: ok=%v asset=%+v", ok, asset)
	}
	if asset, ok := info.PickAsset("shiguang-vps", "linux", "amd64", ".sha256"); !ok || asset.BrowserDownloadURL == "" {
		t.Fatalf("PickAsset sha256 failed: ok=%v asset=%+v", ok, asset)
	}
	if _, ok := info.PickAsset("shiguang-vps", "windows", "amd64", ""); ok {
		t.Fatalf("PickAsset must miss unsupported os")
	}
}
