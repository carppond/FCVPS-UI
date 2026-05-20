package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// recoveryCodeRE matches the expected lowercase hex format for a single code.
var recoveryCodeRE = regexp.MustCompile(`^[0-9a-f]{8}$`)

func TestGenerateRecoveryCodesShape(t *testing.T) {
	codes := GenerateRecoveryCodes()
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("expected %d codes, got %d", RecoveryCodeCount, len(codes))
	}
	seen := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		if !recoveryCodeRE.MatchString(c) {
			t.Fatalf("code %q does not match lowercase 8-hex pattern", c)
		}
		if _, dup := seen[c]; dup {
			t.Fatalf("duplicate code generated: %q", c)
		}
		seen[c] = struct{}{}
	}
}

func TestHashRecoveryCodesRoundtrip(t *testing.T) {
	codes := []string{"deadbeef", "abad1dea"}
	raw, err := HashRecoveryCodes(codes)
	if err != nil {
		t.Fatalf("HashRecoveryCodes: %v", err)
	}
	decoded, err := DecodeRecoveryCodeHashes(raw)
	if err != nil {
		t.Fatalf("DecodeRecoveryCodeHashes: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 hashes, got %d", len(decoded))
	}
	for _, h := range decoded {
		if len(h) != 64 {
			t.Fatalf("expected 64-char sha256 hex, got %q (len %d)", h, len(h))
		}
	}
}

func TestDecodeRecoveryCodeHashesEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "null"} {
		out, err := DecodeRecoveryCodeHashes(in)
		if err != nil {
			t.Fatalf("input %q: unexpected error %v", in, err)
		}
		if len(out) != 0 {
			t.Fatalf("input %q: expected empty slice, got %v", in, out)
		}
	}
}

// fakeRecoveryRepo is the minimal RecoveryCodeRepository used by the consume tests.
type fakeRecoveryRepo struct {
	stored string
}

func (f *fakeRecoveryRepo) GetRecoveryCodesHash(_ context.Context, _ string) (string, error) {
	return f.stored, nil
}

func (f *fakeRecoveryRepo) UpdateRecoveryCodes(_ context.Context, _ string, hashesJSON string) error {
	f.stored = hashesJSON
	return nil
}

func TestVerifyAndConsumeRecoveryCodeConsumesOnce(t *testing.T) {
	codes := []string{"AAAAAAAA", "bbbbbbbb"}
	hashed, err := HashRecoveryCodes(codes)
	if err != nil {
		t.Fatalf("HashRecoveryCodes: %v", err)
	}
	repo := &fakeRecoveryRepo{stored: hashed}

	remaining, err := VerifyAndConsumeRecoveryCode(context.Background(), repo, "u", "aaaaaaaa")
	if err != nil {
		t.Fatalf("first redemption: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 remaining, got %d", remaining)
	}

	// Reusing the same code must fail.
	_, err = VerifyAndConsumeRecoveryCode(context.Background(), repo, "u", "aaaaaaaa")
	if !errors.Is(err, ErrRecoveryCodeInvalid) {
		t.Fatalf("expected ErrRecoveryCodeInvalid on reuse, got %v", err)
	}

	// The repo should now contain only the second code (after the first was burned).
	decoded, err := DecodeRecoveryCodeHashes(repo.stored)
	if err != nil {
		t.Fatalf("decode after consume: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 stored hash after consume, got %d", len(decoded))
	}
}

func TestVerifyAndConsumeRecoveryCodeNormalisesInput(t *testing.T) {
	codes := []string{"abcd1234"}
	hashed, _ := HashRecoveryCodes(codes)
	repo := &fakeRecoveryRepo{stored: hashed}

	// Mixed-case + hyphens + whitespace should still match.
	_, err := VerifyAndConsumeRecoveryCode(context.Background(), repo, "u", "  ABCD-1234  ")
	if err != nil {
		t.Fatalf("normalised input must be accepted: %v", err)
	}
}

func TestVerifyAndConsumeRecoveryCodeEmptyStore(t *testing.T) {
	repo := &fakeRecoveryRepo{stored: ""}
	_, err := VerifyAndConsumeRecoveryCode(context.Background(), repo, "u", "deadbeef")
	if !errors.Is(err, ErrRecoveryExhausted) {
		t.Fatalf("expected ErrRecoveryExhausted on empty store, got %v", err)
	}
}

func TestGenerateRecoveryCodesPrintability(t *testing.T) {
	for _, c := range GenerateRecoveryCodes() {
		if strings.ContainsAny(c, " \t\n") {
			t.Fatalf("code %q contains whitespace", c)
		}
	}
}
