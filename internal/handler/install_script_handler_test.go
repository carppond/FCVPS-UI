package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newInstallScriptStack(t *testing.T) http.Handler {
	t.Helper()
	deps := &Deps{
		InstallScriptHandler: NewInstallScriptHandler(InstallScriptHandlerConfig{}),
	}
	mux := NewRouter(deps)
	return mux
}

func TestInstallScriptRendersTokenAndHubURL(t *testing.T) {
	mux := newInstallScriptStack(t)
	req := httptest.NewRequest(http.MethodGet,
		"/install-agent.sh?token=abc123def456&hub_url=https://hub.example.com", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `TOKEN='abc123def456'`) {
		t.Fatalf("rendered script missing token: %s", body)
	}
	if !strings.Contains(body, `HUB_URL='https://hub.example.com'`) {
		t.Fatalf("rendered script missing hub_url: %s", body)
	}
	if !strings.Contains(body, "#!/usr/bin/env bash") {
		t.Fatalf("rendered script missing shebang: %s", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/x-shellscript") {
		t.Fatalf("unexpected content-type: %q", ct)
	}
}

func TestInstallScriptRequiresToken(t *testing.T) {
	mux := newInstallScriptStack(t)
	req := httptest.NewRequest(http.MethodGet, "/install-agent.sh", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without token, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestInstallScriptRejectsBadToken(t *testing.T) {
	mux := newInstallScriptStack(t)
	req := httptest.NewRequest(http.MethodGet,
		"/install-agent.sh?token=bad%20token", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid token, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestInstallScriptFallsBackToRequestHost(t *testing.T) {
	mux := newInstallScriptStack(t)
	req := httptest.NewRequest(http.MethodGet,
		"/install-agent.sh?token=tokenvalue", nil)
	req.Host = "hub.local:8080"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `HUB_URL='http://hub.local:8080'`) {
		t.Fatalf("rendered script missing host-derived hub url: %s", body)
	}
}

// TestInstallScript_HubURLInjection_Rejected covers Bug-1 (review-round1).
// A maliciously crafted hub_url with shell-substitution metacharacters must
// be refused so that `curl … | bash` cannot execute injected commands.
func TestInstallScript_HubURLInjection_Rejected(t *testing.T) {
	mux := newInstallScriptStack(t)
	cases := []string{
		// Each value uses encoded form for shell metacharacters that the
		// URL parser would otherwise truncate (`;`, `&`) so they reach the
		// validator intact.
		"/install-agent.sh?token=tokenvalue&hub_url=https://x$(touch%20/tmp/pwn)",
		"/install-agent.sh?token=tokenvalue&hub_url=https://x%60id%60",
		"/install-agent.sh?token=tokenvalue&hub_url=https://x%3Bid",
		"/install-agent.sh?token=tokenvalue&hub_url=https://x%27evil",
		"/install-agent.sh?token=tokenvalue&hub_url=https://x%7Cid",
		"/install-agent.sh?token=tokenvalue&hub_url=https://x%20evil",
		"/install-agent.sh?token=tokenvalue&hub_url=ftp://example.com",
		"/install-agent.sh?token=tokenvalue&hub_url=http://x.com/%24%28id%29",
	}
	for _, target := range cases {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("target=%s: expected 400, got %d body=%s", target, rec.Code, rec.Body.String())
		}
	}
}

// TestInstallScript_LegitHubURLAccepted ensures the allow-list does not
// reject reasonable production values.
func TestInstallScript_LegitHubURLAccepted(t *testing.T) {
	mux := newInstallScriptStack(t)
	cases := []string{
		"https://hub.example.com",
		"https://hub.example.com:8443",
		"http://10.0.0.1:8080",
		"https://hub.example.com/_app/abcdef0123456789abcdef0123456789",
	}
	for _, hubURL := range cases {
		target := "/install-agent.sh?token=tokenvalue&hub_url=" + hubURL
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("hub_url=%s: expected 200, got %d body=%s", hubURL, rec.Code, rec.Body.String())
		}
	}
}

func TestAgentDownloadMissingPlatformReturns404(t *testing.T) {
	mux := newInstallScriptStack(t)
	req := httptest.NewRequest(http.MethodGet, "/dl/agent-linux-amd64", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	// At v1 development time the embedded FS is empty so we expect 404.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for not-yet-embedded asset, got %d", rec.Code)
	}
}

func TestAgentDownloadRejectsTraversal(t *testing.T) {
	mux := newInstallScriptStack(t)
	req := httptest.NewRequest(http.MethodGet, "/dl/agent..linux", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for traversal, got %d", rec.Code)
	}
}

func TestAgentDownloadRejectsNonAgent(t *testing.T) {
	mux := newInstallScriptStack(t)
	req := httptest.NewRequest(http.MethodGet, "/dl/README.md", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-agent asset, got %d", rec.Code)
	}
}
