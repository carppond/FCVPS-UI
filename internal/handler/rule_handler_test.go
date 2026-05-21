package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// ruleTestStack wires the bare minimum for /api/rules/* HTTP tests.
type ruleTestStack struct {
	t      *testing.T
	mux    http.Handler
	rules  *storage.CustomRuleRepo
	users  *storage.UserRepo
	tokens *auth.TokenStore
	dbRef  *storage.DB
}

func newRuleTestStack(t *testing.T) *ruleTestStack {
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
	subs := storage.NewSubscriptionRepo(db, time.Now)
	rules := storage.NewCustomRuleRepo(db, time.Now)
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
	rh := NewRuleHandler(rules, subs, nil, nil)
	deps := &Deps{
		DB:             db,
		AuthManager:    mgr,
		TokenStore:     tokens,
		UserRepo:       users,
		SessionRepo:    sessions,
		TOTPManager:    totpMgr,
		RuleHandler:    rh,
		LoginRateLimit: ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &ruleTestStack{t: t, mux: mux, rules: rules, users: users, tokens: tokens, dbRef: db}
}

func (s *ruleTestStack) createUser(username string) (string, string) {
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

func (s *ruleTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

type ruleEnvelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestRuleHandler_RequiresAuth(t *testing.T) {
	s := newRuleTestStack(t)
	rec := s.do(http.MethodGet, "/api/rules", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuleHandler_CreateGetUpdateDelete(t *testing.T) {
	s := newRuleTestStack(t)
	_, tok := s.createUser("alice")

	createReq := types.CreateRuleRequest{
		Name: "block-ads", Type: types.RuleTypeRules, Mode: types.RuleModePrepend,
		Content: "DOMAIN-KEYWORD,ads,REJECT\n", Enabled: true,
	}
	rec := s.do(http.MethodPost, "/api/rules", createReq, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", rec.Code, rec.Body.String())
	}
	var env ruleEnvelope[types.CustomRule]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	id := env.Data.ID
	if id == "" {
		t.Fatalf("missing id: %s", rec.Body.String())
	}
	if env.Data.Sort != 100 {
		t.Fatalf("first rule should get sort=100, got %d", env.Data.Sort)
	}

	// Get.
	rec = s.do(http.MethodGet, "/api/rules/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d body=%s", rec.Code, rec.Body.String())
	}

	// Update: rename + disable.
	enabled := false
	updReq := types.UpdateRuleRequest{Name: "ads-disabled", Enabled: &enabled}
	rec = s.do(http.MethodPut, "/api/rules/"+id, updReq, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d body=%s", rec.Code, rec.Body.String())
	}
	var env2 ruleEnvelope[types.CustomRule]
	_ = json.Unmarshal(rec.Body.Bytes(), &env2)
	if env2.Data.Name != "ads-disabled" {
		t.Fatalf("name not updated: %q", env2.Data.Name)
	}
	if env2.Data.Enabled {
		t.Fatalf("enabled flag should be false")
	}

	// Delete.
	rec = s.do(http.MethodDelete, "/api/rules/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d body=%s", rec.Code, rec.Body.String())
	}
	// 404 on follow-up GET.
	rec = s.do(http.MethodGet, "/api/rules/"+id, nil, tok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuleHandler_ValidateTypeAndMode(t *testing.T) {
	s := newRuleTestStack(t)
	_, tok := s.createUser("bob")

	bad := map[string]any{
		"name": "x", "type": "garbage", "mode": "replace",
		"content": "x: y\n", "enabled": true,
	}
	rec := s.do(http.MethodPost, "/api/rules", bad, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad type, got %d body=%s", rec.Code, rec.Body.String())
	}

	bad2 := map[string]any{
		"name": "x", "type": "rules", "mode": "garbage",
		"content": "MATCH,DIRECT\n", "enabled": true,
	}
	rec = s.do(http.MethodPost, "/api/rules", bad2, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad mode, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuleHandler_ListAndFilter(t *testing.T) {
	s := newRuleTestStack(t)
	_, tok := s.createUser("carol")

	for i, kind := range []types.RuleType{
		types.RuleTypeRules, types.RuleTypeDNS, types.RuleTypeRuleProviders, types.RuleTypeRules,
	} {
		req := types.CreateRuleRequest{
			Name: "r" + string(rune('a'+i)), Type: kind, Mode: types.RuleModeReplace,
			Content: dummyContent(kind), Enabled: true,
		}
		rec := s.do(http.MethodPost, "/api/rules", req, tok)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %d: %d body=%s", i, rec.Code, rec.Body.String())
		}
	}

	rec := s.do(http.MethodGet, "/api/rules?type=rules", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var env ruleEnvelope[types.PagedResponse[types.CustomRule]]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.Total != 2 {
		t.Fatalf("type=rules total: %d body=%s", env.Data.Total, rec.Body.String())
	}
}

func TestRuleHandler_Reorder(t *testing.T) {
	s := newRuleTestStack(t)
	_, tok := s.createUser("dan")
	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		req := types.CreateRuleRequest{
			Name: "r" + string(rune('a'+i)), Type: types.RuleTypeRules, Mode: types.RuleModeAppend,
			Content: "MATCH,Proxy\n", Enabled: true,
		}
		rec := s.do(http.MethodPost, "/api/rules", req, tok)
		var env ruleEnvelope[types.CustomRule]
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		ids = append(ids, env.Data.ID)
	}
	// Reorder: swap first and last.
	body := types.UpdateRuleOrderRequest{
		Orders: []types.RuleOrder{
			{ID: ids[0], Sort: 999},
			{ID: ids[2], Sort: 50},
		},
	}
	rec := s.do(http.MethodPost, "/api/rules/reorder", body, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder: %d body=%s", rec.Code, rec.Body.String())
	}
	// Verify via list.
	rec = s.do(http.MethodGet, "/api/rules", nil, tok)
	var env ruleEnvelope[types.PagedResponse[types.CustomRule]]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Data.Items[0].ID != ids[2] {
		t.Fatalf("expected ids[2] first after reorder; got %v",
			[]string{env.Data.Items[0].ID, env.Data.Items[1].ID, env.Data.Items[2].ID})
	}
	if env.Data.Items[2].ID != ids[0] {
		t.Fatalf("expected ids[0] last after reorder; got %s", env.Data.Items[2].ID)
	}
}

func TestRuleHandler_Templates(t *testing.T) {
	s := newRuleTestStack(t)
	_, tok := s.createUser("eve")
	rec := s.do(http.MethodGet, "/api/rules/templates", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("templates: %d body=%s", rec.Code, rec.Body.String())
	}
	var env ruleEnvelope[[]types.RuleTemplate]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if len(env.Data) != 3 {
		t.Fatalf("want 3 templates, got %d", len(env.Data))
	}
	ids := map[string]bool{}
	for _, tpl := range env.Data {
		ids[tpl.ID] = true
	}
	for _, want := range []string{"cn-direct-foreign-proxy", "global-proxy", "ad-block"} {
		if !ids[want] {
			t.Fatalf("missing template %s; got %v", want, env.Data)
		}
	}
}

func TestRuleHandler_Preview_NoSubscription(t *testing.T) {
	s := newRuleTestStack(t)
	_, tok := s.createUser("frank")
	// Preview against unknown sub → 404.
	rec := s.do(http.MethodGet, "/api/rules/preview/missing-sub", nil, tok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRuleHandler_Preview_AppliesRules(t *testing.T) {
	s := newRuleTestStack(t)
	uid, tok := s.createUser("grace")

	// Seed a subscription so the preview path can succeed.
	subRepo := storage.NewSubscriptionRepo(getDBFromStack(t, s), time.Now)
	_, err := subRepo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-preview", UserID: uid, Name: "primary",
		Type: string(types.SubTypeManual), SyncInterval: 21600,
	})
	if err != nil {
		t.Fatalf("seed sub: %v", err)
	}

	// Add an enabled rule that injects a recognisable line.
	createReq := types.CreateRuleRequest{
		Name: "magic", Type: types.RuleTypeRules, Mode: types.RuleModeAppend,
		Content: "DOMAIN-SUFFIX,magic.test,REJECT\n", Enabled: true,
	}
	if rec := s.do(http.MethodPost, "/api/rules", createReq, tok); rec.Code != http.StatusCreated {
		t.Fatalf("create rule: %d body=%s", rec.Code, rec.Body.String())
	}

	rec := s.do(http.MethodGet, "/api/rules/preview/sub-preview", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "magic.test") {
		t.Fatalf("preview did not include rule line: %s", rec.Body.String())
	}
}

// dummyContent returns a minimal valid content payload for the given type so
// the create handler's validation does not reject it.
func dummyContent(kind types.RuleType) string {
	switch kind {
	case types.RuleTypeDNS:
		return "nameservers: [1.1.1.1]\n"
	case types.RuleTypeRuleProviders:
		return "x:\n  type: http\n  behavior: domain\n  url: https://example.com/x.yaml\n  path: ./x.yaml\n  interval: 86400\n"
	}
	return "MATCH,Proxy\n"
}

// getDBFromStack returns the live storage.DB the stack is built around.
func getDBFromStack(t *testing.T, s *ruleTestStack) *storage.DB {
	t.Helper()
	return s.dbRef
}
