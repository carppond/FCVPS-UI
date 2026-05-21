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
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// ruleSetTestStack 包装 /api/rule-sets/* 的最小依赖。
type ruleSetTestStack struct {
	t         *testing.T
	mux       http.Handler
	repo      *storage.RuleSetProviderRepo
	users     *storage.UserRepo
	tokens    *auth.TokenStore
	upstream  *httptest.Server
	upstream5 *httptest.Server
}

func newRuleSetTestStack(t *testing.T) *ruleSetTestStack {
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
	repo := storage.NewRuleSetProviderRepo(db, time.Now)
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

	// upstream: 200 OK (sync 成功); upstream5: 503 (sync 失败)。
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)
	upstream5 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(upstream5.Close)

	rsh := NewRuleSetHandler(repo, &http.Client{Timeout: 2 * time.Second}, nil, time.Now)
	deps := &Deps{
		DB:             db,
		AuthManager:    mgr,
		TokenStore:     tokens,
		UserRepo:       users,
		SessionRepo:    sessions,
		TOTPManager:    totpMgr,
		RuleSetHandler: rsh,
		LoginRateLimit: ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &ruleSetTestStack{
		t: t, mux: mux, repo: repo, users: users, tokens: tokens,
		upstream: upstream, upstream5: upstream5,
	}
}

func (s *ruleSetTestStack) createUser(username string) (string, string) {
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

func (s *ruleSetTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

type ruleSetEnvelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestRuleSetHandler_RequiresAuth(t *testing.T) {
	s := newRuleSetTestStack(t)
	rec := s.do(http.MethodGet, "/api/rule-sets", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuleSetHandler_CreateGetUpdateDelete(t *testing.T) {
	s := newRuleSetTestStack(t)
	_, tok := s.createUser("alice")

	createReq := types.CreateRuleSetRequest{
		Name: "openai-mirror", Behavior: types.RuleSetBehaviorDomain,
		Format: types.RuleSetFormatMRS, URL: "https://example.com/openai.mrs",
		IntervalSeconds: 0, // 让 handler 用默认值
		Enabled:         true,
	}
	rec := s.do(http.MethodPost, "/api/rule-sets", createReq, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", rec.Code, rec.Body.String())
	}
	var env ruleSetEnvelope[types.RuleSetProvider]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID
	if id == "" || env.Data.IntervalSeconds != 86400 {
		t.Fatalf("create response off: %+v", env.Data)
	}

	rec = s.do(http.MethodGet, "/api/rule-sets/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d body=%s", rec.Code, rec.Body.String())
	}

	disabled := false
	upd := types.UpdateRuleSetRequest{Name: "openai-renamed", Enabled: &disabled}
	rec = s.do(http.MethodPut, "/api/rule-sets/"+id, upd, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d body=%s", rec.Code, rec.Body.String())
	}
	var env2 ruleSetEnvelope[types.RuleSetProvider]
	_ = json.Unmarshal(rec.Body.Bytes(), &env2)
	if env2.Data.Name != "openai-renamed" || env2.Data.Enabled {
		t.Fatalf("update mismatch: %+v", env2.Data)
	}

	rec = s.do(http.MethodDelete, "/api/rule-sets/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d body=%s", rec.Code, rec.Body.String())
	}
	rec = s.do(http.MethodGet, "/api/rule-sets/"+id, nil, tok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestRuleSetHandler_ValidateBehaviorAndFormat(t *testing.T) {
	s := newRuleSetTestStack(t)
	_, tok := s.createUser("bob")

	bad := map[string]any{
		"name": "x", "behavior": "garbage", "format": "mrs",
		"url": "https://example.com/x.mrs", "enabled": true,
	}
	rec := s.do(http.MethodPost, "/api/rule-sets", bad, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad behavior, got %d", rec.Code)
	}

	bad2 := map[string]any{
		"name": "x", "behavior": "domain", "format": "garbage",
		"url": "https://example.com/x", "enabled": true,
	}
	rec = s.do(http.MethodPost, "/api/rule-sets", bad2, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad format, got %d", rec.Code)
	}
}

func TestRuleSetHandler_Sync(t *testing.T) {
	s := newRuleSetTestStack(t)
	_, tok := s.createUser("carol")

	// 成功路径：upstream 返回 200。
	ok := types.CreateRuleSetRequest{
		Name: "ok-set", Behavior: types.RuleSetBehaviorDomain,
		Format: types.RuleSetFormatMRS, URL: s.upstream.URL, Enabled: true,
	}
	rec := s.do(http.MethodPost, "/api/rule-sets", ok, tok)
	var env ruleSetEnvelope[types.RuleSetProvider]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	okID := env.Data.ID

	rec = s.do(http.MethodPost, "/api/rule-sets/"+okID+"/sync", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync ok: %d body=%s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.LastSyncStatus != "ok" || env.Data.LastSyncedAt == 0 {
		t.Fatalf("sync ok status not persisted: %+v", env.Data)
	}

	// 失败路径：upstream 返回 503。
	fail := types.CreateRuleSetRequest{
		Name: "fail-set", Behavior: types.RuleSetBehaviorDomain,
		Format: types.RuleSetFormatMRS, URL: s.upstream5.URL, Enabled: true,
	}
	rec = s.do(http.MethodPost, "/api/rule-sets", fail, tok)
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	failID := env.Data.ID

	rec = s.do(http.MethodPost, "/api/rule-sets/"+failID+"/sync", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync fail HTTP: %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.LastSyncStatus != "error" || env.Data.LastSyncError == "" {
		t.Fatalf("sync fail status not persisted: %+v", env.Data)
	}
}

func TestRuleSetHandler_Presets(t *testing.T) {
	s := newRuleSetTestStack(t)
	_, tok := s.createUser("dan")
	rec := s.do(http.MethodGet, "/api/rule-sets/presets", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("presets: %d body=%s", rec.Code, rec.Body.String())
	}
	var env ruleSetEnvelope[[]types.RuleSetPreset]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if len(env.Data) < 15 {
		t.Fatalf("want >= 15 rule set presets, got %d", len(env.Data))
	}
	// 类目至少要覆盖 region / app / block。
	cats := map[string]int{}
	for _, p := range env.Data {
		cats[p.Category]++
	}
	for _, c := range []string{"region", "app", "block"} {
		if cats[c] == 0 {
			t.Fatalf("category %q is empty; cats=%v", c, cats)
		}
	}
}

func TestRuleSetHandler_CrossUserIsolation(t *testing.T) {
	s := newRuleSetTestStack(t)
	_, tokA := s.createUser("user-a")
	_, tokB := s.createUser("user-b")

	createReq := types.CreateRuleSetRequest{
		Name: "secret", Behavior: types.RuleSetBehaviorDomain,
		Format: types.RuleSetFormatMRS, URL: "https://example.com/secret.mrs",
		Enabled: true,
	}
	rec := s.do(http.MethodPost, "/api/rule-sets", createReq, tokA)
	var env ruleSetEnvelope[types.RuleSetProvider]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID

	// userB 看不到 A 的 rule set。
	rec = s.do(http.MethodGet, "/api/rule-sets/"+id, nil, tokB)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user GET: expected 404, got %d", rec.Code)
	}
	rec = s.do(http.MethodDelete, "/api/rule-sets/"+id, nil, tokB)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user DELETE: expected 404, got %d", rec.Code)
	}
}
