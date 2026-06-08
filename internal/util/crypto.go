package util

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"shiguang-vps/internal/config"
)

// SHA256Hex returns the lowercase hex digest of sha256(input).
func SHA256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// SHA256HexBytes mirrors SHA256Hex for raw byte input. Avoids the implicit
// conversion when callers already hold a []byte.
func SHA256HexBytes(input []byte) string {
	sum := sha256.Sum256(input)
	return hex.EncodeToString(sum[:])
}

// HashPassword hashes plaintext with bcrypt at the project-wide cost
// (config.DefaultBcryptCost). Returns the encoded hash including salt.
//
// Under `go test` the cost drops to bcrypt.MinCost: cost-12 hashing × the race
// detector × the many hashes the suite performs otherwise blows the per-package
// test timeout. Production binaries always use the full cost.
func HashPassword(plaintext string) (string, error) {
	if plaintext == "" {
		return "", fmt.Errorf("util.HashPassword: empty plaintext")
	}
	cost := config.DefaultBcryptCost
	if testing.Testing() {
		cost = bcrypt.MinCost
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(plaintext), cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hashed), nil
}

// VerifyPassword reports whether plaintext matches the bcrypt hash. Returns
// false on any decoding error (callers should not distinguish "wrong password"
// from "malformed hash" to avoid timing side-channels).
func VerifyPassword(plaintext, hashed string) bool {
	if plaintext == "" || hashed == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plaintext))
	return err == nil
}

// Base64URL encodes bytes using URL-safe base64 without padding (RFC 4648 §5).
// Suitable for session/access tokens delivered through URLs.
func Base64URL(in []byte) string {
	return base64.RawURLEncoding.EncodeToString(in)
}

// UnBase64URL decodes a string produced by Base64URL. Returns the decoded
// bytes or an error wrapping the underlying decode failure.
func UnBase64URL(s string) ([]byte, error) {
	out, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode base64url: %w", err)
	}
	return out, nil
}
