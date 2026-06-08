package auth

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"shiguang-vps/internal/util"
)

// RecoveryCodeCount is the number of one-shot codes minted on Enable /
// regenerate; PRD M-USER-3 specifies 8.
const RecoveryCodeCount = 8

// RecoveryCodeLength is the textual length of each code (hex chars). 16 hex
// characters → 64 bits of entropy, so a leaked recovery_codes_hash column
// cannot be brute-forced/enumerated offline (unsalted sha256 of a 64-bit
// value has a 2^64 preimage cost). Older 8-hex codes minted before this bump
// keep verifying — they are hashed the same way.
const RecoveryCodeLength = 16

// GenerateRecoveryCodes returns RecoveryCodeCount distinct codes in plaintext.
// Caller must hash them via HashRecoveryCodes before persisting; the plaintext
// list is shown to the user exactly once.
func GenerateRecoveryCodes() []string {
	out := make([]string, 0, RecoveryCodeCount)
	seen := make(map[string]struct{}, RecoveryCodeCount)
	for len(out) < RecoveryCodeCount {
		code := newRecoveryCode()
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

// HashRecoveryCodes returns the JSON-encoded array of sha256 hashes suitable
// for users.recovery_codes_hash.
func HashRecoveryCodes(codes []string) (string, error) {
	hashes := make([]string, len(codes))
	for i, c := range codes {
		hashes[i] = util.SHA256Hex(normaliseRecoveryCode(c))
	}
	buf, err := json.Marshal(hashes)
	if err != nil {
		return "", fmt.Errorf("recovery codes marshal: %w", err)
	}
	return string(buf), nil
}

// DecodeRecoveryCodeHashes parses the JSON array stored in
// users.recovery_codes_hash. Empty input yields an empty slice (no error).
func DecodeRecoveryCodeHashes(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("recovery codes unmarshal: %w", err)
	}
	return out, nil
}

// RecoveryCodeRepository abstracts the persistence calls
// VerifyAndConsumeRecoveryCode needs. internal/storage.UserRepo satisfies it.
type RecoveryCodeRepository interface {
	GetRecoveryCodesHash(ctx context.Context, userID string) (string, error)
	UpdateRecoveryCodes(ctx context.Context, userID, hashesJSON string) error
}

// VerifyAndConsumeRecoveryCode burns one matching recovery code from userID.
// It returns the number of codes still available after the call.
//
//   - ErrRecoveryCodeInvalid is returned when no entry matches (the supplied
//     code is wrong OR was already used).
//   - ErrRecoveryExhausted is returned when the stored set is empty before
//     this call.
func VerifyAndConsumeRecoveryCode(ctx context.Context, repo RecoveryCodeRepository, userID, code string) (remaining int, err error) {
	stored, err := repo.GetRecoveryCodesHash(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("load recovery codes: %w", err)
	}
	hashes, err := DecodeRecoveryCodeHashes(stored)
	if err != nil {
		return 0, err
	}
	if len(hashes) == 0 {
		return 0, ErrRecoveryExhausted
	}
	want := util.SHA256Hex(normaliseRecoveryCode(code))
	// Constant-time scan: compare every entry without an early break so the
	// matched index isn't revealed through timing.
	idx := -1
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 {
			idx = i
		}
	}
	if idx < 0 {
		return len(hashes), ErrRecoveryCodeInvalid
	}
	hashes = append(hashes[:idx], hashes[idx+1:]...)
	out, err := json.Marshal(hashes)
	if err != nil {
		return 0, fmt.Errorf("marshal remaining hashes: %w", err)
	}
	if err := repo.UpdateRecoveryCodes(ctx, userID, string(out)); err != nil {
		return 0, fmt.Errorf("persist remaining hashes: %w", err)
	}
	return len(hashes), nil
}

// newRecoveryCode returns a single lowercase 8-hex recovery code. Internal —
// callers want GenerateRecoveryCodes for the full set.
func newRecoveryCode() string {
	buf := util.RandomBytes(RecoveryCodeLength / 2)
	return hex.EncodeToString(buf)
}

// normaliseRecoveryCode strips whitespace and lowercases the input so users
// who paste the printed code can survive incidental hyphens / capitalisation.
func normaliseRecoveryCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}
