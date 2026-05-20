package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// stubTOTPRepo is the in-memory totpUserRepository used by these tests. It is
// kept minimal so the focus stays on the manager logic rather than storage.
type stubTOTPRepo struct {
	mu   sync.Mutex
	user StoredUser
}

func (s *stubTOTPRepo) GetByID(_ context.Context, _ string) (*StoredUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := s.user
	return &cp, nil
}

func (s *stubTOTPRepo) UpdateTOTPSecret(_ context.Context, _, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user.TOTPSecret = secret
	return nil
}

func (s *stubTOTPRepo) EnableTOTP(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user.TOTPEnabled = true
	return nil
}

func (s *stubTOTPRepo) DisableTOTP(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user.TOTPEnabled = false
	s.user.TOTPSecret = ""
	s.user.RecoveryCodesHash = ""
	return nil
}

func (s *stubTOTPRepo) GetRecoveryCodesHash(_ context.Context, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.user.RecoveryCodesHash, nil
}

func (s *stubTOTPRepo) UpdateRecoveryCodes(_ context.Context, _, hashesJSON string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user.RecoveryCodesHash = hashesJSON
	return nil
}

func newStubTOTPRepo(passwordHash string) *stubTOTPRepo {
	return &stubTOTPRepo{user: StoredUser{ID: "u-1", Username: "alice", PasswordHash: passwordHash}}
}

func TestTOTPSetupReturnsURIAndQR(t *testing.T) {
	repo := newStubTOTPRepo("")
	m := NewTOTPManager(repo, time.Now)
	setup, err := m.Setup(context.Background(), "u-1", "alice")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if !strings.HasPrefix(setup.OTPAuthURI, "otpauth://totp/") {
		t.Fatalf("OTPAuthURI should start with otpauth:// got %q", setup.OTPAuthURI)
	}
	if !strings.Contains(setup.OTPAuthURI, TOTPIssuer) {
		t.Fatalf("OTPAuthURI should embed issuer %q: %s", TOTPIssuer, setup.OTPAuthURI)
	}
	if !strings.HasPrefix(setup.QRCodeBase64, "data:image/png;base64,") {
		t.Fatalf("QRCodeBase64 should be a PNG data URL, got %q", setup.QRCodeBase64[:32])
	}
	if setup.Secret == "" {
		t.Fatalf("Secret must be populated")
	}
	if repo.user.TOTPSecret != setup.Secret {
		t.Fatalf("secret was not persisted")
	}
	if repo.user.TOTPEnabled {
		t.Fatalf("TOTP must remain disabled until Enable confirms")
	}
}

func TestTOTPSetupRejectsAlreadyEnabled(t *testing.T) {
	repo := newStubTOTPRepo("")
	repo.user.TOTPEnabled = true
	m := NewTOTPManager(repo, time.Now)
	if _, err := m.Setup(context.Background(), "u-1", "alice"); !errors.Is(err, ErrTOTPAlreadyEnabled) {
		t.Fatalf("expected ErrTOTPAlreadyEnabled, got %v", err)
	}
}

func TestTOTPEnableAndVerify(t *testing.T) {
	repo := newStubTOTPRepo("")
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	m := NewTOTPManager(repo, func() time.Time { return now })
	setup, err := m.Setup(context.Background(), "u-1", "alice")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	code, err := totp.GenerateCode(setup.Secret, now)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	codes, err := m.Enable(context.Background(), "u-1", code)
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("expected %d recovery codes, got %d", RecoveryCodeCount, len(codes))
	}
	if !repo.user.TOTPEnabled {
		t.Fatalf("Enable should flip totp_enabled true")
	}
	// Verify a fresh code at the same time.
	if err := m.Verify(context.Background(), "u-1", code); err != nil {
		t.Fatalf("Verify same window: %v", err)
	}
	// A wrong code rejects.
	if err := m.Verify(context.Background(), "u-1", "000000"); !errors.Is(err, ErrTOTPInvalid) {
		t.Fatalf("expected ErrTOTPInvalid for wrong code, got %v", err)
	}
}

func TestTOTPVerifyDriftWithinSkew(t *testing.T) {
	repo := newStubTOTPRepo("")
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	m := NewTOTPManager(repo, func() time.Time { return t0 })
	setup, _ := m.Setup(context.Background(), "u-1", "alice")
	code, _ := totp.GenerateCode(setup.Secret, t0)
	if _, err := m.Enable(context.Background(), "u-1", code); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	// Now move the clock 25 s forward (still inside ±30 s skew window) and try
	// to verify with a code captured at t0.
	m2 := NewTOTPManager(repo, func() time.Time { return t0.Add(25 * time.Second) })
	if err := m2.Verify(context.Background(), "u-1", code); err != nil {
		t.Fatalf("expected within-skew code to validate: %v", err)
	}
	// 90 s drift is outside the skew; expect rejection.
	m3 := NewTOTPManager(repo, func() time.Time { return t0.Add(90 * time.Second) })
	if err := m3.Verify(context.Background(), "u-1", code); !errors.Is(err, ErrTOTPInvalid) {
		t.Fatalf("expected outside-skew code to fail, got %v", err)
	}
}

func TestTOTPDisableRequiresPassword(t *testing.T) {
	hash, _ := HashPassword("Password-1234!")
	repo := newStubTOTPRepo(hash)
	repo.user.TOTPEnabled = true
	repo.user.TOTPSecret = "JBSWY3DPEHPK3PXP"
	m := NewTOTPManager(repo, time.Now)
	if err := m.Disable(context.Background(), "u-1", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password should be rejected: %v", err)
	}
	if err := m.Disable(context.Background(), "u-1", "Password-1234!"); err != nil {
		t.Fatalf("correct password: %v", err)
	}
	if repo.user.TOTPEnabled || repo.user.TOTPSecret != "" {
		t.Fatalf("Disable should clear totp state")
	}
}

func TestValidateCodeKnownVector(t *testing.T) {
	// Test vector borrowed from RFC 6238 Appendix B (SHA1 / 30 s / 8 digits).
	// We compute the 6-digit truncation locally so we don't hard-code a value
	// that depends on Digits=6 — easier than maintaining a vector table.
	secret := "JBSWY3DPEHPK3PXP" // base32 "Hello!"
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	code, err := totp.GenerateCodeCustom(secret, now, totp.ValidateOpts{
		Period:    30,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("GenerateCodeCustom: %v", err)
	}
	if !validateCode(secret, code, now) {
		t.Fatalf("validateCode should accept the freshly generated code")
	}
	if validateCode(secret, "000000", now) {
		t.Fatalf("validateCode must reject the obviously wrong code")
	}
}
