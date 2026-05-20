package auth

import (
	"fmt"
	"unicode"

	"shiguang-vps/internal/util"
)

// MinPasswordLength is the lower bound enforced by ValidatePassword. The
// project policy (PRD §6.3) is "≥ 8 chars + at least three of:
// upper/lower/digit/symbol".
const MinPasswordLength = 8

// HashPassword wraps util.HashPassword so the auth package owns its own
// strength check + bcrypt invocation. Returns ErrPasswordTooWeak when the
// plaintext fails ValidatePassword.
func HashPassword(plaintext string) (string, error) {
	if err := ValidatePassword(plaintext); err != nil {
		return "", err
	}
	hash, err := util.HashPassword(plaintext)
	if err != nil {
		return "", fmt.Errorf("auth.HashPassword: %w", err)
	}
	return hash, nil
}

// VerifyPassword is a thin alias over util.VerifyPassword kept here so call
// sites importing the auth package do not need to depend on internal/util
// directly.
func VerifyPassword(plaintext, hashed string) bool {
	return util.VerifyPassword(plaintext, hashed)
}

// ValidatePassword enforces the project password policy. Returns
// ErrPasswordTooWeak with a wrapped reason when the input is rejected.
func ValidatePassword(plaintext string) error {
	if len(plaintext) < MinPasswordLength {
		return fmt.Errorf("%w: length < %d", ErrPasswordTooWeak, MinPasswordLength)
	}
	if len(plaintext) > 128 {
		return fmt.Errorf("%w: length > 128", ErrPasswordTooWeak)
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range plaintext {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	classes := 0
	for _, b := range []bool{hasUpper, hasLower, hasDigit, hasSymbol} {
		if b {
			classes++
		}
	}
	if classes < 3 {
		return fmt.Errorf("%w: need at least 3 character classes", ErrPasswordTooWeak)
	}
	return nil
}

// GenerateStrongPassword returns a 16-character password constructed from
// upper / lower / digit / symbol classes. Used by EnsureAdmin and admin
// reset-password.
func GenerateStrongPassword() string {
	const (
		upper  = "ABCDEFGHJKMNPQRSTUVWXYZ"
		lower  = "abcdefghjkmnpqrstuvwxyz"
		digit  = "23456789"
		symbol = "!@#$%^&*-_=+"
	)
	all := upper + lower + digit + symbol
	out := make([]byte, 16)
	bytes := util.RandomBytes(16)
	// Force one of each class in the first four positions.
	out[0] = upper[int(bytes[0])%len(upper)]
	out[1] = lower[int(bytes[1])%len(lower)]
	out[2] = digit[int(bytes[2])%len(digit)]
	out[3] = symbol[int(bytes[3])%len(symbol)]
	for i := 4; i < 16; i++ {
		out[i] = all[int(bytes[i])%len(all)]
	}
	// Fisher-Yates shuffle using a second batch of randomness to spread the
	// forced characters across the string.
	shuf := util.RandomBytes(16)
	for i := 15; i > 0; i-- {
		j := int(shuf[i]) % (i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}
