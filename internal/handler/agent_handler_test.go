package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// agentTestStack wires the agent REST handler against a tmp-dir SQLite DB +
// an in-memory hub. Tests can register users, mint sessions, and exercise the
// HTTP surface end-to-end.
type agentTestStack struct {
	t        *testing.T
	mux      http.Handler
	users    *storage.UserRepo
	repo     *storage.AgentRepo
	tokens   *auth.TokenStore
	hub      *agent.Hub
}

func newAgentTestStack(t *testing.T) *agentTestStack {
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
	repo := storage.NewAgentRepo(db, time.Now)
	recordRepo := storage.NewAgentRecordRepo(db)
	hub := agent.NewHub(agent.HubConfig{
		AgentRepo:  repo,
		RecordRepo: recordRepo,
	})
	agentHandler := NewAgentHandler(repo, recordRepo, hub, nil)
	wsHandler := NewAgentWSHandler(hub, repo, recordRepo, nil)
	deps := &Deps{
		DB:             db,
		AuthManager:    mgr,
		TokenStore:     tokens,
		UserRepo:       users,
		SessionRepo:    sessions,
		TOTPManager:    totpMgr,
		AgentHandler:   agentHandler,
		AgentWSHandler: wsHandler,
		AgentHub:       hub,
	}
	mux := NewRouter(deps)
	return &agentTestStack{
		t: t, mux: mux, users: users, repo: repo,
		tokens: tokens, hub: hub,
	}
}

func (s *agentTestStack) createUserAndToken(username, role string) (string, string) {
	s.t.Helper()
	hash, err := auth.HashPassword("Hunter2-AAAA")
	if err != nil {
		s.t.Fatalf("hash: %v", err)
	}
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

func (s *agentTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
	s.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		reader = bytes.NewReader(buf)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec
}

type agentEnvelope[T any] struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	RequestID string `json:"request_id"`
}

func TestAgentCreateReturnsTokenOnce(t *testing.T) {
	s := newAgentTestStack(t)
	_, bearer := s.createUserAndToken("alice", "user")
	rec := s.do(http.MethodPost, "/api/agents",
		map[string]string{"name": "vps-hk-01"}, bearer)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env agentEnvelope[types.AgentCreateResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Token == "" {
		t.Fatalf("expected plaintext token, got empty")
	}
	if env.Data.ID == "" || env.Data.Name != "vps-hk-01" {
		t.Fatalf("unexpected agent body: %+v", env.Data)
	}
	// Subsequent GET must not expose the token.
	getRec := s.do(http.MethodGet, "/api/agents/"+env.Data.ID, nil, bearer)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getRec.Code, getRec.Body.String())
	}
	if bytes.Contains(getRec.Body.Bytes(), []byte(env.Data.Token)) {
		t.Fatalf("token leaked via GET: %s", getRec.Body.String())
	}
}

func TestAgentCrossUserHidden(t *testing.T) {
	s := newAgentTestStack(t)
	_, bearerA := s.createUserAndToken("alice", "user")
	_, bearerB := s.createUserAndToken("bob", "user")
	// Alice creates an agent.
	rec := s.do(http.MethodPost, "/api/agents",
		map[string]string{"name": "alice-vps"}, bearerA)
	var env agentEnvelope[types.AgentCreateResponse]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID
	// Bob attempts to GET → 404.
	getRec := s.do(http.MethodGet, "/api/agents/"+id, nil, bearerB)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 cross-user, got %d", getRec.Code)
	}
	// Bob attempts to delete → 404.
	delRec := s.do(http.MethodDelete, "/api/agents/"+id, nil, bearerB)
	if delRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 cross-user delete, got %d", delRec.Code)
	}
}

