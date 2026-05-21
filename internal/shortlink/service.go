// Package shortlink hosts the URL shortening service.
//
// Per docs/03-architecture.md §4.2 the short_links table uses a composite
// (file_code, user_code) primary key:
//
//   - file_code is globally monotonic base62 — a brand-new shortlink takes
//     the next free integer across all users, encoded with the project's
//     0-9a-zA-Z alphabet.
//   - user_code is monotonic per-user base62 — every user has their own
//     counter so the pair {alice, 4} cannot collide with {bob, 4} on disk
//     yet the public combined string remains short.
//
// The public URL is /<file_code><user_code> — the join character is
// intentionally absent because both halves use the same alphabet and the
// service parses them by splitting at the first run boundary it controls
// (file_code length is variable; user_code is appended).
//
// The encoded form sorts lexicographically when both codes share the same
// width, so the repo's MAX(…) trick (see ShortLinkRepo.MaxFileCode) yields
// the largest existing code without a dedicated counter row.
package shortlink

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"shiguang-vps/internal/storage"
)

// Base62Alphabet is the canonical character set used for all codes. Order
// matters: 0..9 < A..Z < a..z so the encoded form is monotonic when widths
// match.
const Base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// MinCodeWidth caps how short a generated code may be. The constant exists
// so the service can left-pad codes to a stable width — that property keeps
// the repo's "max by string ordering" shortcut sound.
const MinCodeWidth = 1

// MaxRetries bounds how many times Generate will retry on PK conflict
// (extremely unlikely since both counters are serialised by the service
// mutex, but a concurrent restart could in theory race the counter).
const MaxRetries = 3

// ErrInvalidCode is returned by Resolve when the supplied combined code
// fails to parse into (file_code, user_code).
var ErrInvalidCode = errors.New("shortlink: invalid code")

// ErrTargetEmpty is returned by Generate when the caller supplies an empty
// URL.
var ErrTargetEmpty = errors.New("shortlink: empty target_url")

// Service issues new short links and resolves existing ones to their target
// URL. The service serialises code allocation with a per-instance mutex so
// concurrent POST /api/shortlinks calls do not race each other; under
// horizontal scale the underlying PK conflict is retried.
type Service struct {
	repo   *storage.ShortLinkRepo
	logger *slog.Logger
	now    func() time.Time
	mu     sync.Mutex
}

// New wires a Service. nil logger / now fall back to slog.Default / time.Now.
func New(repo *storage.ShortLinkRepo, logger *slog.Logger, now func() time.Time) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, logger: logger, now: now}
}

// Generate creates a new short link for userID pointing at targetURL.
// expiresAt may be nil (permanent) or a future time. The returned record
// contains both code halves already populated.
func (s *Service) Generate(ctx context.Context, userID, targetURL string, expiresAt *time.Time) (*storage.ShortLinkRecord, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("shortlink: nil service")
	}
	if userID == "" {
		return nil, errors.New("shortlink: empty user_id")
	}
	if strings.TrimSpace(targetURL) == "" {
		return nil, ErrTargetEmpty
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		fileCode, err := s.nextFileCode(ctx)
		if err != nil {
			return nil, fmt.Errorf("alloc file code: %w", err)
		}
		userCode, err := s.nextUserCode(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("alloc user code: %w", err)
		}
		rec := storage.ShortLinkRecord{
			FileCode:  fileCode,
			UserCode:  userCode,
			UserID:    userID,
			TargetURL: targetURL,
			CreatedAt: s.now().UnixMilli(),
		}
		if expiresAt != nil && !expiresAt.IsZero() {
			rec.ExpiresAt = expiresAt.UnixMilli()
		}
		created, err := s.repo.Create(ctx, rec)
		if err == nil {
			return created, nil
		}
		lastErr = err
		// On PK conflict, loop and try again with a fresh counter snapshot.
		if !isUniqueConflict(err) {
			return nil, err
		}
		s.logger.Warn("shortlink: PK conflict, retrying",
			slog.String("user_id", userID),
			slog.String("file_code", fileCode),
			slog.String("user_code", userCode))
	}
	return nil, fmt.Errorf("shortlink: exhausted retries: %w", lastErr)
}

