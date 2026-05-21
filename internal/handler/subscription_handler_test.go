package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/types"
)

// subTestStack mirrors authTestStack but adds the subscription handler so the
// full HTTP surface is reachable.
type subTestStack struct {
	t          *testing.T
	mux        http.Handler
	users      *storage.UserRepo
	subs       *storage.SubscriptionRepo
	sync       *substore.SyncService
	tokens     *auth.TokenStore
	mgr        *auth.Manager
	subHandler *SubscriptionHandler
	db         *storage.DB
}

func newSubTestStack(t *testing.T) *subTestStack {
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
	syncSvc, err := substore.NewSyncService(substore.SyncServiceConfig{
		Repo: subs, NodeRepo: substore.NoopNodeRepo{},
	})
	if err != nil {
		t.Fatalf("NewSyncService: %v", err)
	}
	subHandler := NewSubscriptionHandler(subs, syncSvc, nil)

	deps := &Deps{
		DB:                  db,
		AuthManager:         mgr,
		TokenStore:          tokens,
		UserRepo:            users,
		SessionRepo:         sessions,
		TOTPManager:         totpMgr,
		SubscriptionHandler: subHandler,
		LoginRateLimit:      ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &subTestStack{
		t: t, mux: mux,
		users: users, subs: subs, sync: syncSvc,
		tokens:     tokens,
		mgr:        mgr,
		subHandler: subHandler,
		db:         db,
	}
}

// createUserWithToken provisions a user and mints a session token bypassing
// the brute-force checker.
func (s *subTestStack) createUserWithToken(username string) (userID, token string) {
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
		s.t.Fatalf("Issue: %v", err)
	}
	return user.ID, tok
}

// do mirrors authTestStack.do but lets tests pass an arbitrary io.Reader so
// multipart bodies can flow through.
func (s *subTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

type subEnvelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestSubscriptionRequiresAuth(t *testing.T) {
	s := newSubTestStack(t)
	rec := s.do(http.MethodGet, "/api/subscriptions", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubscriptionCreateAndList(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("alice")
	createReq := types.CreateSubscriptionRequest{
		Name:      "primary",
		Type:      types.SubTypeManual,
		Tags:      []string{"prod"},
		Remark:    "first",
	}
	rec := s.do(http.MethodPost, "/api/subscriptions", createReq, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var createEnv subEnvelope[types.SubscriptionDetail]
	if err := json.Unmarshal(rec.Body.Bytes(), &createEnv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if createEnv.Data.ID == "" {
		t.Fatalf("expected id in response, body=%s", rec.Body.String())
	}
	if createEnv.Data.ShareToken == "" {
		t.Fatalf("expected share_token on detail, body=%s", rec.Body.String())
	}

	// List should return 1 item, NO share_token field.
	listRec := s.do(http.MethodGet, "/api/subscriptions", nil, tok)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: status=%d", listRec.Code)
	}
	var listEnv subEnvelope[types.PagedResponse[types.Subscription]]
	if err := json.Unmarshal(listRec.Body.Bytes(), &listEnv); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if listEnv.Data.Total != 1 {
		t.Fatalf("expected total=1, got %d", listEnv.Data.Total)
	}
	if strings.Contains(listRec.Body.String(), "share_token") {
		t.Fatalf("list response unexpectedly contains share_token: %s", listRec.Body.String())
	}
}

func TestSubscriptionGetIncludesShareToken(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("bob")
	rec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: "sub", Type: types.SubTypeManual,
	}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	var env subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID

	getRec := s.do(http.MethodGet, "/api/subscriptions/"+id, nil, tok)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: %d body=%s", getRec.Code, getRec.Body.String())
	}
	var getEnv subEnvelope[types.SubscriptionDetail]
	if err := json.Unmarshal(getRec.Body.Bytes(), &getEnv); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if getEnv.Data.ShareToken == "" {
		t.Fatalf("get must include share_token, body=%s", getRec.Body.String())
	}
}

func TestSubscriptionRotateShareToken(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("carol")
	rec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: "rotsub", Type: types.SubTypeManual,
	}, tok)
	var createEnv subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(rec.Body.Bytes(), &createEnv)
	id := createEnv.Data.ID
	old := createEnv.Data.ShareToken

	rotRec := s.do(http.MethodPost, "/api/subscriptions/"+id+"/rotate-share-token", nil, tok)
	if rotRec.Code != http.StatusOK {
		t.Fatalf("rotate: %d body=%s", rotRec.Code, rotRec.Body.String())
	}
	var rotEnv subEnvelope[types.RotateShareTokenResponse]
	if err := json.Unmarshal(rotRec.Body.Bytes(), &rotEnv); err != nil {
		t.Fatalf("decode rot: %v", err)
	}
	if rotEnv.Data.ShareToken == "" || rotEnv.Data.ShareToken == old {
		t.Fatalf("rotation did not produce new token: old=%q new=%q", old, rotEnv.Data.ShareToken)
	}
}

