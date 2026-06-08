package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// PendingTokenTTL is how long a pending_2fa session stays valid before the
// user must restart from /api/auth/login. 5 minutes matches the contract's
// PendingTOTPResponse.expires_in default.
const PendingTokenTTL = 5 * time.Minute

// ManagerConfig wires the Manager to its collaborators.
type ManagerConfig struct {
	Users    *storage.UserRepo
	Sessions *storage.SessionRepo
	Tokens   *TokenStore
	TOTP     TOTPManager
	Brute    *BruteProtector
	Logger   *slog.Logger
	Now      func() time.Time
}

// Manager is the M-USER service entry point used by the HTTP layer.
type Manager struct {
	cfg ManagerConfig
}

// NewManager validates the dependencies and returns a ready-to-use Manager.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.Users == nil || cfg.Sessions == nil || cfg.Tokens == nil {
		return nil, fmt.Errorf("auth manager: users/sessions/tokens required")
	}
	if cfg.TOTP == nil {
		return nil, fmt.Errorf("auth manager: TOTP required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Manager{cfg: cfg}, nil
}

// LoginResult bundles the outcome of a Login / PromoteFromPending call. Only
// one of AccessToken / PendingToken is populated.
type LoginResult struct {
	User         *storage.UserRecord
	AccessToken  string
	PendingToken string
	ExpiresAt    time.Time
	TOTPRequired bool
}

// Login verifies username + password and either returns a full access token
// or a pending_2fa token. The caller passes ip/userAgent for the session row;
// pass empty strings when those values are not available.
func (m *Manager) Login(ctx context.Context, username, password, ip, userAgent string) (*LoginResult, error) {
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	if m.cfg.Brute != nil {
		if blocked, _ := m.cfg.Brute.IsBlocked(ip, username); blocked {
			return nil, ErrBruteForceBlocked
		}
	}
	user, err := m.cfg.Users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			m.recordFailure(ip, username)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("login lookup: %w", err)
	}
	if !VerifyPassword(password, user.PasswordHash) {
		m.recordFailure(ip, username)
		return nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, ErrAccountDisabled
	}
	if user.TOTPEnabled {
		token, exp, err := m.cfg.Tokens.Issue(ctx, user.ID, ip, userAgent, true)
		if err != nil {
			return nil, err
		}
		return &LoginResult{
			User:         user,
			PendingToken: token,
			ExpiresAt:    exp,
			TOTPRequired: true,
		}, nil
	}
	// No 2FA — full access immediately.
	token, exp, err := m.cfg.Tokens.Issue(ctx, user.ID, ip, userAgent, false)
	if err != nil {
		return nil, err
	}
	if m.cfg.Brute != nil {
		m.cfg.Brute.RecordSuccess(ip, username)
	}
	return &LoginResult{User: user, AccessToken: token, ExpiresAt: exp}, nil
}

// VerifyTOTP checks the six-digit code against the user identified by the
// pending token and, on success, promotes the pending session to a full one.
func (m *Manager) VerifyTOTP(ctx context.Context, pendingToken, code, ip string) (*LoginResult, error) {
	lookup, err := m.cfg.Tokens.LookupPending(ctx, pendingToken)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrPendingTokenInvalid
		}
		return nil, err
	}
	// Brute-force guard on the 2FA step (userID axis survives IP spoofing):
	// without it an attacker holding a pending token could brute-force the
	// 6-digit code within the pending TTL.
	if m.cfg.Brute != nil {
		if blocked, _ := m.cfg.Brute.IsBlocked(ip, lookup.User.ID); blocked {
			return nil, ErrBruteForceBlocked
		}
	}
	if err := m.cfg.TOTP.Verify(ctx, lookup.User.ID, code); err != nil {
		if m.cfg.Brute != nil {
			m.cfg.Brute.RecordFailure(ip, lookup.User.ID)
		}
		return nil, err
	}
	access, exp, user, err := m.cfg.Tokens.PromoteFromPending(ctx, pendingToken)
	if err != nil {
		return nil, err
	}
	if m.cfg.Brute != nil {
		m.cfg.Brute.RecordSuccess(lookup.User.ID, user.Username)
	}
	return &LoginResult{User: user, AccessToken: access, ExpiresAt: exp}, nil
}

