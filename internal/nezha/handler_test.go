package nezha_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/nezha"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

// stubAdapter records every OnHeartbeat invocation so tests can assert the
// handler wired through correctly without bringing up the real adapter.
type stubAdapter struct {
	mu        sync.Mutex
	calls     []stubCall
	returnErr error
}

type stubCall struct {
	agentID string
	hb      nezha.NezhaHeartbeat
}

func (s *stubAdapter) OnHeartbeat(_ context.Context, agentID string, hb nezha.NezhaHeartbeat) error {
	s.mu.Lock()
	s.calls = append(s.calls, stubCall{agentID: agentID, hb: hb})
	s.mu.Unlock()
	return s.returnErr
}

func (s *stubAdapter) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *stubAdapter) last() stubCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return stubCall{}
	}
	return s.calls[len(s.calls)-1]
}

// nezhaTestStack wires the handler against a tmp-dir SQLite DB so the secret
// lookup goes through the real AgentRepo.
type nezhaTestStack struct {
	t       *testing.T
	repo    *storage.AgentRepo
	adapter *stubAdapter
	handler *nezha.Handler
}

func newNezhaTestStack(t *testing.T) *nezhaTestStack {
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
		t.Fatalf("migrate: %v", err)
	}
	// agents.user_id has a FK on users(id); seed a synthetic user so the
	// AgentRepo.Create call in seedAgent succeeds.
	users := storage.NewUserRepo(db, time.Now)
	if _, err := users.Create(context.Background(), storage.UserRecord{
		ID: "test-user", Username: "tester", PasswordHash: "h",
		Role: "user", IsActive: true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	repo := storage.NewAgentRepo(db, time.Now)
	// The handler also writes via the agent_records repo through the adapter,
	// but we stub the adapter so we only need the agents table populated.
	adapter := &stubAdapter{}
	h := nezha.NewHandler(nezha.HandlerConfig{
		AgentRepo: repo,
		Adapter:   adapter,
	})
	return &nezhaTestStack{t: t, repo: repo, adapter: adapter, handler: h}
}

// seedAgent inserts an agent row of the given kind and returns the plaintext
// secret + agent ID. The token-hashing path matches what the production
// handler does (sha256 hex of the secret).
func (s *nezhaTestStack) seedAgent(t *testing.T, kind string) (id, secret string) {
	t.Helper()
	// Create a user-id sentinel — we don't run the user repo here so the FK
	// is satisfied via the migration's relaxed setup. AgentRepo only requires
	// non-empty user_id.
	secret = util.RandomBase64URL(16)
	rec := storage.AgentRecord{
		ID:        util.UUIDv7(),
		UserID:    "test-user",
		Name:      "n",
		TokenHash: util.SHA256Hex(secret),
		Kind:      kind,
	}
	created, err := s.repo.Create(context.Background(), rec)
	if err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	return created.ID, secret
}

func (s *nezhaTestStack) post(target string, headers http.Header, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	var reader *bytes.Reader
	switch v := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case []byte:
		reader = bytes.NewReader(v)
	case string:
		reader = bytes.NewReader([]byte(v))
	default:
		buf, _ := json.Marshal(body)
		reader = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(http.MethodPost, target, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func validBody(secret string) map[string]any {
	return map[string]any{
		"secret": secret,
		"state": map[string]any{
			"cpu":               12.5,
			"mem_used":          1024,
			"disk_used":         2048,
			"net_in_speed":      11,
			"net_out_speed":     22,
			"net_in_transfer":   100,
			"net_out_transfer":  200,
			"load_1":            0.5,
			"load_5":            0.3,
			"load_15":           0.2,
			"tcp_conn_count":    10,
			"udp_conn_count":    20,
			"process_count":     100,
			"uptime":            86400,
		},
		"host": map[string]any{
			"platform":  "linux",
			"arch":      "amd64",
			"mem_total": 8 * 1024 * 1024 * 1024,
		},
	}
}

// TestHandlerAcceptsValidHeartbeat — happy path: secret in body, kind matches.
func TestHandlerAcceptsValidHeartbeat(t *testing.T) {
	s := newNezhaTestStack(t)
	id, secret := s.seedAgent(t, "nezha_compat")

	rec := s.post("/api/v1/nezha/heartbeat", nil, validBody(secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if code, _ := resp["code"].(float64); code != 0 {
		t.Fatalf("expected code=0, got %v (body=%s)", resp["code"], rec.Body.String())
	}
	if msg, _ := resp["message"].(string); msg != "ok" {
		t.Fatalf("expected message=ok, got %v", resp["message"])
	}
	if got := s.adapter.callCount(); got != 1 {
		t.Fatalf("expected 1 adapter call, got %d", got)
	}
	if got := s.adapter.last().agentID; got != id {
		t.Fatalf("adapter received wrong agent id: %s want %s", got, id)
	}
}

// TestHandlerAcceptsBearerHeader — secret via Authorization header takes
// precedence over the body. We seed a single agent and use the wrong body
// secret to prove the header value is what authenticated us.
func TestHandlerAcceptsBearerHeader(t *testing.T) {
	s := newNezhaTestStack(t)
	_, secret := s.seedAgent(t, "nezha_compat")

	body := validBody("not-the-real-secret")
	headers := http.Header{"Authorization": []string{"Bearer " + secret}}
	rec := s.post("/api/v1/nezha/heartbeat", headers, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for bearer auth, got %d body=%s", rec.Code, rec.Body.String())
	}
	if s.adapter.callCount() != 1 {
		t.Fatalf("expected adapter invoked once via bearer path")
	}
}

// TestHandlerAcceptsQuerySecret — secret via ?secret=… query param.
func TestHandlerAcceptsQuerySecret(t *testing.T) {
	s := newNezhaTestStack(t)
	_, secret := s.seedAgent(t, "nezha_compat")

	body := validBody("not-the-real-secret")
	target := "/api/v1/nezha/heartbeat?secret=" + url.QueryEscape(secret)
	rec := s.post(target, nil, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for query secret, got %d body=%s", rec.Code, rec.Body.String())
	}
	if s.adapter.callCount() != 1 {
		t.Fatalf("expected adapter invoked once via query path")
	}
}

// TestHandlerReturns404OnUnknownSecret — silent 404 per ADR 0006.
func TestHandlerReturns404OnUnknownSecret(t *testing.T) {
	s := newNezhaTestStack(t)
	_, _ = s.seedAgent(t, "nezha_compat") // ensures the repo is non-empty

	body := validBody("definitely-not-issued")
	rec := s.post("/api/v1/nezha/heartbeat", nil, body)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected silent 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Server"); !strings.HasPrefix(got, "nginx") {
		t.Fatalf("expected nginx-style 404 header, got Server=%q", got)
	}
	if s.adapter.callCount() != 0 {
		t.Fatalf("adapter should not be invoked on auth failure, got %d calls", s.adapter.callCount())
	}
}

// TestHandlerReturns404OnMissingSecret — no header, no query, no body field.
func TestHandlerReturns404OnMissingSecret(t *testing.T) {
	s := newNezhaTestStack(t)
	_, _ = s.seedAgent(t, "nezha_compat")

	body := map[string]any{"state": map[string]any{"cpu": 1.0}}
	rec := s.post("/api/v1/nezha/heartbeat", nil, body)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected silent 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandlerReturns404OnWrongKind — secret matches a native agent → reject.
func TestHandlerReturns404OnWrongKind(t *testing.T) {
	s := newNezhaTestStack(t)
	_, secret := s.seedAgent(t, "native")

	body := validBody(secret)
	rec := s.post("/api/v1/nezha/heartbeat", nil, body)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected silent 404 for native-kind agent, got %d body=%s",
			rec.Code, rec.Body.String())
	}
}

// TestHandlerReturns404OnNonPost — GET / PUT etc. must look like a 404 too.
func TestHandlerReturns404OnNonPost(t *testing.T) {
	s := newNezhaTestStack(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nezha/heartbeat", nil)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected silent 404 for GET, got %d", rec.Code)
	}
}

// TestHandlerAcceptsPartialPayloadStillReturns200 — fields-not-fully-populated
// requests are admitted (zero-fill semantics) so a misbehaving Nezha agent
// does not retry-storm against the hub.
func TestHandlerAcceptsPartialPayloadStillReturns200(t *testing.T) {
	s := newNezhaTestStack(t)
	_, secret := s.seedAgent(t, "nezha_compat")

	body := map[string]any{
		"secret": secret,
		// no state, no host — pure auth ping
	}
	rec := s.post("/api/v1/nezha/heartbeat", nil, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for partial payload, got %d body=%s", rec.Code, rec.Body.String())
	}
	if s.adapter.callCount() != 1 {
		t.Fatalf("expected adapter invoked once for partial payload")
	}
}

// TestHandlerReportAliasRoutes — the /report path must behave identically.
func TestHandlerReportAliasRoutes(t *testing.T) {
	s := newNezhaTestStack(t)
	_, secret := s.seedAgent(t, "nezha_compat")

	rec := s.post("/api/v1/nezha/report", nil, validBody(secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 via /report alias, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandlerReturns500WhenAdapterFails — adapter errors propagate as the
// Nezha-style error envelope with HTTP 500. (Important: still NOT a silent
// 404 — auth succeeded, so the agent operator should see the failure.)
func TestHandlerReturns500WhenAdapterFails(t *testing.T) {
	s := newNezhaTestStack(t)
	_, secret := s.seedAgent(t, "nezha_compat")
	s.adapter.returnErr = errors.New("synthetic adapter failure")

	rec := s.post("/api/v1/nezha/heartbeat", nil, validBody(secret))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on adapter failure, got %d body=%s",
			rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if code, _ := resp["code"].(float64); code != 1 {
		t.Fatalf("expected code=1 in error envelope, got %v", resp["code"])
	}
}

// TestHandlerReturns404OnInvalidJSON — unparseable body looks like an attack.
func TestHandlerReturns404OnInvalidJSON(t *testing.T) {
	s := newNezhaTestStack(t)
	_, _ = s.seedAgent(t, "nezha_compat")

	rec := s.post("/api/v1/nezha/heartbeat", nil, []byte("not-json-at-all"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected silent 404 on bad JSON, got %d body=%s", rec.Code, rec.Body.String())
	}
}
