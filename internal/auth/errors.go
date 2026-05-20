// Package auth implements the M-USER subsystem: password login, TOTP 2FA,
// recovery codes, session token issuance, HTTP middlewares and brute-force
// protection. See docs/05-tech-lead-plan.md T-4 for the task spec.
package auth

import "errors"

// Sentinel errors surfaced by the Manager and its collaborators.
//
// Handler code should compare against these via errors.Is and map them to
// the canonical types.ErrorCode constants for HTTP responses.
var (
	// ErrInvalidCredentials marks "username unknown" or "password wrong"; the
	// two cases are deliberately conflated so callers cannot enumerate users.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrAccountDisabled is returned when a valid login attempt targets a user
	// whose is_active flag is 0.
	ErrAccountDisabled = errors.New("auth: account disabled")

	// ErrTOTPRequired indicates a successful password match against a 2FA-
	// enabled account; the caller must issue a pending-token + ask for the
	// six-digit TOTP code.
	ErrTOTPRequired = errors.New("auth: totp required")

	// ErrTOTPInvalid signals a wrong six-digit code (or a code outside the
	// ±30 s drift window).
	ErrTOTPInvalid = errors.New("auth: totp invalid")

	// ErrTOTPNotEnabled is returned when TOTP-specific actions (disable /
	// regenerate recovery codes) target an account without 2FA on.
	ErrTOTPNotEnabled = errors.New("auth: totp not enabled")

	// ErrTOTPAlreadyEnabled is returned when Enable is called against an
	// account that has already finished the setup flow.
	ErrTOTPAlreadyEnabled = errors.New("auth: totp already enabled")

	// ErrPendingTokenInvalid is surfaced for malformed / expired / wrong-purpose
	// pending tokens supplied by verify-totp / verify-recovery.
	ErrPendingTokenInvalid = errors.New("auth: pending token invalid")

	// ErrRecoveryCodeInvalid signals a wrong (or already used) recovery code.
	ErrRecoveryCodeInvalid = errors.New("auth: recovery code invalid")

	// ErrRecoveryExhausted means every recovery code has been consumed; the
	// user must regenerate the set before retrying.
	ErrRecoveryExhausted = errors.New("auth: recovery codes exhausted")

	// ErrPasswordTooWeak fires when a new / reset password fails the project
	// strength check (length >= 8, mixed character classes).
	ErrPasswordTooWeak = errors.New("auth: password too weak")

	// ErrUsernameTaken / ErrUserAlreadyExists report unique-constraint
	// conflicts on users.username.
	ErrUsernameTaken     = errors.New("auth: username taken")
	ErrUserAlreadyExists = errors.New("auth: user already exists")

	// ErrUserNotFound abstracts the storage layer's "no rows" for lookup-by-id
	// / lookup-by-username helpers.
	ErrUserNotFound = errors.New("auth: user not found")

	// ErrSessionNotFound is returned by TokenStore.Lookup / Revoke when the
	// supplied (hashed) token is unknown or already expired.
	ErrSessionNotFound = errors.New("auth: session not found")

	// ErrForbidden marks "logged in but lacking permission" (e.g. user trying
	// to hit an admin route).
	ErrForbidden = errors.New("auth: forbidden")

	// ErrBruteForceBlocked is returned by Manager.Login when the brute-force
	// protector currently bans the source IP.
	ErrBruteForceBlocked = errors.New("auth: brute-force blocked")
)
