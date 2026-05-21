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

// pipeTestStack wires the bare minimum for /api/pipelines/* HTTP tests.
type pipeTestStack struct {
	t       *testing.T
	mux     http.Handler
	repo    *storage.PipelineRepo
	subRepo *storage.SubscriptionRepo
	users   *storage.UserRepo
	tokens  *auth.TokenStore
}

func newPipeTestStack(t *testing.T) *pipeTestStack {
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
	pipes := storage.NewPipelineRepo(db, time.Now)
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
	ph := NewPipelineHandler(pipes, subs, nil)

	deps := &Deps{
		DB:              db,
		AuthManager:     mgr,
		TokenStore:      tokens,
		UserRepo:        users,
		SessionRepo:     sessions,
		TOTPManager:     totpMgr,
		PipelineHandler: ph,
		LoginRateLimit:  ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &pipeTestStack{
		t: t, mux: mux,
		repo: pipes, subRepo: subs, users: users, tokens: tokens,
	}
}

func (s *pipeTestStack) createUser(username string) (userID, token string) {
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

func (s *pipeTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

type pipeEnvelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

const sampleYAML = `apiVersion: shiguang/v1
operators:
  - kind: filter
    args:
      expr: protocol == "vmess"
  - kind: output
    args:
      format: clash
`

func TestPipelineHandler_RequiresAuth(t *testing.T) {
	s := newPipeTestStack(t)
	rec := s.do(http.MethodGet, "/api/pipelines", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPipelineHandler_CreateGetUpdateDelete(t *testing.T) {
	s := newPipeTestStack(t)
	_, tok := s.createUser("alice")

	// Create.
	createReq := types.CreatePipelineRequest{
		Name:        "p1",
		YAMLContent: sampleYAML,
	}
	rec := s.do(http.MethodPost, "/api/pipelines", createReq, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var env pipeEnvelope[types.Pipeline]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, rec.Body.String())
	}
	id := env.Data.ID
	if id == "" {
		t.Fatalf("missing id in response: %s", rec.Body.String())
	}
	if env.Data.Version != 1 {
		t.Fatalf("want version=1, got %d", env.Data.Version)
	}

	// Get.
	rec = s.do(http.MethodGet, "/api/pipelines/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Update with new yaml + version=1.
	upd := types.UpdatePipelineRequest{
		Name:        "p1-renamed",
		YAMLContent: sampleYAML,
		Version:     1,
	}
	rec = s.do(http.MethodPut, "/api/pipelines/"+id, upd, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var env2 pipeEnvelope[types.Pipeline]
	_ = json.Unmarshal(rec.Body.Bytes(), &env2)
	if env2.Data.Name != "p1-renamed" {
		t.Fatalf("name not updated: %q", env2.Data.Name)
	}
	if env2.Data.Version != 2 {
		t.Fatalf("version not bumped: %d", env2.Data.Version)
	}

	// Stale version → 409.
	stale := types.UpdatePipelineRequest{YAMLContent: sampleYAML, Version: 1}
	rec = s.do(http.MethodPut, "/api/pipelines/"+id, stale, tok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale version should 409, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Delete.
	rec = s.do(http.MethodDelete, "/api/pipelines/"+id, nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// GET after delete → 404.
	rec = s.do(http.MethodGet, "/api/pipelines/"+id, nil, tok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete should 404, got %d", rec.Code)
	}
}

func TestPipelineHandler_CrossUserIsolated(t *testing.T) {
	s := newPipeTestStack(t)
	_, alice := s.createUser("alice2")
	_, bob := s.createUser("bob2")

	rec := s.do(http.MethodPost, "/api/pipelines",
		types.CreatePipelineRequest{Name: "p", YAMLContent: sampleYAML}, alice)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var env pipeEnvelope[types.Pipeline]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)

	// Bob cannot read.
	rec = s.do(http.MethodGet, "/api/pipelines/"+env.Data.ID, nil, bob)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user GET should 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	// Bob cannot delete.
	rec = s.do(http.MethodDelete, "/api/pipelines/"+env.Data.ID, nil, bob)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user DELETE should 404, got %d", rec.Code)
	}
}

func TestPipelineHandler_BadYAMLRejected(t *testing.T) {
	s := newPipeTestStack(t)
	_, tok := s.createUser("alice3")
	rec := s.do(http.MethodPost, "/api/pipelines",
		types.CreatePipelineRequest{Name: "bad", YAMLContent: "apiVersion: wrong\noperators: []"}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad apiVersion should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), string(types.ErrValidationSchemaMismatch)) {
		t.Fatalf("want schema mismatch code, got body=%s", rec.Body.String())
	}
}

func TestPipelineHandler_YAMLToAST_ASTToYAML(t *testing.T) {
	s := newPipeTestStack(t)
	_, tok := s.createUser("alice4")

	// YAML → AST.
	rec := s.do(http.MethodPost, "/api/pipelines/yaml-to-ast",
		types.YAMLToASTRequest{YAMLContent: sampleYAML}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("yaml-to-ast: %d %s", rec.Code, rec.Body.String())
	}
	var env pipeEnvelope[map[string]string]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	astJSON := env.Data["ast_json"]
	if !strings.Contains(astJSON, "shiguang/v1") {
		t.Fatalf("ast_json missing api_version: %q", astJSON)
	}

	// AST → YAML round-trip.
	rec = s.do(http.MethodPost, "/api/pipelines/ast-to-yaml",
		types.ASTToYAMLRequest{ASTJson: astJSON}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("ast-to-yaml: %d %s", rec.Code, rec.Body.String())
	}
	var env2 pipeEnvelope[map[string]string]
	_ = json.Unmarshal(rec.Body.Bytes(), &env2)
	if !strings.Contains(env2.Data["yaml_content"], "apiVersion: shiguang/v1") {
		t.Fatalf("round-trip lost apiVersion: %q", env2.Data["yaml_content"])
	}
}

func TestPipelineHandler_Operators(t *testing.T) {
	s := newPipeTestStack(t)
	_, tok := s.createUser("alice5")
	rec := s.do(http.MethodGet, "/api/pipelines/operators", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("operators: %d %s", rec.Code, rec.Body.String())
	}
	var env pipeEnvelope[[]types.OperatorSchema]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data) != 6 {
		t.Fatalf("expected 6 operators, got %d", len(env.Data))
	}
	seen := map[string]bool{}
	for _, op := range env.Data {
		seen[string(op.Type)] = true
	}
	for _, kind := range []string{"filter", "map", "sort", "dedupe", "regex_rename", "output"} {
		if !seen[kind] {
			t.Fatalf("operator catalog missing %q: %+v", kind, env.Data)
		}
	}
}

func TestPipelineHandler_RunWithInputNodes(t *testing.T) {
	s := newPipeTestStack(t)
	_, tok := s.createUser("alice6")

	// Create a pipeline that filters to vless then outputs.
	yamlBody := `apiVersion: shiguang/v1
operators:
  - kind: filter
    args:
      expr: protocol == "vless"
  - kind: output
    args:
      format: clash
`
	rec := s.do(http.MethodPost, "/api/pipelines",
		types.CreatePipelineRequest{Name: "vless-only", YAMLContent: yamlBody}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var env pipeEnvelope[types.Pipeline]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID

	// Run with input_nodes: a vless URI and an ss URI; only vless should pass.
	body := map[string]any{
		"input_nodes": []string{
			"vless://aa-bb-cc-dd-ee@1.1.1.1:443?encryption=none#HK-test",
			"ss://YWVzLTI1Ni1nY206cHc=@2.2.2.2:8388#JP-test",
		},
		"debug": true,
	}
	rec = s.do(http.MethodPost, "/api/pipelines/"+id+"/run", body, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	var env2 pipeEnvelope[types.RunPipelineResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env2.Data.OutputCount != 1 {
		t.Fatalf("expected 1 output node (only vless survives), got %d", env2.Data.OutputCount)
	}
	if len(env2.Data.Steps) == 0 {
		t.Fatalf("debug=true must return steps")
	}
}

func TestPipelineHandler_RunRequiresInput(t *testing.T) {
	s := newPipeTestStack(t)
	_, tok := s.createUser("alice7")
	rec := s.do(http.MethodPost, "/api/pipelines",
		types.CreatePipelineRequest{Name: "p", YAMLContent: sampleYAML}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var env pipeEnvelope[types.Pipeline]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)

	// No subscription_id or input_nodes → 400.
	rec = s.do(http.MethodPost, "/api/pipelines/"+env.Data.ID+"/run",
		map[string]any{}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing input should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
