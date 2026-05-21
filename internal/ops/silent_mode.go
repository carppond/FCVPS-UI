// Package ops hosts the operational primitives — silent-mode prefix rotation,
// settings management, and backup/restore — used by the M-OPS admin surface.
//
// The package keeps cross-cutting workflows out of the HTTP handler so the
// rotate / backup paths can be unit-tested without spinning up a mux. Handler
// code wraps the methods exposed here with auth + JSON marshalling.
package ops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"shiguang-vps/internal/storage"
)

// silentPrefixLen is the byte length of the random prefix; encoded as hex it
// yields the canonical 32-character string consumed by silent_mode middleware.
const silentPrefixLen = 16

// hexPrefixPattern enforces "exactly 32 lowercase hex characters" — mirrors
// the regex used by the silent_mode middleware so the two layers cannot drift.
var hexPrefixPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// PrefixApplier is the callback the rotation flow uses to push the freshly
// generated prefix into the live middleware. Returning an error aborts the
// rotation BEFORE settings are written, so the running server never ends up
// with a state where the middleware accepts a prefix that is not yet in the
// DB (or vice-versa).
//
// In production this is wired to silent_mode middleware's SetPrefix; in tests
// it's a no-op closure.
type PrefixApplier func(newPrefix string)

// SessionRevoker matches the subset of *auth.TokenStore the rotation flow
// needs. Defining it as an interface lets tests avoid the full auth wiring.
type SessionRevoker interface {
	RevokeAll(ctx context.Context) (int64, error)
}

// SilentModeConfig wires SilentMode to its collaborators.
type SilentModeConfig struct {
	Repo      *storage.SettingsRepo
	Applier   PrefixApplier  // called with the new prefix before persistence
	Revoker   SessionRevoker // optional — when set, Rotate force-logs every user
	Logger    *slog.Logger
}

// SilentMode owns the prefix generation / rotation policy. Construct via
// NewSilentMode.
type SilentMode struct {
	repo    *storage.SettingsRepo
	applier PrefixApplier
	revoker SessionRevoker
	logger  *slog.Logger
}

// NewSilentMode returns a configured SilentMode. cfg.Repo is required;
// Applier / Revoker / Logger may be nil for tests.
func NewSilentMode(cfg SilentModeConfig) (*SilentMode, error) {
	if cfg.Repo == nil {
		return nil, fmt.Errorf("silent mode: settings repo required")
	}
	return &SilentMode{
		repo:    cfg.Repo,
		applier: cfg.Applier,
		revoker: cfg.Revoker,
		logger:  cfg.Logger,
	}, nil
}

// Generate returns a fresh 32-character lowercase hex string drawn from the
// system CSPRNG. Panics only if crypto/rand fails, which would indicate the OS
// entropy pool is unavailable — at that point the host is unusable anyway.
func (s *SilentMode) Generate() string {
	buf := make([]byte, silentPrefixLen)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failures are catastrophic; surfacing as a panic keeps
		// the call site simple (no error return everywhere) and matches the
		// behaviour of util.RandomHex32.
		panic(fmt.Errorf("silent mode generate: %w", err))
	}
	return hex.EncodeToString(buf)
}

// Validate reports whether prefix is a syntactically-valid 32-hex string.
// Strict matching mirrors the middleware regex — uppercase, whitespace, and
// non-hex characters all fail. Callers that want lenient input should lower-
// case and trim before invoking.
func (s *SilentMode) Validate(prefix string) bool {
	return hexPrefixPattern.MatchString(prefix)
}

// Rotate generates a new prefix, persists it under
// storage.SettingSilentModePrefix, hot-applies it to the live middleware
// (via the Applier callback), and force-logs every user by purging the
// sessions table. The new prefix is returned to the caller so the admin can
// be shown a one-time "copy this URL" toast (see UI §17).
//
// Ordering rationale:
//  1. Generate + Validate first — no DB write if the CSPRNG hiccups.
//  2. Persist BEFORE Applier so the middleware can pick up the new value on
//     its next poll even if the in-process Applier crashes between steps.
//  3. Apply to live middleware so subsequent requests start using the new
//     prefix immediately (no 30 s reload lag).
//  4. Revoke sessions LAST — once we get here the new prefix is the only
//     valid one, so wiping tokens force-logs anyone still using the old URL.
//     If revocation fails the prefix is still rotated; we log + continue
//     because the failure mode is "users got a longer-lived session" which
//     is less bad than "rotation half-applied".
func (s *SilentMode) Rotate(ctx context.Context) (string, error) {
	newPrefix := s.Generate()
	if !s.Validate(newPrefix) {
		return "", fmt.Errorf("silent mode rotate: generated prefix failed validation")
	}
	if err := s.repo.Set(ctx, storage.SettingSilentModePrefix, newPrefix); err != nil {
		return "", fmt.Errorf("persist new prefix: %w", err)
	}
	if s.applier != nil {
		s.applier(newPrefix)
	}
	if s.revoker != nil {
		n, err := s.revoker.RevokeAll(ctx)
		if err != nil {
			// Log but do not fail the rotation — the prefix is already live.
			if s.logger != nil {
				s.logger.Warn("silent_mode: prefix rotated but session purge failed",
					slog.String("err", err.Error()))
			}
		} else if s.logger != nil {
			s.logger.Info("silent_mode: prefix rotated; sessions purged",
				slog.Int64("sessions_revoked", n),
				slog.String("prefix_prefix", maskPrefix(newPrefix)))
		}
	} else if s.logger != nil {
		s.logger.Info("silent_mode: prefix rotated",
			slog.String("prefix_prefix", maskPrefix(newPrefix)))
	}
	return newPrefix, nil
}

// Current returns the prefix persisted in system_settings. Empty string +
// nil error means silent mode is disabled (no row).
func (s *SilentMode) Current(ctx context.Context) (string, error) {
	value, err := s.repo.Get(ctx, storage.SettingSilentModePrefix)
	if err != nil {
		if err == storage.ErrSettingNotFound {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

// EnsureInitial generates a prefix on first boot when none is persisted, and
// returns whatever is now stored. Idempotent: a second call observes the row
// from the first call and returns it unchanged. Designed for cmd/server's
// startup sequence (architecture §1 step 5).
func (s *SilentMode) EnsureInitial(ctx context.Context) (string, error) {
	current, err := s.Current(ctx)
	if err != nil {
		return "", err
	}
	if current != "" {
		return current, nil
	}
	prefix := s.Generate()
	if err := s.repo.Set(ctx, storage.SettingSilentModePrefix, prefix); err != nil {
		return "", fmt.Errorf("persist initial prefix: %w", err)
	}
	if s.logger != nil {
		s.logger.Info("silent_mode: initial prefix generated",
			slog.String("prefix_prefix", maskPrefix(prefix)))
	}
	return prefix, nil
}

// maskPrefix returns "abcd...wxyz" — the first 4 + last 4 hex chars — so the
// full value never lands in logs / audit captures. Matches the masking used by
// the silent_mode middleware on reload events.
func maskPrefix(prefix string) string {
	if len(prefix) <= 8 {
		return strings.Repeat("*", len(prefix))
	}
	return prefix[:4] + "..." + prefix[len(prefix)-4:]
}