func TestAgentRotateTokenInvalidatesOld(t *testing.T) {
	s := newAgentTestStack(t)
	_, bearer := s.createUserAndToken("alice", "user")
	createRec := s.do(http.MethodPost, "/api/agents",
		map[string]string{"name": "x"}, bearer)
	var createEnv agentEnvelope[types.AgentCreateResponse]
	_ = json.Unmarshal(createRec.Body.Bytes(), &createEnv)
	id := createEnv.Data.ID
	oldToken := createEnv.Data.Token

	rotRec := s.do(http.MethodPost, "/api/agents/"+id+"/rotate-token", nil, bearer)
	if rotRec.Code != http.StatusOK {
		t.Fatalf("rotate status = %d body=%s", rotRec.Code, rotRec.Body.String())
	}
	var rotEnv agentEnvelope[types.RotateTokenResponse]
	_ = json.Unmarshal(rotRec.Body.Bytes(), &rotEnv)
	if rotEnv.Data.Token == "" || rotEnv.Data.Token == oldToken {
		t.Fatalf("rotated token must differ + be non-empty (old=%q new=%q)",
			oldToken, rotEnv.Data.Token)
	}
}

func TestAgentCommandOfflineReturns409(t *testing.T) {
	s := newAgentTestStack(t)
	_, bearer := s.createUserAndToken("alice", "user")
	createRec := s.do(http.MethodPost, "/api/agents",
		map[string]string{"name": "x"}, bearer)
	var createEnv agentEnvelope[types.AgentCreateResponse]
	_ = json.Unmarshal(createRec.Body.Bytes(), &createEnv)
	id := createEnv.Data.ID

	body := map[string]any{
		"cmd": "refresh_subscription",
		"args": map[string]string{"subscription_id": "sub1"},
	}
	rec := s.do(http.MethodPost, "/api/agents/"+id+"/command", body, bearer)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 (agent offline), got %d body=%s", rec.Code, rec.Body.String())
	}
	var env agentEnvelope[any]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != string(types.ErrAgentOffline) {
		t.Fatalf("expected code %s, got %s", types.ErrAgentOffline, env.Code)
	}
}

func TestAgentCommandRejectsUnknownCmd(t *testing.T) {
	s := newAgentTestStack(t)
	_, bearer := s.createUserAndToken("alice", "user")
	createRec := s.do(http.MethodPost, "/api/agents",
		map[string]string{"name": "x"}, bearer)
	var createEnv agentEnvelope[types.AgentCreateResponse]
	_ = json.Unmarshal(createRec.Body.Bytes(), &createEnv)
	id := createEnv.Data.ID

	body := map[string]any{"cmd": "bogus_cmd"}
	rec := s.do(http.MethodPost, "/api/agents/"+id+"/command", body, bearer)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown cmd, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentListPaginatesAndIncludesOnlineFlag(t *testing.T) {
	s := newAgentTestStack(t)
	_, bearer := s.createUserAndToken("alice", "user")
	for i := 0; i < 3; i++ {
		s.do(http.MethodPost, "/api/agents",
			map[string]string{"name": "vps-" + string(rune('a'+i))}, bearer)
	}
	rec := s.do(http.MethodGet, "/api/agents?page=1&page_size=10", nil, bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env agentEnvelope[types.PagedResponse[agentListItem]]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Total != 3 || len(env.Data.Items) != 3 {
		t.Fatalf("expected 3 rows, got %+v", env.Data)
	}
	for _, item := range env.Data.Items {
		if item.Online {
			t.Fatalf("no agents are connected, expected online=false: %+v", item)
		}
	}
}

func TestAgentDeleteDisconnectsHubRegistration(t *testing.T) {
	s := newAgentTestStack(t)
	uid, bearer := s.createUserAndToken("alice", "user")
	createRec := s.do(http.MethodPost, "/api/agents",
		map[string]string{"name": "x"}, bearer)
	var createEnv agentEnvelope[types.AgentCreateResponse]
	_ = json.Unmarshal(createRec.Body.Bytes(), &createEnv)
	id := createEnv.Data.ID
	_ = uid

	delRec := s.do(http.MethodDelete, "/api/agents/"+id, nil, bearer)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", delRec.Code, delRec.Body.String())
	}
	// After delete, GET → 404.
	getRec := s.do(http.MethodGet, "/api/agents/"+id, nil, bearer)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getRec.Code)
	}
}

func TestAgentWSReturns404OnMissingToken(t *testing.T) {
	s := newAgentTestStack(t)
	rec := s.do(http.MethodGet, "/api/agent/ws", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 with no token, got %d", rec.Code)
	}
}
