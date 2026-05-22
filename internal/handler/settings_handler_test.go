package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"shiguang-vps/internal/ops"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// settingsTestStack extends authTestStack with the SettingsHandler wired up so
// the admin-settings endpoints are reachable through the regular mux.
type settingsTestStack struct {
	*authTestStack
	repo   *storage.SettingsRepo
	silent *ops.SilentMode
}

func newSettingsTestStack(t *testing.T) *settingsTestStack {
	t.Helper()
	base := newAuthTestStack(t)
	repo := storage.NewSettingsRepo(base.dbRef, time.Now)
	silent, err := ops.NewSilentMode(ops.SilentModeConfig{
		Repo:    repo,
		Revoker: base.tokens,
	})
	if err != nil {
		t.Fatalf("NewSilentMode: %v", err)
	}
	settingsHandler := NewSettingsHandler(SettingsHandlerConfig{
		Repo:   repo,
		Silent: silent,
	})

	deps := &Deps{
		DB:              base.dbRef,
		AuthManager:     base.mgr,
		TokenStore:      base.tokens,
		UserRepo:        base.users,
		SessionRepo:     base.sessions,
		TOTPManager:     base.totp,
		SettingsHandler: settingsHandler,
	}
	base.mux = NewRouter(deps)
	return &settingsTestStack{authTestStack: base, repo: repo, silent: silent}
}

