package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
)

// sshWSStack wires the SSH relay handler against a real SQLite + token store.
type sshWSStack struct {
	srv    *httptest.Server
	tokens *auth.TokenStore
	users  *storage.UserRepo
	assets *storage.VpsAssetRepo
}

func newSSHWSStack(t *testing.T) *sshWSStack {
	t.Helper()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: t.TempDir(), Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	users := storage.NewUserRepo(db, time.Now)
	sessions := storage.NewSessionRepo(db, time.Now)
	tokens, err := auth.NewTokenStore(auth.TokenStoreConfig{
		Sessions: sessions, Users: users, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("token store: %v", err)
	}
	assets := storage.NewVpsAssetRepo(db, time.Now)

	mux := http.NewServeMux()
	h := NewSSHWSHandler(tokens, assets, nil)
	mux.Handle("GET /api/vps-assets/{id}/ssh", http.HandlerFunc(h.ServeHTTP))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &sshWSStack{srv: srv, tokens: tokens, users: users, assets: assets}
}

func (s *sshWSStack) seedUserToken(t *testing.T, id, name string) string {
	t.Helper()
	if _, err := s.users.Create(context.Background(), storage.UserRecord{
		ID: id, Username: name, PasswordHash: "h", Role: "user", IsActive: true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	token, _, err := s.tokens.Issue(context.Background(), id, "127.0.0.1", "test", false)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

func (s *sshWSStack) seedAsset(t *testing.T, userID string, sshUser string) string {
	t.Helper()
	rec, err := s.assets.Create(context.Background(), storage.VpsAssetRecord{
		ID: "asset-" + userID + "-" + sshUser, UserID: userID, Name: "box",
		Provider: "test", ExpireAt: "2030-01-01", BillingCycle: "monthly",
		IP: "127.0.0.1", SSHPort: 1, SSHUser: sshUser, SSHPassword: "pw",
	})
	if err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return rec.ID
}

func (s *sshWSStack) wsURL(assetID, token string) string {
	u := strings.Replace(s.srv.URL, "http://", "ws://", 1)
	return u + "/api/vps-assets/" + assetID + "/ssh?token=" + token
}

func TestSSHWSUnauthenticated(t *testing.T) {
	s := newSSHWSStack(t)
	ownerToken := s.seedUserToken(t, "u1", "alice")
	assetID := s.seedAsset(t, "u1", "root")
	_ = ownerToken

	resp, err := http.Get(s.srv.URL + "/api/vps-assets/" + assetID + "/ssh")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: got %d want 401", resp.StatusCode)
	}
}

func TestSSHWSCrossUserForbidden(t *testing.T) {
	s := newSSHWSStack(t)
	_ = s.seedUserToken(t, "u1", "alice")
	assetID := s.seedAsset(t, "u1", "root")
	otherToken := s.seedUserToken(t, "u2", "bob")

	//nolint:noctx // test helper
	resp, err := http.Get(s.srv.URL + "/api/vps-assets/" + assetID + "/ssh?token=" + otherToken)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-user: got %d want 404", resp.StatusCode)
	}
}

func TestSSHWSNotConfigured(t *testing.T) {
	s := newSSHWSStack(t)
	token := s.seedUserToken(t, "u1", "alice")
	assetID := s.seedAsset(t, "u1", "") // no ssh_user

	//nolint:noctx // test helper
	resp, err := http.Get(s.srv.URL + "/api/vps-assets/" + assetID + "/ssh?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unconfigured asset: got %d want 400", resp.StatusCode)
	}
}

func TestSSHWSUpgradeThenDialErrorSurfaced(t *testing.T) {
	s := newSSHWSStack(t)
	token := s.seedUserToken(t, "u1", "alice")
	// Port 1 on loopback refuses immediately → relay dial fails fast and the
	// handler must surface a text-frame error over the upgraded socket.
	assetID := s.seedAsset(t, "u1", "root")

	conn, resp, err := websocket.DefaultDialer.Dial(s.wsURL(assetID, token), nil)
	if err != nil {
		t.Fatalf("ws dial: %v (resp=%v)", err, resp)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	mt, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read error frame: %v", err)
	}
	if mt != websocket.TextMessage {
		t.Fatalf("want text error frame, got type %d", mt)
	}
	if !strings.Contains(string(data), "connection failed") {
		t.Errorf("error frame should mention failure, got %q", string(data))
	}
}
