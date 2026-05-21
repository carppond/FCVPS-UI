package handler

import (
	"bytes"
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

// trafficTestStack wires the traffic REST handler against a tmpdir SQLite DB.
// Helper methods mint users/sessions and seed traffic_records rows so the
// HTTP surface is exercised end-to-end.
type trafficTestStack struct {
	t        *testing.T
	mux      http.Handler
	users    *storage.UserRepo
	agents   *storage.AgentRepo
	traffic  *storage.TrafficRepo
	settings *storage.SettingsRepo
	tokens   *auth.TokenStore
}

func newTrafficTestStack(t *testing.T) *trafficTestStack {
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
	th := NewTrafficHandler(tRepo, agentRepo, sRepo, nil)
	deps := &Deps{
		DB:             db,
		AuthManager:    mgr,
		TokenStore:     tokens,
		UserRepo:       users,
		SessionRepo:    sessions,
		TOTPManager:    totpMgr,
		TrafficHandler: th,
	}
	return &trafficTestStack{
		t: t, mux: NewRouter(deps),
		users: users, agents: agentRepo, traffic: tRepo, settings: sRepo,
		tokens: tokens,
	}
}

func (s *trafficTestStack) makeUser(username, role string) (string, string) {
	s.t.Helper()
	hash, _ := auth.HashPassword("Hunter2-AAAA")
	rec := storage.UserRecord{
		ID: username + "-id", Username: username, PasswordHash: hash,
		Role: role, IsActive: true,
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

func (s *trafficTestStack) seedAgent(userID, agentID string) {
	s.t.Helper()
	if _, err := s.agents.Create(context.Background(), storage.AgentRecord{
		ID: agentID, UserID: userID, Name: agentID,
		TokenHash: "h-" + agentID, Kind: "native",
	}); err != nil {
		s.t.Fatalf("create agent: %v", err)
	}
}

func (s *trafficTestStack) seedTraffic(userID, agentID, date string, used int64) {
	s.t.Helper()
	if err := s.traffic.UpsertDaily(context.Background(), storage.TrafficRecord{
		Date: date, UserID: userID, AgentID: agentID,
		TotalUsed: used, TotalIn: used / 2, TotalOut: used - used/2,
	}); err != nil {
		s.t.Fatalf("seed traffic: %v", err)
	}
}

func TestTrafficHandlerSummaryComputesPercent(t *testing.T) {
	s := newTrafficTestStack(t)
	userID, token := s.makeUser("alice", "user")
	s.seedAgent(userID, "a1")
	// Set monthly limit to 1000 bytes and use 250 → 25%.
	if err := s.settings.Set(context.Background(), "monthly_traffic_limit", "1000"); err != nil {
		t.Fatalf("set limit: %v", err)
	}
	now := time.Now().UTC()
	s.seedTraffic(userID, "a1", now.Format("2006-01-02"), 250)

	req := httptest.NewRequest(http.MethodGet, "/api/traffic/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp types.APIResponse[trafficSummaryDTO]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.TotalUsed != 250 {
		t.Fatalf("expected total_used=250, got %d", resp.Data.TotalUsed)
	}
	if resp.Data.UsagePercent < 24.9 || resp.Data.UsagePercent > 25.1 {
		t.Fatalf("expected percent ~25, got %v", resp.Data.UsagePercent)
	}
	if len(resp.Data.Agents) != 1 || resp.Data.Agents[0].AgentName != "a1" {
		t.Fatalf("unexpected agent breakdown: %+v", resp.Data.Agents)
	}
}

func TestTrafficHandlerHistoryDayBuckets(t *testing.T) {
	s := newTrafficTestStack(t)
	userID, token := s.makeUser("alice", "user")
	s.seedAgent(userID, "a1")
	now := time.Now().UTC()
	for i := 1; i <= 3; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		s.seedTraffic(userID, "a1", date, int64(i*100))
	}
	req := httptest.NewRequest(http.MethodGet, "/api/traffic/history?range=7d&view=day", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp types.APIResponse[[]types.TrafficChartPoint]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 chart points, got %d", len(resp.Data))
	}
}

func TestTrafficHandlerSetThresholdRequiresAdmin(t *testing.T) {
	s := newTrafficTestStack(t)
	_, userToken := s.makeUser("alice", "user")
	body, _ := json.Marshal(map[string]any{"percents": []int{80, 90, 100}})
	req := httptest.NewRequest(http.MethodPut, "/api/traffic/threshold", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+userToken)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin should get 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	_, adminToken := s.makeUser("admin", "admin")
	req2 := httptest.NewRequest(http.MethodPut, "/api/traffic/threshold", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	rec2 := httptest.NewRecorder()
	s.mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("admin should get 200, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	v, _ := s.settings.Get(context.Background(), "traffic_threshold_percents")
	if v != "80,90,100" {
		t.Fatalf("expected stored 80,90,100, got %q", v)
	}
}

func TestTrafficHandlerSetLimitAdminOnly(t *testing.T) {
	s := newTrafficTestStack(t)
	_, adminToken := s.makeUser("admin", "admin")
	body, _ := json.Marshal(map[string]any{"total_limit": 5_000_000_000})
	req := httptest.NewRequest(http.MethodPut, "/api/traffic/limit", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin should get 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	v, _ := s.settings.Get(context.Background(), "monthly_traffic_limit")
	if v != "5000000000" {
		t.Fatalf("expected stored 5000000000, got %q", v)
	}
}
