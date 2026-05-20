package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/substore"
	"shiguang-vps/internal/types"
)

func newCompatTestStack(t *testing.T) (*storage.SubscriptionRepo, http.Handler) {
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
	_, _ = users.Create(context.Background(), storage.UserRecord{
		ID: "u-compat", Username: "u", PasswordHash: "x",
		Role: string(types.RoleUser), IsActive: true,
	})
	subs := storage.NewSubscriptionRepo(db, time.Now)
	syncSvc, err := substore.NewSyncService(substore.SyncServiceConfig{
		Repo: subs, NodeRepo: substore.NoopNodeRepo{},
	})
	if err != nil {
		t.Fatalf("NewSyncService: %v", err)
	}
	compatSvc, err := substore.NewSubstoreCompatService(substore.SubstoreCompatConfig{
		Repo: subs, NodeRepo: substore.NoopNodeRepo{}, Sync: syncSvc,
	})
	if err != nil {
		t.Fatalf("NewSubstoreCompatService: %v", err)
	}
	deps := &Deps{
		DB:                    db,
		SubstoreCompatHandler: NewSubstoreCompatHandler(compatSvc, nil),
	}
	mux := NewRouter(deps)
	return subs, mux
}

func TestSubstoreCompatHappyPath(t *testing.T) {
	subs, mux := newCompatTestStack(t)
	created, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-compat-1", UserID: "u-compat", Name: "myname",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/download/myname?token="+created.ShareToken, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Total-Nodes") == "" {
		t.Fatalf("expected X-Total-Nodes header")
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/yaml") {
		t.Fatalf("expected text/yaml content type, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestSubstoreCompatWrongTokenReturns404(t *testing.T) {
	subs, mux := newCompatTestStack(t)
	_, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-compat-2", UserID: "u-compat", Name: "myname",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/download/myname?token=wrongtoken", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on bad token, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubstoreCompatMismatchedNameReturns404(t *testing.T) {
	subs, mux := newCompatTestStack(t)
	created, err := subs.Create(context.Background(), storage.SubscriptionRecord{
		ID: "sub-compat-3", UserID: "u-compat", Name: "real-name",
		Type: string(types.SubTypeManual),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet,
		"/download/wrong-name?token="+created.ShareToken, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when name does not match, got %d", rec.Code)
	}
}

func TestSubstoreCompatMissingTokenReturns404(t *testing.T) {
	_, mux := newCompatTestStack(t)
	req := httptest.NewRequest(http.MethodGet, "/download/anything", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on missing token, got %d", rec.Code)
	}
}

