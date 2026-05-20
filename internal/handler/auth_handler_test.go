package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
)

// authTestStack wires the full auth + handler chain against a tmp-dir SQLite
// database. Returned helpers let individual tests register users with arbitrary
// password / TOTP state and then exercise the HTTP handler directly.
type authTestStack struct {
	t        *testing.T
	mux      http.Handler
	users    *storage.UserRepo
	sessions *storage.SessionRepo
	mgr      *auth.Manager
	tokens   *auth.TokenStore
	totp     auth.TOTPManager
}

func newAuthTestStack(t *testing.T) *authTestStack {
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
	deps := &Deps{
		DB:             db,
		AuthManager:    mgr,
		TokenStore:     tokens,
		UserRepo:       users,
		SessionRepo:    sessions,
		TOTPManager:    totpMgr,
		LoginRateLimit: ratelimit.New(5.0/3600.0, 5, 0),
	}
	mux := NewRouter(deps)
	return &authTestStack{
		t:        t,
		mux:      mux,
		users:    users,
		sessions: sessions,
		mgr:      mgr,
		tokens:   tokens,
		totp:     totpMgr,
	}
}

// createUser provisions a user row with the supplied plaintext password and
// returns the inserted UserRecord.
func (s *authTestStack) createUser(username, password string, role types.UserRole, totpSecret string) *storage.UserRecord {
	hash, err := auth.HashPassword(password)
	if err != nil {
		s.t.Fatalf("HashPassword: %v", err)
	}
	rec := storage.UserRecord{
		ID: username + "-id", Username: username, PasswordHash: hash,
		Role: string(role), IsActive: true,
	}
	if totpSecret != "" {
		rec.TOTPSecret = totpSecret
		rec.TOTPEnabled = true
	}
	out, err := s.users.Create(context.Background(), rec)
	if err != nil {
		s.t.Fatalf("create user: %v", err)
	}
	return out
}

// do issues a request against the mounted router. Body may be nil; bearer may
// be empty.
func (s *authTestStack) do(method, target string, body any, bearer string) *httptest.ResponseRecorder {
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

// envelope is the minimal projection of types.APIResponse used by tests that
// only need to inspect code + data fields.
type envelope[T any] struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	RequestID string `json:"request_id"`
}

func TestLoginSuccessReturnsAccessToken(t *testing.T) {
	s := newAuthTestStack(t)
	s.createUser("alice", "Hunter2-AAAA", types.RoleUser, "")
	rec := s.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice", "password": "Hunter2-AAAA",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env envelope[types.LoginResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if env.Data.AccessToken == "" {
		t.Fatalf("expected access_token populated, body=%s", rec.Body.String())
	}
}

func TestLoginWith2FAReturnsPendingToken(t *testing.T) {
	s := newAuthTestStack(t)
	s.createUser("alice", "Hunter2-AAAA", types.RoleUser, "JBSWY3DPEHPK3PXP")
	rec := s.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice", "password": "Hunter2-AAAA",
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even for TOTP-required (§1.2 裁决), got %d body=%s", rec.Code, rec.Body.String())
	}
	var env envelope[types.PendingTOTPResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !env.Data.TOTPRequired || env.Data.PendingToken == "" {
		t.Fatalf("expected pending TOTP response, got %+v", env.Data)
	}
}

func TestLoginInvalidPasswordReturns401(t *testing.T) {
	s := newAuthTestStack(t)
	s.createUser("alice", "Hunter2-AAAA", types.RoleUser, "")
	rec := s.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice", "password": "wrong",
	}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env envelope[any]
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != string(types.ErrAuthInvalidPassword) {
		t.Fatalf("expected code %s, got %s", types.ErrAuthInvalidPassword, env.Code)
	}
}

func TestLoginRateLimiterAfterBurst(t *testing.T) {
	s := newAuthTestStack(t)
	s.createUser("alice", "Hunter2-AAAA", types.RoleUser, "")
	body := map[string]string{"username": "alice", "password": "wrong"}
	// Burst capacity is 5; the 6th attempt must surface ErrAuthRateLimited.
	for i := 0; i < 5; i++ {
		rec := s.do(http.MethodPost, "/api/auth/login", body, "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d unexpected status %d", i+1, rec.Code)
		}
	}
	rec := s.do(http.MethodPost, "/api/auth/login", body, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt should be 429, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVerifyTOTPSuccess(t *testing.T) {
	s := newAuthTestStack(t)
	const secret = "JBSWY3DPEHPK3PXP"
	s.createUser("alice", "Hunter2-AAAA", types.RoleUser, secret)
	// Login to grab the pending token.
	rec := s.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice", "password": "Hunter2-AAAA",
	}, "")
	var loginEnv envelope[types.PendingTOTPResponse]
	_ = json.Unmarshal(rec.Body.Bytes(), &loginEnv)

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	verifyRec := s.do(http.MethodPost, "/api/auth/verify-totp", map[string]string{
		"pending_token": loginEnv.Data.PendingToken,
		"code":          code,
	}, "")
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	var env envelope[types.LoginResponse]
	if err := json.Unmarshal(verifyRec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if env.Data.AccessToken == "" {
		t.Fatalf("expected access_token after verify, body=%s", verifyRec.Body.String())
	}
}

func TestVerifyTOTPWrongCode(t *testing.T) {
	s := newAuthTestStack(t)
	const secret = "JBSWY3DPEHPK3PXP"
	s.createUser("alice", "Hunter2-AAAA", types.RoleUser, secret)
	rec := s.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice", "password": "Hunter2-AAAA",
	}, "")
	var loginEnv envelope[types.PendingTOTPResponse]
	_ = json.Unmarshal(rec.Body.Bytes(), &loginEnv)

	bad := s.do(http.MethodPost, "/api/auth/verify-totp", map[string]string{
		"pending_token": loginEnv.Data.PendingToken,
		"code":          "000000",
	}, "")
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong TOTP, got %d body=%s", bad.Code, bad.Body.String())
	}
	var env envelope[any]
	_ = json.Unmarshal(bad.Body.Bytes(), &env)
	if env.Code != string(types.ErrAuthTOTPInvalid) {
		t.Fatalf("expected %s, got %s", types.ErrAuthTOTPInvalid, env.Code)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	s := newAuthTestStack(t)
	s.createUser("alice", "Hunter2-AAAA", types.RoleUser, "")
	loginRec := s.do(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice", "password": "Hunter2-AAAA",
	}, "")
	var env envelope[types.LoginResponse]
	_ = json.Unmarshal(loginRec.Body.Bytes(), &env)
	tok := env.Data.AccessToken

	// Logout must succeed.
	out := s.do(http.MethodPost, "/api/auth/logout", nil, tok)
	if out.Code != http.StatusOK {
		t.Fatalf("logout status = %d body=%s", out.Code, out.Body.String())
	}
	// Subsequent /api/me must return 401.
	me := s.do(http.MethodGet, "/api/me", nil, tok)
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d body=%s", me.Code, me.Body.String())
	}
}
