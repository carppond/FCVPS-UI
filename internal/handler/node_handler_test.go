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

// nodeTestStack bundles the dependencies the M-NODE handlers exercise. It is
// intentionally closer to subTestStack but adds the NodeHandler + TCPingHandler
// wiring so the routes mounted by mountNodeRoutes / mountTCPingRoutes are
// reachable.
type nodeTestStack struct {
	t              *testing.T
	mux            http.Handler
	users          *storage.UserRepo
	subs           *storage.SubscriptionRepo
	nodes          *storage.NodeRepo
	tokens         *auth.TokenStore
	tcpingHandler  *TCPingHandler
}

func newNodeTestStack(t *testing.T) *nodeTestStack {
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
	nodes := storage.NewNodeRepo(db, time.Now)
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

	nodeHandler := NewNodeHandler(nodes, subs, nil)
	tcping := NewTCPingHandler(nodes, nil)
	// Inject a deterministic dialer so tests can simulate reachable +
	// unreachable hosts without depending on the real network.
	tcping.dial = stubDialFor(t)

	deps := &Deps{
		DB:              db,
		AuthManager:     mgr,
		TokenStore:      tokens,
		UserRepo:        users,
		SessionRepo:     sessions,
		TOTPManager:     totpMgr,
		NodeHandler:     nodeHandler,
		TCPingHandler:   tcping,
		LoginRateLimit:  ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &nodeTestStack{
		t: t, mux: mux,
		users: users, subs: subs, nodes: nodes,
		tokens:        tokens,
		tcpingHandler: tcping,
	}
}

// createUserWithToken provisions a user and mints a bearer token bypassing
// the brute-force checker (mirrors subTestStack.createUserWithToken).
func (s *nodeTestStack) createUserWithToken(username string) (string, string) {
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

// seedSub creates a subscription owned by userID and returns its ID.
func (s *nodeTestStack) seedSub(userID, name string, subType types.SubType) string {
	rec, err := s.subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: name + "-id", UserID: userID, Name: name, Type: string(subType),
	})
	if err != nil {
		s.t.Fatalf("seedSub: %v", err)
	}
	return rec.ID
}

// seedNode inserts a node directly via the repo so list-style tests don't
// need to go through the manual-create HTTP path.
func (s *nodeTestStack) seedNode(subID, id, proto, server string, port int32) {
	if _, err := s.nodes.Create(context.Background(), storage.NodeRecord{
		ID: id, SubscriptionID: subID,
		RawURI: proto + "://" + id, Protocol: proto,
		Server: server, Port: port, Tag: id,
	}); err != nil {
		s.t.Fatalf("seedNode: %v", err)
	}
}

func (s *nodeTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

type nodeEnvelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestNodeRoutesRequireAuth(t *testing.T) {
	s := newNodeTestStack(t)
	if rec := s.do(http.MethodGet, "/api/nodes", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauth list, got %d", rec.Code)
	}
}

func TestNodeListByUserReturnsPagedItems(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("alice")
	subID := s.seedSub("alice-id", "primary", types.SubTypeManual)
	s.seedNode(subID, "n-1", "vmess", "1.1.1.1", 443)
	s.seedNode(subID, "n-2", "trojan", "2.2.2.2", 443)

	rec := s.do(http.MethodGet, "/api/nodes", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d body=%s", rec.Code, rec.Body.String())
	}
	var env nodeEnvelope[types.PagedResponse[types.NodeWithLatency]]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Total != 2 || len(env.Data.Items) != 2 {
		t.Fatalf("expected total=2 items=2, got %+v", env.Data)
	}
}

func TestNodeManualCreate(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("bob")
	subID := s.seedSub("bob-id", "manual", types.SubTypeManual)

	// Use a trivial ss URI so substore.ParseURI succeeds without depending on
	// the more complex protocols. The fragment becomes the proxy name.
	body := types.AddNodeRequest{
		RawURI: "ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@1.2.3.4:8388#example",
	}
	rec := s.do(http.MethodPost, "/api/subscriptions/"+subID+"/nodes", body, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", rec.Code, rec.Body.String())
	}
	var env nodeEnvelope[types.NodeWithLatency]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.ID == "" || env.Data.Server != "1.2.3.4" || env.Data.Port != 8388 {
		t.Fatalf("expected parsed ss node, got %+v", env.Data)
	}
}

func TestNodeManualCreateRejectsURLSubscription(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("carol")
	subID := s.seedSub("carol-id", "url-sub", types.SubTypeURL)

	rec := s.do(http.MethodPost, "/api/subscriptions/"+subID+"/nodes",
		types.AddNodeRequest{RawURI: "ss://aaa@1.1.1.1:1#x"}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-manual sub, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNodeUpdateAndDelete(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("dave")
	subID := s.seedSub("dave-id", "primary", types.SubTypeManual)
	s.seedNode(subID, "n-x", "ss", "1.1.1.1", 8388)

	patch := types.UpdateNodeRequest{Tags: []string{"a", "b"}}
	rec := s.do(http.MethodPatch, "/api/nodes/n-x", patch, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d body=%s", rec.Code, rec.Body.String())
	}
	var env nodeEnvelope[types.NodeWithLatency]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if len(env.Data.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %+v", env.Data.Tags)
	}

	del := s.do(http.MethodDelete, "/api/nodes/n-x", nil, tok)
	if del.Code != http.StatusOK {
		t.Fatalf("delete: %d body=%s", del.Code, del.Body.String())
	}
	if rec := s.do(http.MethodGet, "/api/nodes/n-x", nil, tok); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestNodeCrossUserIsolation(t *testing.T) {
	s := newNodeTestStack(t)
	_, aliceTok := s.createUserWithToken("alice")
	_, bobTok := s.createUserWithToken("bob")
	subID := s.seedSub("alice-id", "primary", types.SubTypeManual)
	s.seedNode(subID, "n-priv", "ss", "1.1.1.1", 8388)

	if rec := s.do(http.MethodGet, "/api/nodes/n-priv", nil, bobTok); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 cross-user, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := s.do(http.MethodDelete, "/api/nodes/n-priv", nil, bobTok); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 delete cross-user, got %d", rec.Code)
	}
	// Alice can still see it.
	if rec := s.do(http.MethodGet, "/api/nodes/n-priv", nil, aliceTok); rec.Code != http.StatusOK {
		t.Fatalf("owner read failed: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNodeCopyURI(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("eve")
	subID := s.seedSub("eve-id", "primary", types.SubTypeManual)
	s.seedNode(subID, "n-copy", "ss", "1.1.1.1", 8388)
	rec := s.do(http.MethodPost, "/api/nodes/n-copy/copy-uri", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("copy-uri: %d body=%s", rec.Code, rec.Body.String())
	}
	var env nodeEnvelope[map[string]string]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data["raw_uri"] != "ss://n-copy" {
		t.Fatalf("expected raw_uri round-trip, got %q", env.Data["raw_uri"])
	}
}
