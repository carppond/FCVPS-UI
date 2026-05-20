package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// newTestStack spins up a fresh on-disk SQLite database (under t.TempDir),
// applies migrations and wires the auth.Manager + collaborators against it.
// The returned cleanup must be invoked via t.Cleanup.
func newTestStack(t *testing.T) (*Manager, *storage.UserRepo, *storage.SessionRepo, *TokenStore) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.DatabaseConfig{
		DataDir:       dir,
		Filename:      "test.db",
		BusyTimeoutMs: 5000,
		MaxOpenWrite:  1,
		MaxOpenRead:   2,
	}
	// touch path so we can rely on it
	_ = filepath.Join(dir, cfg.Filename)
	db, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	users := storage.NewUserRepo(db, time.Now)
	sessions := storage.NewSessionRepo(db, time.Now)
	tokens, err := NewTokenStore(TokenStoreConfig{
		Sessions: sessions,
		Users:    users,
		TTL:      time.Hour,
	})
	if err != nil {
		t.Fatalf("NewTokenStore: %v", err)
	}
	totp := NewTOTPManager(NewStorageUserAdapter(users), time.Now)
	mgr, err := NewManager(ManagerConfig{
		Users:    users,
		Sessions: sessions,
		Tokens:   tokens,
		TOTP:     totp,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr, users, sessions, tokens
}

func TestEnsureAdminBootstrapsOnEmptyDB(t *testing.T) {
	mgr, users, _, _ := newTestStack(t)
	username, plain, err := mgr.EnsureAdmin(context.Background())
	if err != nil {
		t.Fatalf("EnsureAdmin: %v", err)
	}
	if username != "admin" {
		t.Fatalf("expected username admin, got %q", username)
	}
	if len(plain) < 16 {
		t.Fatalf("strong password should be ≥ 16 chars, got %d", len(plain))
	}
	user, err := users.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetByUsername(admin): %v", err)
	}
	if user.Role != string(types.RoleAdmin) {
		t.Fatalf("admin role should be %q", types.RoleAdmin)
	}
	if !VerifyPassword(plain, user.PasswordHash) {
		t.Fatalf("returned plaintext does not verify against stored hash")
	}
}

func TestEnsureAdminIdempotent(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	if _, _, err := mgr.EnsureAdmin(context.Background()); err != nil {
		t.Fatalf("first EnsureAdmin: %v", err)
	}
	u, p, err := mgr.EnsureAdmin(context.Background())
	if err != nil {
		t.Fatalf("second EnsureAdmin: %v", err)
	}
	if u != "" || p != "" {
		t.Fatalf("second EnsureAdmin should be a no-op, got u=%q p_len=%d", u, len(p))
	}
}

func TestLoginSuccessNoTOTP(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	plain := "Hunter2-AAAA"
	hash, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	_, err = mgr.cfg.Users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: hash, Role: "user", IsActive: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	res, err := mgr.Login(context.Background(), "alice", plain, "1.2.3.4", "go-test")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.TOTPRequired {
		t.Fatalf("user has no TOTP — login must not require 2FA")
	}
	if res.AccessToken == "" {
		t.Fatalf("AccessToken should be populated")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	hash, _ := HashPassword("Hunter2-AAAA")
	_, _ = mgr.cfg.Users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: hash, Role: "user", IsActive: true,
	})
	if _, err := mgr.Login(context.Background(), "alice", "wrong", "1.2.3.4", "go-test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownUsernameUsesSameError(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	if _, err := mgr.Login(context.Background(), "nobody", "Hunter2-AAAA", "1.2.3.4", "go-test"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown username must surface ErrInvalidCredentials (not ErrUserNotFound) to defeat enumeration, got %v", err)
	}
}

func TestLoginInactiveUser(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	hash, _ := HashPassword("Hunter2-AAAA")
	_, _ = mgr.cfg.Users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: hash, Role: "user", IsActive: false,
	})
	if _, err := mgr.Login(context.Background(), "alice", "Hunter2-AAAA", "1.2.3.4", "go-test"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("expected ErrAccountDisabled, got %v", err)
	}
}

