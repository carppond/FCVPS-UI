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
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/scriptengine"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

type scriptTestStack struct {
	t      *testing.T
	mux    http.Handler
	repo   *storage.ScriptRepo
	users  *storage.UserRepo
	tokens *auth.TokenStore
}

func newScriptTestStack(t *testing.T) *scriptTestStack {
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
	scripts := storage.NewScriptRepo(db, time.Now)
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
	engine := scriptengine.NewEngine(nil)
	sh := NewScriptHandler(scripts, engine, nil)
	deps := &Deps{
		DB:              db,
		AuthManager:     mgr,
		TokenStore:      tokens,
		UserRepo:        users,
		SessionRepo:     sessions,
		TOTPManager:     totpMgr,
		ScriptHandler:   sh,
		LoginRateLimit:  ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &scriptTestStack{t: t, mux: mux, repo: scripts, users: users, tokens: tokens}
}

func (s *scriptTestStack) createUser(username string) (userID, token string) {
	hash, err := auth.HashPassword("Hunter2-AAAA")
	if err != nil {
		s.t.Fatalf("HashPassword: %v", err)
	}
	user, err := s.users.Create(context.Background(), storage.UserRecord{
		ID: username + "-id", Username: username, PasswordHash: hash,
		Role: string(types.RoleUser), IsActive: true,
	})
	if err != nil {
		s.t.Fatalf("Create user: %v", err)
	}
	tok, _, err := s.tokens.Issue(context.Background(), user.ID, "127.0.0.1", "test", false)
	if err != nil {
		s.t.Fatalf("Issue token: %v", err)
	}
	return user.ID, tok
}

func (s *scriptTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) types.APIResponse[T] {
	t.Helper()
	var out types.APIResponse[T]
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v (raw=%s)", err, rec.Body.String())
	}
	return out
}

// Happy path: create → list → get → patch → delete.
func TestScriptHandler_CRUD(t *testing.T) {
	s := newScriptTestStack(t)
	_, token := s.createUser("alice")

	// Create
	createBody := types.CreateScriptRequest{
		Name:    "my-pre",
		Hook:    types.HookPreSaveNodes,
		Code:    `__output = __input;`,
		Enabled: true,
	}
	rec := s.do(http.MethodPost, "/api/scripts", createBody, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: got %d (body=%s)", rec.Code, rec.Body.String())
	}
	created := decodeJSON[types.Script](t, rec).Data
	if created.ID == "" || created.Hook != types.HookPreSaveNodes {
		t.Fatalf("created: %+v", created)
	}

	// List
	rec = s.do(http.MethodGet, "/api/scripts", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("List: %d", rec.Code)
	}
	listed := decodeJSON[types.PagedResponse[types.Script]](t, rec).Data
	if listed.Total != 1 || len(listed.Items) != 1 {
		t.Fatalf("listed: %+v", listed)
	}

	// Get
	rec = s.do(http.MethodGet, "/api/scripts/"+created.ID, nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("Get: %d", rec.Code)
	}

	// Patch: rename + disable
	enabled := false
	patch := types.UpdateScriptRequest{Name: "renamed", Enabled: &enabled}
	rec = s.do(http.MethodPatch, "/api/scripts/"+created.ID, patch, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("Patch: %d (%s)", rec.Code, rec.Body.String())
	}
	patched := decodeJSON[types.Script](t, rec).Data
	if patched.Name != "renamed" || patched.Enabled {
		t.Fatalf("patched: %+v", patched)
	}

	// Delete
	rec = s.do(http.MethodDelete, "/api/scripts/"+created.ID, nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("Delete: %d", rec.Code)
	}
	// Subsequent GET → 404
	rec = s.do(http.MethodGet, "/api/scripts/"+created.ID, nil, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("Get after delete: want 404, got %d", rec.Code)
	}
}

// Cross-user GET / DELETE all return 404 (information hiding).
func TestScriptHandler_CrossUserIsolation(t *testing.T) {
	s := newScriptTestStack(t)
	_, tokenA := s.createUser("alice")
	_, tokenB := s.createUser("bob")
	rec := s.do(http.MethodPost, "/api/scripts", types.CreateScriptRequest{
		Name: "p", Hook: types.HookPostFetch, Code: `__output = 1;`, Enabled: true,
	}, tokenA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	id := decodeJSON[types.Script](t, rec).Data.ID
	if rec := s.do(http.MethodGet, "/api/scripts/"+id, nil, tokenB); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user GET: want 404, got %d", rec.Code)
	}
	if rec := s.do(http.MethodDelete, "/api/scripts/"+id, nil, tokenB); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user DELETE: want 404, got %d", rec.Code)
	}
}

// Anonymous calls → 401.
func TestScriptHandler_RequiresAuth(t *testing.T) {
	s := newScriptTestStack(t)
	rec := s.do(http.MethodGet, "/api/scripts", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anon list: want 401, got %d", rec.Code)
	}
}

// Test endpoint: happy path returns 200 with output, logs and 0 duration.
func TestScriptHandler_Test_HappyPath(t *testing.T) {
	s := newScriptTestStack(t)
	_, token := s.createUser("alice")
	rec := s.do(http.MethodPost, "/api/scripts", types.CreateScriptRequest{
		Name: "doubler",
		Hook: types.HookPreSaveNodes,
		Code: `console.log('hi'); __output = { y: __input.x * 2 };`,
	}, token)
	id := decodeJSON[types.Script](t, rec).Data.ID
	rec = s.do(http.MethodPost, "/api/scripts/"+id+"/test", map[string]any{
		"input": map[string]any{"x": 7},
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("Test: %d (%s)", rec.Code, rec.Body.String())
	}
	body := decodeJSON[scriptTestResponse](t, rec).Data
	if body.Error != "" {
		t.Fatalf("unexpected error: %q", body.Error)
	}
	var got map[string]float64
	if err := json.Unmarshal([]byte(body.Output), &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if got["y"] != 14 {
		t.Fatalf("y=%v, want 14", got["y"])
	}
	if len(body.Logs) != 1 || body.Logs[0] == "" {
		t.Fatalf("logs: %+v", body.Logs)
	}
}

// Test endpoint: sandbox violation surfaces in `error` (HTTP 200; the
// engine ran the script, it just refused a blocked call).
func TestScriptHandler_Test_SandboxViolation(t *testing.T) {
	s := newScriptTestStack(t)
	_, token := s.createUser("alice")
	rec := s.do(http.MethodPost, "/api/scripts", types.CreateScriptRequest{
		Name: "evil", Hook: types.HookPreSaveNodes, Code: `require('fs');`,
	}, token)
	id := decodeJSON[types.Script](t, rec).Data.ID
	rec = s.do(http.MethodPost, "/api/scripts/"+id+"/test", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("Test: %d", rec.Code)
	}
	body := decodeJSON[scriptTestResponse](t, rec).Data
	if body.Error == "" {
		t.Fatalf("expected sandbox error in body, got empty")
	}
	if !contains(body.Error, "sandbox") {
		t.Fatalf("error missing sandbox marker: %q", body.Error)
	}
}

// Create with invalid hook → 400.
func TestScriptHandler_Create_InvalidHook(t *testing.T) {
	s := newScriptTestStack(t)
	_, token := s.createUser("alice")
	rec := s.do(http.MethodPost, "/api/scripts", types.CreateScriptRequest{
		Name: "x", Hook: types.HookType("bogus"), Code: `__output = 1;`,
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