// VerifyRecovery burns one recovery code instead of a TOTP code. On success
// the pending session is promoted exactly as for VerifyTOTP.
func (m *Manager) VerifyRecovery(ctx context.Context, pendingToken, code, ip string) (*LoginResult, int, error) {
	lookup, err := m.cfg.Tokens.LookupPending(ctx, pendingToken)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, 0, ErrPendingTokenInvalid
		}
		return nil, 0, err
	}
	// Same brute-force guard as VerifyTOTP — recovery codes are low-entropy
	// and must not be brute-forceable.
	if m.cfg.Brute != nil {
		if blocked, _ := m.cfg.Brute.IsBlocked(ip, lookup.User.ID); blocked {
			return nil, 0, ErrBruteForceBlocked
		}
	}
	remaining, err := VerifyAndConsumeRecoveryCode(ctx, m.cfg.Users, lookup.User.ID, code)
	if err != nil {
		if m.cfg.Brute != nil {
			m.cfg.Brute.RecordFailure(ip, lookup.User.ID)
		}
		return nil, remaining, err
	}
	access, exp, user, err := m.cfg.Tokens.PromoteFromPending(ctx, pendingToken)
	if err != nil {
		return nil, remaining, err
	}
	if m.cfg.Brute != nil {
		m.cfg.Brute.RecordSuccess(lookup.User.ID, user.Username)
	}
	return &LoginResult{User: user, AccessToken: access, ExpiresAt: exp}, remaining, nil
}

// Logout revokes the supplied access token. ErrSessionNotFound is returned
// when the token has already expired or never existed.
func (m *Manager) Logout(ctx context.Context, accessToken string) error {
	return m.cfg.Tokens.Revoke(ctx, accessToken)
}

// ChangePassword swaps the current password for newPassword after verifying
// oldPassword. All other sessions for the user are invalidated; the caller's
// own session is also revoked so the next request must re-login.
func (m *Manager) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := m.cfg.Users.GetByID(ctx, userID)
	if err != nil {
		return mapUserErr(err)
	}
	if !VerifyPassword(oldPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := m.cfg.Users.UpdatePassword(ctx, userID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return m.cfg.Tokens.RevokeAllForUser(ctx, userID)
}

// ResetPassword (admin) overwrites the password without verifying the old
// one. Returns the new plaintext password so the admin can deliver it to
// the user.
func (m *Manager) ResetPassword(ctx context.Context, targetUserID string) (string, error) {
	if _, err := m.cfg.Users.GetByID(ctx, targetUserID); err != nil {
		return "", mapUserErr(err)
	}
	plain := GenerateStrongPassword()
	hash, err := HashPassword(plain)
	if err != nil {
		return "", err
	}
	if err := m.cfg.Users.UpdatePassword(ctx, targetUserID, hash); err != nil {
		return "", fmt.Errorf("reset password: %w", err)
	}
	if err := m.cfg.Tokens.RevokeAllForUser(ctx, targetUserID); err != nil {
		return "", err
	}
	return plain, nil
}

// CreateUser inserts a fresh user (admin only). When req.Password is empty a
// strong password is generated and returned in the second slot.
func (m *Manager) CreateUser(ctx context.Context, req types.CreateUserRequest) (*storage.UserRecord, string, error) {
	role := strings.ToLower(string(req.Role))
	if role != string(types.RoleAdmin) && role != string(types.RoleUser) {
		return nil, "", fmt.Errorf("create user: invalid role %q", req.Role)
	}
	plain := req.Password
	if plain == "" {
		plain = GenerateStrongPassword()
	}
	if err := ValidatePassword(plain); err != nil {
		return nil, "", err
	}
	hash, err := HashPassword(plain)
	if err != nil {
		return nil, "", err
	}
	rec := storage.UserRecord{
		ID:           util.UUIDv7(),
		Username:     req.Username,
		PasswordHash: hash,
		Role:         role,
		IsActive:     true,
		Email:        req.Email,
		Locale:       "zh-CN",
	}
	created, err := m.cfg.Users.Create(ctx, rec)
	if err != nil {
		if errors.Is(err, storage.ErrUsernameTaken) {
			return nil, "", ErrUsernameTaken
		}
		return nil, "", fmt.Errorf("create user: %w", err)
	}
	return created, plain, nil
}

// DeleteUser removes the user. CASCADE deletes pick up everything they own.
func (m *Manager) DeleteUser(ctx context.Context, userID string) error {
	if err := m.cfg.Users.Delete(ctx, userID); err != nil {
		return mapUserErr(err)
	}
	return nil
}

// RegenerateRecoveryCodes burns the existing recovery codes and returns a
// fresh batch. Requires the user's password.
func (m *Manager) RegenerateRecoveryCodes(ctx context.Context, userID, password string) ([]string, error) {
	user, err := m.cfg.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, mapUserErr(err)
	}
	if !user.TOTPEnabled {
		return nil, ErrTOTPNotEnabled
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}
	codes := GenerateRecoveryCodes()
	hashesJSON, err := HashRecoveryCodes(codes)
	if err != nil {
		return nil, err
	}
	if err := m.cfg.Users.UpdateRecoveryCodes(ctx, userID, hashesJSON); err != nil {
		return nil, fmt.Errorf("update recovery codes: %w", err)
	}
	return codes, nil
}