// TestSettingsHandler_GetRequiresAdmin verifies a regular user is rejected by
// the RequireAdmin middleware.
func TestSettingsHandler_GetRequiresAdmin(t *testing.T) {
	s := newSettingsTestStack(t)
	tok, _ := s.loginAs(t, "alice", "Hunter2-AAAA", types.RoleUser)
	rec := s.do(http.MethodGet, "/api/admin/settings", nil, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_GetAndPutRoundTrip ensures the admin can read settings,
// update them, and read back the new values. The secret keys must be masked.
func TestSettingsHandler_GetAndPutRoundTrip(t *testing.T) {
	s := newSettingsTestStack(t)
	tok, _ := s.loginAs(t, "root", "Hunter2-AAAA", types.RoleAdmin)

	// Seed one regular setting + one sensitive setting straight into the repo.
	if err := s.repo.Set(s.t.Context(), storage.SettingSessionTTLSeconds, "3600"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := s.repo.Set(s.t.Context(), storage.SettingSMTPPassword, "supersecret"); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	rec := s.do(http.MethodGet, "/api/admin/settings", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", rec.Code, rec.Body.String())
	}
	var env envelope[map[string]string]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data[storage.SettingSessionTTLSeconds] != "3600" {
		t.Fatalf("session_ttl unexpected: %#v", env.Data)
	}
	if env.Data[storage.SettingSMTPPassword] != storage.SettingsMask {
		t.Fatalf("smtp_password not masked: %#v", env.Data)
	}

	// PUT — bump session TTL + keep secret unchanged by sending the mask.
	put := s.do(http.MethodPut, "/api/admin/settings", map[string]string{
		storage.SettingSessionTTLSeconds: "7200",
		storage.SettingSMTPPassword:      storage.SettingsMask,
	}, tok)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", put.Code, put.Body.String())
	}

	val, err := s.repo.Get(s.t.Context(), storage.SettingSessionTTLSeconds)
	if err != nil {
		t.Fatalf("get after PUT: %v", err)
	}
	if val != "7200" {
		t.Fatalf("session_ttl after PUT = %q, want 7200", val)
	}
	secret, err := s.repo.Get(s.t.Context(), storage.SettingSMTPPassword)
	if err != nil {
		t.Fatalf("get secret after PUT: %v", err)
	}
	if secret != "supersecret" {
		t.Fatalf("masked PUT corrupted secret: %q", secret)
	}
}

// TestSettingsHandler_PutRejectsOutOfRange verifies the range validation.
func TestSettingsHandler_PutRejectsOutOfRange(t *testing.T) {
	s := newSettingsTestStack(t)
	tok, _ := s.loginAs(t, "root", "Hunter2-AAAA", types.RoleAdmin)
	rec := s.do(http.MethodPut, "/api/admin/settings", map[string]string{
		storage.SettingAgentHeartbeatInterval: "999999",
	}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_PutRejectsRawPrefixUpdate confirms the silent-mode
// prefix cannot be set through the generic PUT — it must go through Rotate.
func TestSettingsHandler_PutRejectsRawPrefixUpdate(t *testing.T) {
	s := newSettingsTestStack(t)
	tok, _ := s.loginAs(t, "root", "Hunter2-AAAA", types.RoleAdmin)
	// Should be silently dropped (PUT returns 200; nothing persisted).
	const fake = "abcdef0123456789abcdef0123456789"
	rec := s.do(http.MethodPut, "/api/admin/settings", map[string]string{
		storage.SettingSilentModePrefix: fake,
	}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status=%d body=%s", rec.Code, rec.Body.String())
	}
	if v, _ := s.repo.Get(s.t.Context(), storage.SettingSilentModePrefix); v == fake {
		t.Fatalf("raw prefix accepted via PUT: %q", v)
	}
}

// TestSettingsHandler_RotateGeneratesNewPrefixAndPurgesSessions covers the
// most security-sensitive flow. After rotation, the old access token must no
// longer authenticate.
func TestSettingsHandler_RotateGeneratesNewPrefixAndPurgesSessions(t *testing.T) {
	s := newSettingsTestStack(t)
	adminTok, _ := s.loginAs(t, "root", "Hunter2-AAAA", types.RoleAdmin)
	// Sanity: admin can fetch settings.
	pre := s.do(http.MethodGet, "/api/admin/settings", nil, adminTok)
	if pre.Code != http.StatusOK {
		t.Fatalf("pre-rotate GET status=%d", pre.Code)
	}

	// Rotate now requires silent mode to be enabled first.
	if rec := s.do(http.MethodPost, "/api/admin/silent-mode/enable", nil, adminTok); rec.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec := s.do(http.MethodPost, "/api/admin/silent-mode/rotate", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var env envelope[types.SilentModeResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !s.silent.Validate(env.Data.Prefix) {
		t.Fatalf("rotate returned invalid prefix %q", env.Data.Prefix)
	}
	if env.Data.LoginURL == "" {
		t.Fatalf("LoginURL not populated")
	}

	// Post-rotation: the original token is now invalid because RevokeAll
	// wiped the sessions table.
	post := s.do(http.MethodGet, "/api/admin/settings", nil, adminTok)
	if post.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after rotation, got %d body=%s", post.Code, post.Body.String())
	}
}

// TestSettingsHandler_SilentModeStatus verifies the new GET status endpoint
// surfaces the default-disabled state on a fresh DB.
func TestSettingsHandler_SilentModeStatus(t *testing.T) {
	s := newSettingsTestStack(t)
	tok, _ := s.loginAs(t, "root", "Hunter2-AAAA", types.RoleAdmin)

	rec := s.do(http.MethodGet, "/api/admin/silent-mode/status", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("status request status=%d body=%s", rec.Code, rec.Body.String())
	}
	var env envelope[types.SilentModeResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Enabled {
		t.Fatalf("fresh DB should report enabled=false; got true")
	}
	if env.Data.Prefix != "" || env.Data.LoginURL != "" {
		t.Fatalf("status should not leak prefix/url when disabled: %+v", env.Data)
	}
}

// TestSettingsHandler_SilentModeEnableThenDisable exercises the full opt-in
// lifecycle through the HTTP layer: enable mints a prefix + returns it,
// status reports enabled=true with the prefix, disable flips back to off but
// keeps the prefix persisted so a future enable reuses it.
func TestSettingsHandler_SilentModeEnableThenDisable(t *testing.T) {
	s := newSettingsTestStack(t)
	tok, _ := s.loginAs(t, "root", "Hunter2-AAAA", types.RoleAdmin)

	// Enable.
	enableRec := s.do(http.MethodPost, "/api/admin/silent-mode/enable", nil, tok)
	if enableRec.Code != http.StatusOK {
		t.Fatalf("enable status=%d body=%s", enableRec.Code, enableRec.Body.String())
	}
	var enableEnv envelope[types.SilentModeResponse]
	if err := json.Unmarshal(enableRec.Body.Bytes(), &enableEnv); err != nil {
		t.Fatalf("decode enable: %v", err)
	}
	if !enableEnv.Data.Enabled {
		t.Fatalf("enable response should carry enabled=true")
	}
	if !s.silent.Validate(enableEnv.Data.Prefix) {
		t.Fatalf("enable returned invalid prefix %q", enableEnv.Data.Prefix)
	}
	mintedPrefix := enableEnv.Data.Prefix

	// Status reports enabled=true with the prefix surfaced.
	statusRec := s.do(http.MethodGet, "/api/admin/silent-mode/status", nil, tok)
	var statusEnv envelope[types.SilentModeResponse]
	_ = json.Unmarshal(statusRec.Body.Bytes(), &statusEnv)
	if !statusEnv.Data.Enabled || statusEnv.Data.Prefix != mintedPrefix {
		t.Fatalf("status after enable: %+v", statusEnv.Data)
	}

	// Disable.
	disableRec := s.do(http.MethodPost, "/api/admin/silent-mode/disable", nil, tok)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableRec.Code, disableRec.Body.String())
	}

	// Status now reports enabled=false.
	statusRec = s.do(http.MethodGet, "/api/admin/silent-mode/status", nil, tok)
	_ = json.Unmarshal(statusRec.Body.Bytes(), &statusEnv)
	if statusEnv.Data.Enabled {
		t.Fatalf("status should be disabled after Disable")
	}

	// Re-enable returns the same prefix (preservation invariant).
	enable2 := s.do(http.MethodPost, "/api/admin/silent-mode/enable", nil, tok)
	var enable2Env envelope[types.SilentModeResponse]
	_ = json.Unmarshal(enable2.Body.Bytes(), &enable2Env)
	if enable2Env.Data.Prefix != mintedPrefix {
		t.Fatalf("re-enable produced different prefix: %q vs %q",
			enable2Env.Data.Prefix, mintedPrefix)
	}
}

// TestSettingsHandler_SilentModeEnableRequiresAdmin guards the auth posture.
func TestSettingsHandler_SilentModeEnableRequiresAdmin(t *testing.T) {
	s := newSettingsTestStack(t)
	tok, _ := s.loginAs(t, "alice", "Hunter2-AAAA", types.RoleUser)
	rec := s.do(http.MethodPost, "/api/admin/silent-mode/enable", nil, tok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d body=%s", rec.Code, rec.Body.String())
	}
}
