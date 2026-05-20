package substore_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/types"
)

// mockNodeRepo captures UpsertBatch invocations so individual tests can assert
// on the parsed node set without standing up the real node repo (T-11).
type mockNodeRepo struct {
	mu      sync.Mutex
	upserts []upsertCall
	render  []*substore.ParsedNode
	err     error
}

type upsertCall struct {
	SubscriptionID string
	Nodes          []substore.NodeUpsertInput
}

func (m *mockNodeRepo) UpsertBatch(ctx context.Context, subID string, nodes []substore.NodeUpsertInput) (substore.UpsertResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return substore.UpsertResult{}, m.err
	}
	m.upserts = append(m.upserts, upsertCall{SubscriptionID: subID, Nodes: nodes})
	return substore.UpsertResult{Total: len(nodes), Added: len(nodes)}, nil
}

func (m *mockNodeRepo) ListForRender(ctx context.Context, subID string) ([]*substore.ParsedNode, error) {
	return m.render, nil
}

// newTestSyncStack stands up an in-memory subscription repo + mock node repo
// + sync service. The fake HTTP server lets each test supply its own response
// body (YAML / base64 / URI list).
type syncStack struct {
	t            *testing.T
	repo         *storage.SubscriptionRepo
	nodeRepo     *mockNodeRepo
	sync         *substore.SyncService
	httpServer   *httptest.Server
	httpBody     string
	httpHeader   http.Header
	httpReqCount int
}

func newSyncStack(t *testing.T) *syncStack {
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
	// Seed a user so the FK on subscriptions(user_id) is happy.
	users := storage.NewUserRepo(db, time.Now)
	_, err = users.Create(context.Background(), storage.UserRecord{
		ID: "u-sync-test", Username: "user", PasswordHash: "x",
		Role: string(types.RoleUser), IsActive: true,
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	repo := storage.NewSubscriptionRepo(db, time.Now)
	nodeRepo := &mockNodeRepo{}

	stack := &syncStack{t: t, repo: repo, nodeRepo: nodeRepo}
	stack.httpServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stack.httpHeader = r.Header.Clone()
		stack.httpReqCount++
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(stack.httpBody))
	}))
	t.Cleanup(stack.httpServer.Close)

	syncSvc, err := substore.NewSyncService(substore.SyncServiceConfig{
		Repo: repo, NodeRepo: nodeRepo,
		HTTPClient: stack.httpServer.Client(),
	})
	if err != nil {
		t.Fatalf("NewSyncService: %v", err)
	}
	stack.sync = syncSvc
	return stack
}

func TestSyncServiceFetchURIList(t *testing.T) {
	stack := newSyncStack(t)
	uri := "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:test")) +
		"@1.2.3.4:8388#node1"
	stack.httpBody = uri + "\n"

	created, err := stack.repo.Create(context.Background(), storage.SubscriptionRecord{
		ID:        "sub-uri",
		UserID:    "u-sync-test",
		Name:      "uri-list",
		Type:      string(types.SubTypeURL),
		SourceURL: stack.httpServer.URL,
		UA:        "test-ua",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	result, err := stack.sync.SyncOne(context.Background(), created)
	if err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if result.NodeCount != 1 {
		t.Fatalf("expected NodeCount=1, got %d", result.NodeCount)
	}
	if stack.httpHeader.Get("User-Agent") != "test-ua" {
		t.Fatalf("expected custom UA, got %q", stack.httpHeader.Get("User-Agent"))
	}
	if len(stack.nodeRepo.upserts) != 1 {
		t.Fatalf("expected 1 UpsertBatch call, got %d", len(stack.nodeRepo.upserts))
	}
	if stack.nodeRepo.upserts[0].Nodes[0].Protocol != "ss" {
		t.Fatalf("expected ss protocol, got %s", stack.nodeRepo.upserts[0].Nodes[0].Protocol)
	}

	// State persisted as OK.
	got, _ := stack.repo.GetByID(context.Background(), "sub-uri", "u-sync-test")
	if got.LastSyncStatus != string(types.SyncStatusOK) {
		t.Fatalf("expected status=ok, got %q", got.LastSyncStatus)
	}
}

func TestSyncServiceFetchBase64(t *testing.T) {
	stack := newSyncStack(t)
	uri := "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:test")) +
		"@1.2.3.4:8388#node1\n" + "ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:test")) +
		"@5.6.7.8:443#node2"
	stack.httpBody = base64.StdEncoding.EncodeToString([]byte(uri))

	created, err := stack.repo.Create(context.Background(), storage.SubscriptionRecord{
		ID:        "sub-b64",
		UserID:    "u-sync-test",
		Name:      "b64",
		Type:      string(types.SubTypeURL),
		SourceURL: stack.httpServer.URL,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	result, err := stack.sync.SyncOne(context.Background(), created)
	if err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if result.NodeCount != 2 {
		t.Fatalf("expected NodeCount=2, got %d", result.NodeCount)
	}
}

func TestSyncServiceFetchYAML(t *testing.T) {
	stack := newSyncStack(t)
	stack.httpBody = "proxies:\n  - name: ny\n    type: vmess\n    server: 1.2.3.4\n    port: 443\n    uuid: abcd-1234\n"

	created, err := stack.repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-yaml", UserID: "u-sync-test", Name: "yaml",
		Type: string(types.SubTypeURL), SourceURL: stack.httpServer.URL,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	result, err := stack.sync.SyncOne(context.Background(), created)
	if err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if result.NodeCount != 1 {
		t.Fatalf("expected NodeCount=1, got %d", result.NodeCount)
	}
	if stack.nodeRepo.upserts[0].Nodes[0].Server != "1.2.3.4" {
		t.Fatalf("expected server=1.2.3.4, got %s", stack.nodeRepo.upserts[0].Nodes[0].Server)
	}
}

func TestSyncServiceHTTPErrorIsRecorded(t *testing.T) {
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
		t.Fatalf("Migrate: %v", err)
	}
	users := storage.NewUserRepo(db, time.Now)
	_, _ = users.Create(context.Background(), storage.UserRecord{
		ID: "u-err", Username: "err", PasswordHash: "x",
		Role: string(types.RoleUser), IsActive: true,
	})
	repo := storage.NewSubscriptionRepo(db, time.Now)
	nodeRepo := &mockNodeRepo{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	svc, err := substore.NewSyncService(substore.SyncServiceConfig{
		Repo: repo, NodeRepo: nodeRepo, HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewSyncService: %v", err)
	}
	created, _ := repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-err", UserID: "u-err", Name: "errsub",
		Type: string(types.SubTypeURL), SourceURL: srv.URL,
	})
	_, err = svc.SyncOne(context.Background(), created)
	if err == nil {
		t.Fatalf("expected sync error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected http status error, got %v", err)
	}
	got, _ := repo.GetByID(context.Background(), "sub-err", "u-err")
	if got.LastSyncStatus != string(types.SyncStatusError) {
		t.Fatalf("expected status=error, got %q", got.LastSyncStatus)
	}
	if got.LastSyncError == "" {
		t.Fatalf("expected LastSyncError to be populated")
	}
}

func TestSyncServiceManualTypeRejected(t *testing.T) {
	stack := newSyncStack(t)
	created, _ := stack.repo.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-manual", UserID: "u-sync-test", Name: "man",
		Type: string(types.SubTypeManual),
	})
	_, err := stack.sync.SyncOne(context.Background(), created)
	if err == nil {
		t.Fatalf("expected manual type to be rejected for direct sync")
	}
}
