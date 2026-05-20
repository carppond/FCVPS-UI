package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
)

// AccessTokenBytes is the number of random bytes encoded into the base64url
// access token returned to the client. 32 bytes → 256 bits of entropy, well
// above what the 1-hour brute-force lockout would tolerate even with infinite
// attempts.
const AccessTokenBytes = 32

// DefaultLookupCacheSize caps the LRU that fronts sessions.GetByTokenHash. The
// number is intentionally small (1000) so memory stays bounded; an attacker
// spraying random tokens can only push out legitimate cache entries, which
// degrade to a DB hit rather than a security issue.
const DefaultLookupCacheSize = 1000

// DefaultLookupCacheTTL bounds how long a positive cache entry remains
// authoritative. 60 s keeps the latency win while still letting Revoke or
// password-change propagate quickly.
const DefaultLookupCacheTTL = 60 * time.Second

// TokenStoreConfig wires TokenStore to its collaborators.
type TokenStoreConfig struct {
	Sessions  *storage.SessionRepo
	Users     *storage.UserRepo
	TTL       time.Duration
	CacheSize int
	CacheTTL  time.Duration
	Now       func() time.Time
}

// TokenStore is the canonical access-token issuer / validator. Tokens are
// generated as base64url(32 random bytes); only sha256(token) is persisted.
type TokenStore struct {
	sessions *storage.SessionRepo
	users    *storage.UserRepo
	ttl      time.Duration
	now      func() time.Time

	cache    *lru.Cache[string, cacheEntry]
	cacheTTL time.Duration
	cacheMu  sync.Mutex
}

// cacheEntry wraps the cached user with its insertion timestamp so we can
// expire stale rows without relying on a TTL cache implementation.
type cacheEntry struct {
	user      *storage.UserRecord
	sessionID string
	expiresAt int64
	insertedAt time.Time
}

// LookupResult bundles the user record + session row produced by Lookup.
type LookupResult struct {
	User      *storage.UserRecord
	SessionID string
	ExpiresAt int64
}

// NewTokenStore returns a ready-to-use store. Sessions / Users must be non-nil.
func NewTokenStore(cfg TokenStoreConfig) (*TokenStore, error) {
	if cfg.Sessions == nil || cfg.Users == nil {
		return nil, fmt.Errorf("token store: sessions and users repos required")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}
	if cfg.CacheSize <= 0 {
		cfg.CacheSize = DefaultLookupCacheSize
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = DefaultLookupCacheTTL
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	cache, err := lru.New[string, cacheEntry](cfg.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("token store cache: %w", err)
	}
	return &TokenStore{
		sessions: cfg.Sessions,
		users:    cfg.Users,
		ttl:      cfg.TTL,
		now:      cfg.Now,
		cache:    cache,
		cacheTTL: cfg.CacheTTL,
	}, nil
}

// Issue mints a new access token for userID and persists the session row. The
// returned plaintext token is the value the client supplies in subsequent
// Authorization headers; only sha256(token) ever touches the DB.
func (s *TokenStore) Issue(ctx context.Context, userID, ip, userAgent string, pending2FA bool) (token string, expiresAt time.Time, err error) {
	if userID == "" {
		return "", time.Time{}, fmt.Errorf("token store issue: empty userID")
	}
	token = util.Base64URL(util.RandomBytes(AccessTokenBytes))
	hash := util.SHA256Hex(token)
	expiresAt = s.now().Add(s.ttl)
	rec := storage.SessionRecord{
		ID:         util.UUIDv7(),
		UserID:     userID,
		TokenHash:  hash,
		Pending2FA: pending2FA,
		ExpiresAt:  expiresAt.UnixMilli(),
		LastUsedAt: s.now().UnixMilli(),
		IP:         ip,
		UserAgent:  userAgent,
		CreatedAt:  s.now().UnixMilli(),
	}
	if err := s.sessions.Create(ctx, rec); err != nil {
		return "", time.Time{}, fmt.Errorf("persist session: %w", err)
	}
	return token, expiresAt, nil
}

// Lookup resolves an access token to its user, applying the LRU cache and
// sliding expiry. Returns ErrSessionNotFound when the token is unknown,
// expired, or its user has been deactivated / deleted.
//
// pending2FA sessions are NOT returned by this method — middleware that
// needs to validate them must call LookupPending instead.
func (s *TokenStore) Lookup(ctx context.Context, token string) (*LookupResult, error) {
	res, err := s.lookupCommon(ctx, token, false)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// LookupPending is identical to Lookup but accepts only pending_2fa = 1
// sessions. Used by the /api/auth/verify-totp middleware.
func (s *TokenStore) LookupPending(ctx context.Context, token string) (*LookupResult, error) {
	return s.lookupCommon(ctx, token, true)
}

// Revoke deletes the session row + invalidates the cache entry.
func (s *TokenStore) Revoke(ctx context.Context, token string) error {
	hash := util.SHA256Hex(token)
	s.cache.Remove(hash)
	if err := s.sessions.Delete(ctx, hash); err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("revoke: %w", err)
	}
	return nil
}

// RevokeAllForUser drops every session row owned by userID and purges the
// cache. Called on password change, 2FA disable, admin reset, etc.
func (s *TokenStore) RevokeAllForUser(ctx context.Context, userID string) error {
	if err := s.sessions.DeleteAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("revoke all: %w", err)
	}
	// Purge cache entries that match this user. The LRU cache exposes Keys()
	// for iteration; we accept the O(N) walk since this is a one-off event.
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for _, key := range s.cache.Keys() {
		if entry, ok := s.cache.Peek(key); ok && entry.user != nil && entry.user.ID == userID {
			s.cache.Remove(key)
		}
	}
	return nil
}