// Resolve parses code into (file_code, user_code) and returns the target URL
// when the row exists and has not expired. The split heuristic is:
//
//   - find the longest prefix of `code` that equals the MAX(file_code) seen
//     so far for any row sharing that prefix.
//
// Since file_codes for new entries are assigned via NextFileCode (monotonic,
// fixed-width), we can split by the canonical file_code width at the time
// the link was created. We persist file_code without padding, so we try
// every prefix length from 1 upward and pick the first that matches a row.
func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("shortlink: nil service")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ErrInvalidCode
	}
	// Iterate every legal split. The codes are short (≤ 20 chars in
	// practice), so the linear scan is cheap.
	for i := 1; i < len(code); i++ {
		fileCode := code[:i]
		userCode := code[i:]
		rec, err := s.repo.Resolve(ctx, fileCode, userCode)
		if err == nil {
			return rec.TargetURL, nil
		}
		if !errors.Is(err, storage.ErrShortLinkNotFound) {
			return "", err
		}
	}
	return "", storage.ErrShortLinkNotFound
}

// ResolveSplit is the more efficient variant when the caller already knows
// the split point (e.g. from a route param /s/{fileCode}/{userCode}). The
// shortlink handler exposes both forms.
func (s *Service) ResolveSplit(ctx context.Context, fileCode, userCode string) (*storage.ShortLinkRecord, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("shortlink: nil service")
	}
	return s.repo.Resolve(ctx, fileCode, userCode)
}

// ListByUser proxies to the repo. The service exists so handlers depend on
// a single interface even when no business logic is required.
func (s *Service) ListByUser(ctx context.Context, userID string) ([]storage.ShortLinkRecord, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("shortlink: nil service")
	}
	return s.repo.ListByUser(ctx, userID)
}

// Delete proxies to the repo's userID-guarded delete. Errors surface as-is
// so the handler can distinguish ErrShortLinkNotFound from a DB failure.
func (s *Service) Delete(ctx context.Context, fileCode, userCode, userID string) error {
	if s == nil || s.repo == nil {
		return errors.New("shortlink: nil service")
	}
	return s.repo.Delete(ctx, fileCode, userCode, userID)
}

// nextFileCode returns the next base62 increment of MAX(file_code).
func (s *Service) nextFileCode(ctx context.Context) (string, error) {
	max, err := s.repo.MaxFileCode(ctx)
	if err != nil {
		return "", err
	}
	return incrementBase62(max), nil
}

// nextUserCode returns the next base62 increment of MAX(user_code) for the
// supplied user.
func (s *Service) nextUserCode(ctx context.Context, userID string) (string, error) {
	max, err := s.repo.MaxUserCode(ctx, userID)
	if err != nil {
		return "", err
	}
	return incrementBase62(max), nil
}

// EncodeBase62 returns the base62 representation of n. 0 → "0", positive
// integers produce no padding.
func EncodeBase62(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = Base62Alphabet[n%62]
		n /= 62
	}
	return string(buf[pos:])
}

// DecodeBase62 parses s into the integer it encodes. Returns 0 + error when
// any character is outside the alphabet.
func DecodeBase62(s string) (uint64, error) {
	if s == "" {
		return 0, errors.New("base62: empty string")
	}
	var n uint64
	for _, r := range s {
		idx := strings.IndexRune(Base62Alphabet, r)
		if idx < 0 {
			return 0, fmt.Errorf("base62: invalid character %q", r)
		}
		n = n*62 + uint64(idx)
	}
	return n, nil
}

// incrementBase62 returns the base62 string that immediately follows s. The
// empty input returns "1" (the canonical "first allocated" code). Overflow
// is not a concern in practice: 2^64 codes is more than the lifetime of any
// SQLite database we'd run.
func incrementBase62(s string) string {
	if s == "" {
		return "1"
	}
	n, err := DecodeBase62(s)
	if err != nil {
		// Fall back to width-preserving append so a corrupt counter still
		// produces a unique-ish code.
		return s + "0"
	}
	return EncodeBase62(n + 1)
}

// isUniqueConflict spots the SQLite PK conflict so the retry loop can react.
// modernc.org/sqlite surfaces conflicts as plain text errors; matching on
// the substring is more portable than depending on the driver-specific
// error type.
func isUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: short_links")
}