func TestSubscriptionUpdate(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("dave")
	rec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: "before", Type: types.SubTypeManual,
	}, tok)
	var env subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID

	patchRec := s.do(http.MethodPatch, "/api/subscriptions/"+id, types.UpdateSubscriptionRequest{
		Name: "after", Remark: "updated",
	}, tok)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch: %d body=%s", patchRec.Code, patchRec.Body.String())
	}
	var patchEnv subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(patchRec.Body.Bytes(), &patchEnv)
	if patchEnv.Data.Name != "after" || patchEnv.Data.Remark != "updated" {
		t.Fatalf("expected patch to apply, got %+v", patchEnv.Data)
	}
}

func TestSubscriptionDelete(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("eve")
	rec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: "delsub", Type: types.SubTypeManual,
	}, tok)
	var env subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID

	delRec := s.do(http.MethodDelete, "/api/subscriptions/"+id, nil, tok)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete: %d body=%s", delRec.Code, delRec.Body.String())
	}
	getRec := s.do(http.MethodGet, "/api/subscriptions/"+id, nil, tok)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getRec.Code)
	}
}

func TestSubscriptionCrossUserIsolation(t *testing.T) {
	s := newSubTestStack(t)
	_, aliceTok := s.createUserWithToken("alice")
	_, bobTok := s.createUserWithToken("bob")

	// Alice creates a subscription.
	rec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: "alice-sub", Type: types.SubTypeManual,
	}, aliceTok)
	var env subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	id := env.Data.ID

	// Bob cannot read it.
	getRec := s.do(http.MethodGet, "/api/subscriptions/"+id, nil, bobTok)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-user read, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	// Bob cannot delete it.
	delRec := s.do(http.MethodDelete, "/api/subscriptions/"+id, nil, bobTok)
	if delRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-user delete, got %d", delRec.Code)
	}
}

func TestSubscriptionUpload(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("frank")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "uploaded")
	_ = mw.WriteField("tags", "fast,uploaded")
	fileWriter, err := mw.CreateFormFile("file", "sub.yaml")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	_, _ = fileWriter.Write([]byte("proxies:\n  - name: x\n    type: vmess\n    server: 1.1.1.1\n    port: 443\n    uuid: u\n"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d body=%s", rec.Code, rec.Body.String())
	}
	var env subEnvelope[types.SubscriptionDetail]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Type != types.SubTypeUpload {
		t.Fatalf("expected type=upload, got %q", env.Data.Type)
	}
	if len(env.Data.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(env.Data.Tags))
	}
}