// PromoteFromPending flips an existing pending_2fa session to a full session
// and re-issues a fresh token (the pending one is immediately revoked). This
// is the verify-totp / verify-recovery hot path.
func (s *TokenStore) PromoteFromPending(ctx context.Context, pendingToken string) (string, time.Time, *storage.UserRecord, error) {
	hash := util.SHA256Hex(pendingToken)
	rec, err := s.sessions.GetByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			return "", time.Time{}, nil, ErrPendingTokenInvalid
		}
		return "", time.Time{}, nil, fmt.Errorf("load pending session: %w", err)
	}
	if !rec.Pending2FA {
		return "", time.Time{}, nil, ErrPendingTokenInvalid
	}
	user, err := s.users.GetByID(ctx, rec.UserID)
	if err != nil {
		return "", time.Time{}, nil, fmt.Errorf("load user: %w", err)
	}
	if !user.IsActive {
		return "", time.Time{}, nil, ErrAccountDisabled
	}
	// Issue fresh token, then drop the pending one. Order matters: if Issue
	// fails we still want the pending row in place so the user can retry.
	token, expiresAt, err := s.Issue(ctx, rec.UserID, rec.IP, rec.UserAgent, false)
	if err != nil {
		return "", time.Time{}, nil, err
	}
	if err := s.sessions.Delete(ctx, hash); err != nil && !errors.Is(err, storage.ErrSessionNotFound) {
		return "", time.Time{}, nil, fmt.Errorf("drop pending session: %w", err)
	}
	s.cache.Remove(hash)
	return token, expiresAt, user, nil
}

// lookupCommon is shared by Lookup / LookupPending.
func (s *TokenStore) lookupCommon(ctx context.Context, token string, wantPending bool) (*LookupResult, error) {
	if token == "" {
		return nil, ErrSessionNotFound
	}
	hash := util.SHA256Hex(token)
	if entry, ok := s.cache.Get(hash); ok {
		if s.now().Sub(entry.insertedAt) < s.cacheTTL &&
			entry.expiresAt > s.now().UnixMilli() {
			// Cache hit is only valid for full sessions; pending sessions
			// are never cached (their lifetime is too short).
			if !wantPending {
				// Touch session asynchronously to keep DB last_used fresh.
				s.bestEffortTouch(ctx, hash)
				return &LookupResult{User: entry.user, SessionID: entry.sessionID, ExpiresAt: entry.expiresAt}, nil
			}
		}
		s.cache.Remove(hash)
	}
	rec, err := s.sessions.GetByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("token lookup: %w", err)
	}
	if rec.Pending2FA != wantPending {
		return nil, ErrSessionNotFound
	}
	user, err := s.users.GetByID(ctx, rec.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("load session user: %w", err)
	}
	if !user.IsActive {
		return nil, ErrAccountDisabled
	}
	newExpiry := s.now().Add(s.ttl)
	// Slide the expiry only when more than half the TTL has elapsed to limit
	// write amplification.
	half := s.ttl / 2
	doSlide := !wantPending && time.UnixMilli(rec.ExpiresAt).Sub(s.now()) < half
	expires := rec.ExpiresAt
	if doSlide {
		if err := s.sessions.Touch(ctx, hash, s.now().UnixMilli(), newExpiry.UnixMilli()); err == nil {
			expires = newExpiry.UnixMilli()
		}
	} else {
		_ = s.sessions.Touch(ctx, hash, s.now().UnixMilli(), 0)
	}
	if !wantPending {
		s.cache.Add(hash, cacheEntry{
			user:       user,
			sessionID:  rec.ID,
			expiresAt:  expires,
			insertedAt: s.now(),
		})
	}
	return &LookupResult{User: user, SessionID: rec.ID, ExpiresAt: expires}, nil
}

// bestEffortTouch updates last_used_at without blocking the caller on a write
// error. Used after a cache hit to keep the metadata fresh.
func (s *TokenStore) bestEffortTouch(ctx context.Context, hash string) {
	_ = s.sessions.Touch(ctx, hash, s.now().UnixMilli(), 0)
}
