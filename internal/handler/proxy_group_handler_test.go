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

// proxyGroupTestStack 包装 /api/proxy-groups/* 的最小依赖。
type proxyGroupTestStack struct {
	t      *testing.T
	mux    http.Handler
	repo   *storage.ProxyGroupRepo
	users  *storage.UserRepo
	tokens *auth.TokenStore
}

func newProxyGroupTestStack(t *testing.T) *proxyGroupTestStack {
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
	repo := storage.NewProxyGroupRepo(db, time.Now)
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
	pgh := NewProxyGroupHandler(repo, nil)
	deps := &Deps{
		DB:                db,
		AuthManager:       mgr,
		TokenStore:        tokens,
		UserRepo:          users,
		SessionRepo:       sessions,
		TOTPManager:       totpMgr,
		ProxyGroupHandler: pgh,
		LoginRateLimit:    ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &proxyGroupTestStack{t: t, mux: mux, repo: repo, users: users, tokens: tokens}
}

func (s *proxyGroupTestStack) createUser(username string) (string, string) {
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

func (s *proxyGroupTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

type proxyGroupEnvelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestProxyGroupHandler_RequiresAuth(t *testing.T) {
	s := newProxyGroupTestStack(t)
	rec := s.do(http.MethodGet, "/api/proxy-groups", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxyGroupHandler_CreateGetUpdateDelete(t *testing.T) {
	s := newProxyGroupTestStack(t)
	_, tok := s.createUser("alice")

	createReq := types.CreateProxyGroupRequest{
		Name: "hk-fast", Type: types.ProxyGroupURLTest,
		Icon: "🇭🇰", IncludeAll: true,
		Filter:       "(?i)HK",
		TestURL:      "http://www.gstatic.com/generate_204",
		TestInterval: 300,
		MemberGroups: []string{"node-select"},
	}
	rec := s.do(http.MethodPost, "/api/proxy-groups", createReq, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", rec.Code, rec.Body.String())
	}
	var env proxyGroupEnvelope[types.ProxyGroupCategory]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID
	if id == "" {
		t.Fatalf("missing id: %s", rec.Body.String())
	}
	if !env.Data.IncludeAll || env.Data.Filter != "(?i)HK" {
		t.Fatalf("create payload not persisted: %+v", env.Data)
	}
	if len(env.Data.MemberGroups) != 1 || env.Data.MemberGroups[0] != "node-select" {
		t.Fatalf("members not round-tripped: %+v", env.Data)
	}

	rec = s.do(http.MethodGet, "/api/proxy-groups/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d body=%s", rec.Code, rec.Body.String())
	}

	// Update: clear filter + change MemberProxies (non-nil pointer to empty slice).
	noFilter := ""
	includeAll := false
	newMembers := []string{"DIRECT", "REJECT"}
	upd := types.UpdateProxyGroupRequest{
		Name:          "hk-renamed",
		Filter:        &noFilter,
		IncludeAll:    &includeAll,
		MemberProxies: &newMembers,
	}
	rec = s.do(http.MethodPut, "/api/proxy-groups/"+id, upd, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d body=%s", rec.Code, rec.Body.String())
	}
	var env2 proxyGroupEnvelope[types.ProxyGroupCategory]
	_ = json.Unmarshal(rec.Body.Bytes(), &env2)
	if env2.Data.Name != "hk-renamed" || env2.Data.IncludeAll || env2.Data.Filter != "" {
		t.Fatalf("update mismatch: %+v", env2.Data)
	}
	if len(env2.Data.MemberProxies) != 2 {
		t.Fatalf("members not updated: %+v", env2.Data)
	}

	rec = s.do(http.MethodDelete, "/api/proxy-groups/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d body=%s", rec.Code, rec.Body.String())
	}
	rec = s.do(http.MethodGet, "/api/proxy-groups/"+id, nil, tok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestProxyGroupHandler_ValidateType(t *testing.T) {
	s := newProxyGroupTestStack(t)
	_, tok := s.createUser("bob")
	bad := map[string]any{
		"name": "x", "type": "garbage",
	}
	rec := s.do(http.MethodPost, "/api/proxy-groups", bad, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxyGroupHandler_Reorder(t *testing.T) {
	s := newProxyGroupTestStack(t)
	_, tok := s.createUser("carol")
	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		req := types.CreateProxyGroupRequest{
			Name: "g" + string(rune('a'+i)), Type: types.ProxyGroupSelect,
			SortOrder: int32((i + 1) * 100),
		}
		rec := s.do(http.MethodPost, "/api/proxy-groups", req, tok)
		var env proxyGroupEnvelope[types.ProxyGroupCategory]
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		ids = append(ids, env.Data.ID)
	}
	// 反转顺序：[2, 1, 0]
	body := types.ProxyGroupReorderRequest{
		IDs: []string{ids[2], ids[1], ids[0]},
	}
	rec := s.do(http.MethodPost, "/api/proxy-groups/reorder", body, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder: %d body=%s", rec.Code, rec.Body.String())
	}
	rec = s.do(http.MethodGet, "/api/proxy-groups", nil, tok)
	var env proxyGroupEnvelope[types.PagedResponse[types.ProxyGroupCategory]]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.Items[0].ID != ids[2] || env.Data.Items[2].ID != ids[0] {
		t.Fatalf("reorder didn't take effect: %v",
			[]string{env.Data.Items[0].ID, env.Data.Items[1].ID, env.Data.Items[2].ID})
	}
}

func TestProxyGroupHandler_Presets(t *testing.T) {
	s := newProxyGroupTestStack(t)
	_, tok := s.createUser("eve")
	rec := s.do(http.MethodGet, "/api/proxy-groups/presets", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("presets: %d body=%s", rec.Code, rec.Body.String())
	}
	var env proxyGroupEnvelope[[]types.ProxyGroupPreset]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if len(env.Data) < 18 {
		t.Fatalf("want >= 18 proxy group presets, got %d", len(env.Data))
	}
	ids := map[string]bool{}
	for _, p := range env.Data {
		ids[p.ID] = true
	}
	for _, want := range []string{
		"node-select", "auto-fastest",
		"region-hk", "region-jp", "region-us", "region-sg", "region-tw", "region-kr",
		"app-ai", "app-streaming", "app-google", "app-microsoft", "app-apple",
		"app-telegram", "app-gaming",
		"global-direct", "global-block", "fish",
	} {
		if !ids[want] {
			t.Fatalf("missing preset %q; ids=%v", want, ids)
		}
	}
}

func TestProxyGroupHandler_CrossUserIsolation(t *testing.T) {
	s := newProxyGroupTestStack(t)
	_, tokA := s.createUser("user-a")
	_, tokB := s.createUser("user-b")
	createReq := types.CreateProxyGroupRequest{
		Name: "secret-grp", Type: types.ProxyGroupSelect,
	}
	rec := s.do(http.MethodPost, "/api/proxy-groups", createReq, tokA)
	var env proxyGroupEnvelope[types.ProxyGroupCategory]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID

	rec = s.do(http.MethodGet, "/api/proxy-groups/"+id, nil, tokB)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user GET: expected 404, got %d", rec.Code)
	}
}