func TestSubscriptionNotFoundReturns404(t *testing.T) {
	s := newSubTestStack(t)
	_, tok := s.createUserWithToken("gina")
	rec := s.do(http.MethodGet, "/api/subscriptions/does-not-exist", nil, tok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestSubscriptionPipelines_Bindings_RoundTrip covers Bug-4 (review-round1):
// the GET / PUT /api/subscriptions/{id}/pipelines routes round-trip a list
// of pipeline bindings. The setup wires a fresh stack that includes the
// PipelineRepo (the default subTestStack omits it, so the endpoint 501s
// there).
func TestSubscriptionPipelines_Bindings_RoundTrip(t *testing.T) {
	s := newSubTestStack(t)
	pipelineRepo := storage.NewPipelineRepo(s.db, time.Now)
	// Inject the pipeline repo into the existing subscription handler. The
	// router was constructed in newSubTestStack — the handler instance is
	// shared, so SetPipelineRepo takes effect immediately.
	s.subHandler.SetPipelineRepo(pipelineRepo)

	_, tok := s.createUserWithToken("paul")
	// Create a subscription.
	subRec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: "primary", Type: types.SubTypeManual,
	}, tok)
	if subRec.Code != http.StatusCreated {
		t.Fatalf("create sub: %d body=%s", subRec.Code, subRec.Body.String())
	}
	var subEnv subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(subRec.Body.Bytes(), &subEnv)
	subID := subEnv.Data.ID

	// Seed two pipelines belonging to paul.
	pipeA, err := pipelineRepo.Create(context.Background(), storage.PipelineRecord{
		ID: "pipe-a", UserID: "paul-id", Name: "A", ASTJSON: "[]", YAMLContent: "operators: []\n",
	})
	if err != nil {
		t.Fatalf("create pipe a: %v", err)
	}
	pipeB, err := pipelineRepo.Create(context.Background(), storage.PipelineRecord{
		ID: "pipe-b", UserID: "paul-id", Name: "B", ASTJSON: "[]", YAMLContent: "operators: []\n",
	})
	if err != nil {
		t.Fatalf("create pipe b: %v", err)
	}

	// Initially: empty list.
	listRec := s.do(http.MethodGet, "/api/subscriptions/"+subID+"/pipelines", nil, tok)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list empty: %d body=%s", listRec.Code, listRec.Body.String())
	}

	// PUT bindings.
	putBody := types.UpdatePipelineBindingsRequest{
		Bindings: []types.PipelineBindingInput{
			{PipelineID: pipeA.ID, Position: 1, Enabled: true},
			{PipelineID: pipeB.ID, Position: 2, Enabled: false},
		},
	}
	putRec := s.do(http.MethodPut, "/api/subscriptions/"+subID+"/pipelines", putBody, tok)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put bindings: %d body=%s", putRec.Code, putRec.Body.String())
	}

	// GET back the bindings.
	listRec2 := s.do(http.MethodGet, "/api/subscriptions/"+subID+"/pipelines", nil, tok)
	if listRec2.Code != http.StatusOK {
		t.Fatalf("list 2: %d body=%s", listRec2.Code, listRec2.Body.String())
	}
	var env subEnvelope[[]types.PipelineBinding]
	if err := json.Unmarshal(listRec2.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 bindings, got %d: %+v", len(env.Data), env.Data)
	}
	if env.Data[0].PipelineID != pipeA.ID || env.Data[1].PipelineID != pipeB.ID {
		t.Fatalf("unexpected binding order: %+v", env.Data)
	}
}

// TestSubscriptionPipelines_RejectsForeignPipeline ensures cross-user
// access is denied (information hiding: 404).
func TestSubscriptionPipelines_RejectsForeignPipeline(t *testing.T) {
	s := newSubTestStack(t)
	pipelineRepo := storage.NewPipelineRepo(s.db, time.Now)
	s.subHandler.SetPipelineRepo(pipelineRepo)

	_, aliceTok := s.createUserWithToken("alicep")
	_, _ = s.createUserWithToken("bobp")

	// Alice owns a subscription.
	subRec := s.do(http.MethodPost, "/api/subscriptions", types.CreateSubscriptionRequest{
		Name: "primary", Type: types.SubTypeManual,
	}, aliceTok)
	var subEnv subEnvelope[types.SubscriptionDetail]
	_ = json.Unmarshal(subRec.Body.Bytes(), &subEnv)
	subID := subEnv.Data.ID

	// Bob owns a pipeline.
	bobPipe, err := pipelineRepo.Create(context.Background(), storage.PipelineRecord{
		ID: "bob-pipe", UserID: "bobp-id", Name: "B", ASTJSON: "[]", YAMLContent: "operators: []\n",
	})
	if err != nil {
		t.Fatalf("create bob pipe: %v", err)
	}

	// Alice tries to bind Bob's pipeline → 404 pipeline not found.
	putRec := s.do(http.MethodPut, "/api/subscriptions/"+subID+"/pipelines",
		types.UpdatePipelineBindingsRequest{
			Bindings: []types.PipelineBindingInput{{PipelineID: bobPipe.ID, Position: 1, Enabled: true}},
		}, aliceTok)
	if putRec.Code != http.StatusNotFound {
		t.Fatalf("cross-user pipeline: expected 404, got %d body=%s",
			putRec.Code, putRec.Body.String())
	}
}

