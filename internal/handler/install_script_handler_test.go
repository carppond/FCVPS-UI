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
	if !strings.Contains(body, `TOKEN="abc123def456"`) {
		t.Fatalf("rendered script missing token: %s", body)
	}
	if !strings.Contains(body, `HUB_URL="https://hub.example.com"`) {
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
	if !strings.Contains(body, `HUB_URL="http://hub.local:8080"`) {
		t.Fatalf("rendered script missing host-derived hub url: %s", body)
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
