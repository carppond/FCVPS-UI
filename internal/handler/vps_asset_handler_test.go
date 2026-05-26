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
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// vpsTestStack wires the VPS asset handler against a tmp-dir SQLite DB.
type vpsTestStack struct {
	t      *testing.T
	mux    http.Handler
	users  *storage.UserRepo
	repo   *storage.VpsAssetRepo
	tokens *auth.TokenStore
}

func newVpsTestStack(t *testing.T) *vpsTestStack {
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
	repo := storage.NewVpsAssetRepo(db, time.Now)
	vpsHandler := NewVpsAssetHandler(repo, nil)
	deps := &Deps{
		DB:              db,
		AuthManager:     mgr,
		TokenStore:      tokens,
		UserRepo:        users,
		SessionRepo:     sessions,
		TOTPManager:     totpMgr,
		VpsAssetHandler: vpsHandler,
	}
	mux := NewRouter(deps)
	return &vpsTestStack{
		t: t, mux: mux, users: users, repo: repo, tokens: tokens,
	}
}

func (s *vpsTestStack) createUserAndToken(username, role string) (string, string) {
	s.t.Helper()
	hash, err := auth.HashPassword("Hunter2-AAAA")
	if err != nil {
		s.t.Fatalf("hash: %v", err)
	}
	rec := storage.UserRecord{
		ID: username + "-id", Username: username, PasswordHash: hash,
		Role: role, IsActive: true,
	}
	if _, err := s.users.Create(context.Background(), rec); err != nil {
		s.t.Fatalf("create user: %v", err)
	}
	tok, _, err := s.tokens.Issue(context.Background(), rec.ID, "127.0.0.1", "test", false)
	if err != nil {
		s.t.Fatalf("issue token: %v", err)
	}
	return rec.ID, tok
}

func TestVpsAssetCRUD(t *testing.T) {
	s := newVpsTestStack(t)
	_, token := s.createUserAndToken("alice", "admin")

	// POST /api/vps-assets — create
	body := types.CreateVpsAssetRequest{
		Name:         "hk-vps-01",
		Provider:     "搬瓦工",
		Price:        49.99,
		Currency:     "USD",
		BillingCycle: types.BillingAnnual,
		ExpireAt:     time.Now().AddDate(0, 6, 0).Format("2006-01-02"),
		IP:           "1.2.3.4",
		Tags:         []string{"prod"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/vps-assets", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST create: want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var createResp types.APIResponse[types.VpsAsset]
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	assetID := createResp.Data.ID
	if assetID == "" {
		t.Fatalf("expected non-empty ID")
	}
	if createResp.Data.Status != "normal" {
		t.Fatalf("expected status=normal, got %q", createResp.Data.Status)
	}

	// GET /api/vps-assets/{id}
	req = httptest.NewRequest("GET", "/api/vps-assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// GET /api/vps-assets — list
	req = httptest.NewRequest("GET", "/api/vps-assets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("LIST: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var listResp types.APIResponse[types.PagedResponse[types.VpsAsset]]
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Data.Total != 1 {
		t.Fatalf("list total: want 1, got %d", listResp.Data.Total)
	}

	// PUT /api/vps-assets/{id} — update
	updateBody := `{"name":"hk-vps-02","price":99.99}`
	req = httptest.NewRequest("PUT", "/api/vps-assets/"+assetID, bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT update: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var updateResp types.APIResponse[types.VpsAsset]
	if err := json.Unmarshal(rec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("unmarshal update: %v", err)
	}
	if updateResp.Data.Name != "hk-vps-02" {
		t.Fatalf("updated name: want hk-vps-02, got %q", updateResp.Data.Name)
	}

	// GET /api/vps-assets/summary
	req = httptest.NewRequest("GET", "/api/vps-assets/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("SUMMARY: want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var sumResp types.APIResponse[types.VpsAssetSummary]
	if err := json.Unmarshal(rec.Body.Bytes(), &sumResp); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if sumResp.Data.Total != 1 {
		t.Fatalf("summary total: want 1, got %d", sumResp.Data.Total)
	}

	// DELETE /api/vps-assets/{id}
	req = httptest.NewRequest("DELETE", "/api/vps-assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE: want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify 404 after deletion.
	req = httptest.NewRequest("GET", "/api/vps-assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET after delete: want 404, got %d", rec.Code)
	}
}

func TestVpsAssetUnauthorized(t *testing.T) {
	s := newVpsTestStack(t)
	req := httptest.NewRequest("GET", "/api/vps-assets", nil)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestVpsAssetCrossUserIsolation(t *testing.T) {
	s := newVpsTestStack(t)
	_, tokenAlice := s.createUserAndToken("alice2", "admin")
	_, tokenBob := s.createUserAndToken("bob2", "user")

	// Alice creates an asset.
	body := types.CreateVpsAssetRequest{
		Name:         "alice-vps",
		Provider:     "p",
		Price:        10,
		BillingCycle: types.BillingMonthly,
		ExpireAt:     "2099-01-01",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/vps-assets", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+tokenAlice)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST: want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp types.APIResponse[types.VpsAsset]
	json.Unmarshal(rec.Body.Bytes(), &resp)
	assetID := resp.Data.ID

	// Bob cannot see Alice's asset.
	req = httptest.NewRequest("GET", "/api/vps-assets/"+assetID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenBob)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("Bob GET: want 404, got %d", rec.Code)
	}

	// Bob's list is empty.
	req = httptest.NewRequest("GET", "/api/vps-assets", nil)
	req.Header.Set("Authorization", "Bearer "+tokenBob)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	var listResp types.APIResponse[types.PagedResponse[types.VpsAsset]]
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	if listResp.Data.Total != 0 {
		t.Fatalf("Bob list total: want 0, got %d", listResp.Data.Total)
	}
}
