package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// widgetTestStack wires the widget handler against a tmpdir SQLite DB through
// the real router, so both the session-authed token endpoints and the
// widget-token-authed /traffic endpoint are exercised end-to-end.
type widgetTestStack struct {
	t       *testing.T
	mux     http.Handler
	users   *storage.UserRepo
	agents  *storage.AgentRepo
	traffic *storage.TrafficRepo
	tokens  *auth.TokenStore
}

func newWidgetTestStack(t *testing.T) *widgetTestStack {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	users := storage.NewUserRepo(db, time.Now)
	sessions := storage.NewSessionRepo(db, time.Now)
	tokens, err := auth.NewTokenStore(auth.TokenStoreConfig{
		Sessions: sessions, Users: users, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewTokenStore: %v", err)
	}
	totpMgr := auth.NewTOTPManager(auth.NewStorageUserAdapter(users), time.Now)
	mgr, err := auth.NewManager(auth.ManagerConfig{
		Users: users, Sessions: sessions, Tokens: tokens, TOTP: totpMgr,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	agentRepo := storage.NewAgentRepo(db, time.Now)
	tRepo := storage.NewTrafficRepo(db, time.Now)
	sRepo := storage.NewSettingsRepo(db, time.Now)
	wtRepo := storage.NewWidgetTokenRepo(db, time.Now)
	wh := NewWidgetHandler(wtRepo, tRepo, agentRepo, sRepo, nil)
	deps := &Deps{
		DB: db, AuthManager: mgr, TokenStore: tokens,
		UserRepo: users, SessionRepo: sessions, TOTPManager: totpMgr,
		WidgetHandler: wh,
	}
	return &widgetTestStack{
		t: t, mux: NewRouter(deps),
		users: users, agents: agentRepo, traffic: tRepo, tokens: tokens,
	}
}

func (s *widgetTestStack) makeUser(username string) (string, string) {
	s.t.Helper()
	hash, _ := auth.HashPassword("Hunter2-AAAA")
	rec := storage.UserRecord{
		ID: username + "-id", Username: username, PasswordHash: hash,
		Role: "user", IsActive: true,
	}
	if _, err := s.users.Create(context.Background(), rec); err != nil {
		s.t.Fatalf("create user: %v", err)
	}
	tok, _, err := s.tokens.Issue(context.Background(), rec.ID, "127.0.0.1", "test", false)
	if err != nil {
		s.t.Fatalf("issue token: %v", err)
	}
	return rec.ID, tok
}

func (s *widgetTestStack) seedAgentTraffic(userID, agentID string, used int64) {
	s.t.Helper()
	if _, err := s.agents.Create(context.Background(), storage.AgentRecord{
		ID: agentID, UserID: userID, Name: agentID,
		TokenHash: "h-" + agentID, Kind: "native",
	}); err != nil {
		s.t.Fatalf("create agent: %v", err)
	}
	now := time.Now().UTC()
	if err := s.traffic.UpsertDaily(context.Background(), storage.TrafficRecord{
		Date: now.Format("2006-01-02"), UserID: userID, AgentID: agentID,
		TotalUsed: used, TotalIn: used / 2, TotalOut: used - used/2,
	}); err != nil {
		s.t.Fatalf("seed traffic: %v", err)
	}
}

// TestWidgetMintThenFetch exercises the full path: mint a widget token via the
// session-authed endpoint, then fetch /traffic with that token and verify the
// payload (total + top-N sorted/capped).
func TestWidgetMintThenFetch(t *testing.T) {
	s := newWidgetTestStack(t)
	userID, session := s.makeUser("alice")
	// Four agents → top must be capped at 3 and sorted desc by usage.
	s.seedAgentTraffic(userID, "a-small", 100)
	s.seedAgentTraffic(userID, "a-big", 900)
	s.seedAgentTraffic(userID, "a-mid", 500)
	s.seedAgentTraffic(userID, "a-tiny", 10)

	// Mint (session auth).
	mintReq := httptest.NewRequest(http.MethodPost, "/api/widget/token", nil)
	mintReq.Header.Set("Authorization", "Bearer "+session)
	mintRec := httptest.NewRecorder()
	s.mux.ServeHTTP(mintRec, mintReq)
	if mintRec.Code != http.StatusOK {
		t.Fatalf("mint status=%d body=%s", mintRec.Code, mintRec.Body.String())
	}
	var mintResp types.APIResponse[widgetTokenResponse]
	if err := json.Unmarshal(mintRec.Body.Bytes(), &mintResp); err != nil {
		t.Fatalf("decode mint: %v", err)
	}
	widgetToken := mintResp.Data.Token
	if widgetToken == "" {
		t.Fatal("mint returned empty token")
	}

	// Fetch /traffic with the WIDGET token (not the session).
	tReq := httptest.NewRequest(http.MethodGet, "/api/widget/traffic", nil)
	tReq.Header.Set("Authorization", "Bearer "+widgetToken)
	tRec := httptest.NewRecorder()
	s.mux.ServeHTTP(tRec, tReq)
	if tRec.Code != http.StatusOK {
		t.Fatalf("traffic status=%d body=%s", tRec.Code, tRec.Body.String())
	}
	var resp types.APIResponse[widgetTrafficPayload]
	if err := json.Unmarshal(tRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode traffic: %v", err)
	}
	if resp.Data.Used != 1510 {
		t.Fatalf("used=%d, want 1510", resp.Data.Used)
	}
	if len(resp.Data.Top) != 3 {
		t.Fatalf("top len=%d, want 3 (capped)", len(resp.Data.Top))
	}
	if resp.Data.Top[0].Name != "a-big" || resp.Data.Top[1].Name != "a-mid" || resp.Data.Top[2].Name != "a-small" {
		t.Fatalf("top not sorted desc: %+v", resp.Data.Top)
	}
}

func TestWidgetTrafficRejectsBadToken(t *testing.T) {
	s := newWidgetTestStack(t)

	// Missing token.
	req := httptest.NewRequest(http.MethodGet, "/api/widget/traffic", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status=%d, want 401", rec.Code)
	}

	// Garbage token.
	req2 := httptest.NewRequest(http.MethodGet, "/api/widget/traffic?token=nope", nil)
	rec2 := httptest.NewRecorder()
	s.mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: status=%d, want 401", rec2.Code)
	}
}

// TestWidgetRevokeInvalidatesToken confirms disabling the widget makes the
// previously-minted token stop working.
func TestWidgetRevokeInvalidatesToken(t *testing.T) {
	s := newWidgetTestStack(t)
	_, session := s.makeUser("bob")

	mintReq := httptest.NewRequest(http.MethodPost, "/api/widget/token", nil)
	mintReq.Header.Set("Authorization", "Bearer "+session)
	mintRec := httptest.NewRecorder()
	s.mux.ServeHTTP(mintRec, mintReq)
	var mintResp types.APIResponse[widgetTokenResponse]
	_ = json.Unmarshal(mintRec.Body.Bytes(), &mintResp)
	widgetToken := mintResp.Data.Token

	// Revoke (session auth).
	revReq := httptest.NewRequest(http.MethodDelete, "/api/widget/token", nil)
	revReq.Header.Set("Authorization", "Bearer "+session)
	revRec := httptest.NewRecorder()
	s.mux.ServeHTTP(revRec, revReq)
	if revRec.Code != http.StatusOK {
		t.Fatalf("revoke status=%d", revRec.Code)
	}

	// The widget token must no longer work.
	tReq := httptest.NewRequest(http.MethodGet, "/api/widget/traffic", nil)
	tReq.Header.Set("Authorization", "Bearer "+widgetToken)
	tRec := httptest.NewRecorder()
	s.mux.ServeHTTP(tRec, tReq)
	if tRec.Code != http.StatusUnauthorized {
		t.Fatalf("after revoke: status=%d, want 401", tRec.Code)
	}
}