func TestLoginRequires2FA(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	hash, _ := HashPassword("Hunter2-AAAA")
	_, _ = mgr.cfg.Users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: hash, Role: "user",
		IsActive: true, TOTPEnabled: true, TOTPSecret: "JBSWY3DPEHPK3PXP",
	})
	res, err := mgr.Login(context.Background(), "alice", "Hunter2-AAAA", "1.2.3.4", "go-test")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if !res.TOTPRequired {
		t.Fatalf("user has TOTP — login must request 2FA")
	}
	if res.PendingToken == "" {
		t.Fatalf("PendingToken should be populated")
	}
	if res.AccessToken != "" {
		t.Fatalf("AccessToken should be empty for pending 2FA login")
	}
}

func TestChangePasswordRevokesSessions(t *testing.T) {
	mgr, _, sessions, tokens := newTestStack(t)
	old := "Hunter2-AAAA"
	hash, _ := HashPassword(old)
	_, _ = mgr.cfg.Users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: hash, Role: "user", IsActive: true,
	})
	if _, _, err := tokens.Issue(context.Background(), "u1", "1.2.3.4", "go-test", false); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if rows, _ := sessions.ListByUser(context.Background(), "u1"); len(rows) != 1 {
		t.Fatalf("precondition: expected one session, got %d", len(rows))
	}
	if err := mgr.ChangePassword(context.Background(), "u1", old, "Hunter3-BBBB"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if rows, _ := sessions.ListByUser(context.Background(), "u1"); len(rows) != 0 {
		t.Fatalf("ChangePassword should have revoked sessions; %d remain", len(rows))
	}
}

func TestCreateUserGeneratesPasswordWhenEmpty(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	rec, plain, err := mgr.CreateUser(context.Background(), types.CreateUserRequest{
		Username: "bob",
		Role:     types.RoleUser,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if rec.Username != "bob" {
		t.Fatalf("username not persisted")
	}
	if len(plain) < 16 {
		t.Fatalf("auto-generated password should be strong, got len=%d", len(plain))
	}
	if !VerifyPassword(plain, rec.PasswordHash) {
		t.Fatalf("returned plaintext does not verify against hash")
	}
}

func TestCreateUserRejectsWeakPassword(t *testing.T) {
	mgr, _, _, _ := newTestStack(t)
	_, _, err := mgr.CreateUser(context.Background(), types.CreateUserRequest{
		Username: "bob",
		Role:     types.RoleUser,
		Password: "abc",
	})
	if !errors.Is(err, ErrPasswordTooWeak) {
		t.Fatalf("expected ErrPasswordTooWeak, got %v", err)
	}
}

func TestResetPasswordReturnsStrongPlaintextAndRevokes(t *testing.T) {
	mgr, users, sessions, tokens := newTestStack(t)
	hash, _ := HashPassword("Hunter2-AAAA")
	_, _ = users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: hash, Role: "user", IsActive: true,
	})
	if _, _, err := tokens.Issue(context.Background(), "u1", "1.2.3.4", "go-test", false); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	plain, err := mgr.ResetPassword(context.Background(), "u1")
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if len(plain) < 16 {
		t.Fatalf("strong password expected, got len %d", len(plain))
	}
	got, _ := users.GetByID(context.Background(), "u1")
	if !VerifyPassword(plain, got.PasswordHash) {
		t.Fatalf("new plaintext should verify against stored hash")
	}
	rows, _ := sessions.ListByUser(context.Background(), "u1")
	if len(rows) != 0 {
		t.Fatalf("ResetPassword should have revoked sessions; %d remain", len(rows))
	}
}

func TestPromoteFromPendingTokenIssuesFreshAccessToken(t *testing.T) {
	mgr, _, _, tokens := newTestStack(t)
	hash, _ := HashPassword("Hunter2-AAAA")
	_, _ = mgr.cfg.Users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "alice", PasswordHash: hash, Role: "user",
		IsActive: true, TOTPEnabled: true, TOTPSecret: "JBSWY3DPEHPK3PXP",
	})
	pending, _, err := tokens.Issue(context.Background(), "u1", "", "", true)
	if err != nil {
		t.Fatalf("Issue pending: %v", err)
	}
	access, _, _, err := tokens.PromoteFromPending(context.Background(), pending)
	if err != nil {
		t.Fatalf("PromoteFromPending: %v", err)
	}
	if access == pending || access == "" {
		t.Fatalf("PromoteFromPending must return a fresh non-empty token")
	}
	// The old pending token should now be unknown.
	if _, err := tokens.Lookup(context.Background(), pending); err == nil {
		t.Fatalf("pending token should be invalid after promotion")
	}
}