// AdminForceDisable2FA strips TOTP + recovery codes from a user without
// asking for their password. Used by the support-rescue endpoint.
func (m *Manager) AdminForceDisable2FA(ctx context.Context, userID string) error {
	if _, err := m.cfg.Users.GetByID(ctx, userID); err != nil {
		return mapUserErr(err)
	}
	if err := m.cfg.Users.DisableTOTP(ctx, userID); err != nil {
		return fmt.Errorf("disable totp: %w", err)
	}
	return m.cfg.Tokens.RevokeAllForUser(ctx, userID)
}

// AdminRevokeSessions purges every session for userID.
func (m *Manager) AdminRevokeSessions(ctx context.Context, userID string) error {
	if _, err := m.cfg.Users.GetByID(ctx, userID); err != nil {
		return mapUserErr(err)
	}
	return m.cfg.Tokens.RevokeAllForUser(ctx, userID)
}

// EnsureAdmin checks whether at least one admin user exists. If not, it
// creates one with username "admin" and a freshly-minted strong password.
// The returned (username, plaintext) is empty when no work was done.
func (m *Manager) EnsureAdmin(ctx context.Context) (username, plaintext string, err error) {
	count, err := m.cfg.Users.CountAdmins(ctx)
	if err != nil {
		return "", "", err
	}
	if count > 0 {
		return "", "", nil
	}
	username = "admin"
	plaintext = GenerateStrongPassword()
	hash, err := HashPassword(plaintext)
	if err != nil {
		return "", "", err
	}
	_, err = m.cfg.Users.Create(ctx, storage.UserRecord{
		ID:           util.UUIDv7(),
		Username:     username,
		PasswordHash: hash,
		Role:         string(types.RoleAdmin),
		IsActive:     true,
		Locale:       "zh-CN",
	})
	if err != nil {
		return "", "", fmt.Errorf("ensure admin: %w", err)
	}
	if m.cfg.Logger != nil {
		m.cfg.Logger.Warn("auth: bootstrapped initial admin user",
			slog.String("username", username))
	}
	return username, plaintext, nil
}

// recordFailure delegates to the configured BruteProtector when present.
func (m *Manager) recordFailure(ip, username string) {
	if m.cfg.Brute == nil {
		return
	}
	m.cfg.Brute.RecordFailure(ip, username)
}

// mapUserErr converts storage.ErrUserNotFound into ErrUserNotFound; other
// errors pass through.
func mapUserErr(err error) error {
	if errors.Is(err, storage.ErrUserNotFound) {
		return ErrUserNotFound
	}
	return err
}
