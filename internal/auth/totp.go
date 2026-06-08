package auth

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPIssuer is the issuer label embedded in the otpauth URI; Google
// Authenticator displays it next to the account label.
const TOTPIssuer = "ShiguangVPS"

// totpQRSize is the side length (pixels) used when rendering the provisioning
// QR code. 256x256 fits comfortably on phone screens at default DPI.
const totpQRSize = 256

// totpSkew is the number of 30 s windows tolerated for clock drift on either
// side of the present moment. ±1 (i.e. ±30 s) is the industry default and
// matches what Google Authenticator / Authy do internally.
const totpSkew = 1

// TOTPManager bundles the TOTP-specific operations (setup, enable, verify,
// disable). Concrete implementations live below; the interface exists so the
// HTTP handler layer can be unit-tested against a fake.
type TOTPManager interface {
	Setup(ctx context.Context, userID, username string) (*TOTPSetup, error)
	Enable(ctx context.Context, userID, code string) ([]string, error)
	Disable(ctx context.Context, userID, password string) error
	Verify(ctx context.Context, userID, code string) error
}

// TOTPSetup carries the artifacts a fresh TOTP setup returns to the client.
// QRCodePNG is base64-encoded data: URL ready for <img src="…" />.
type TOTPSetup struct {
	Secret       string
	OTPAuthURI   string
	QRCodeBase64 string
}

// totpUserRepository captures the storage calls TOTP needs. internal/storage.UserRepo
// satisfies it.
type totpUserRepository interface {
	RecoveryCodeRepository
	GetByID(ctx context.Context, id string) (*StoredUser, error)
	UpdateTOTPSecret(ctx context.Context, userID, secret string) error
	EnableTOTP(ctx context.Context, userID string) error
	DisableTOTP(ctx context.Context, userID string) error
	UpdateTOTPLastStep(ctx context.Context, userID string, step int64) error
}

// StoredUser is the internal projection auth needs from the users row. It is
// kept here (and not in internal/types) so the auth package owns the password
// hash field; HTTP responses use types.UserPublicProfile instead.
type StoredUser struct {
	ID                string
	Username          string
	PasswordHash      string
	Role              string
	IsActive          bool
	Email             string
	Locale            string
	TOTPSecret        string
	TOTPEnabled       bool
	TOTPLastStep      int64
	RecoveryCodesHash string
	CreatedAt         int64
	UpdatedAt         int64
}

// defaultTOTPManager is the production TOTPManager implementation. It is
// returned by NewTOTPManager.
type defaultTOTPManager struct {
	users totpUserRepository
	now   func() time.Time
}

// NewTOTPManager wires a TOTPManager around the supplied user repository.
// When now is nil the production clock is used.
func NewTOTPManager(users totpUserRepository, now func() time.Time) TOTPManager {
	if now == nil {
		now = time.Now
	}
	return &defaultTOTPManager{users: users, now: now}
}

// Setup generates a fresh TOTP secret, persists it (with TOTP still disabled),
// and returns the otpauth provisioning URI + a base64-encoded QR PNG. Calling
// Setup again replaces the previous (un-confirmed) secret — there is no harm
// in repeating until the user successfully captures the QR.
func (m *defaultTOTPManager) Setup(ctx context.Context, userID, username string) (*TOTPSetup, error) {
	if userID == "" || username == "" {
		return nil, fmt.Errorf("totp.Setup: userID/username required")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: username,
		Period:      30,
		SecretSize:  20,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		return nil, fmt.Errorf("totp generate: %w", err)
	}
	user, err := m.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.TOTPEnabled {
		return nil, ErrTOTPAlreadyEnabled
	}
	if err := m.users.UpdateTOTPSecret(ctx, userID, key.Secret()); err != nil {
		return nil, fmt.Errorf("persist totp secret: %w", err)
	}
	qrPNG, err := renderQR(key)
	if err != nil {
		return nil, err
	}
	return &TOTPSetup{
		Secret:       key.Secret(),
		OTPAuthURI:   key.URL(),
		QRCodeBase64: "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrPNG),
	}, nil
}

// Enable confirms the first six-digit code, flips totp_enabled=1 and returns
// a freshly-minted batch of recovery codes (plaintext, shown to the user
// exactly once).
func (m *defaultTOTPManager) Enable(ctx context.Context, userID, code string) ([]string, error) {
	user, err := m.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.TOTPEnabled {
		return nil, ErrTOTPAlreadyEnabled
	}
	if user.TOTPSecret == "" {
		return nil, fmt.Errorf("%w: setup not started", ErrTOTPInvalid)
	}
	if !validateCode(user.TOTPSecret, code, m.now()) {
		return nil, ErrTOTPInvalid
	}
	codes := GenerateRecoveryCodes()
	hashesJSON, err := HashRecoveryCodes(codes)
	if err != nil {
		return nil, err
	}
	if err := m.users.UpdateRecoveryCodes(ctx, userID, hashesJSON); err != nil {
		return nil, fmt.Errorf("persist recovery codes: %w", err)
	}
	if err := m.users.EnableTOTP(ctx, userID); err != nil {
		return nil, fmt.Errorf("enable totp: %w", err)
	}
	return codes, nil
}

// Disable turns 2FA off after re-checking the user's password. Recovery
// codes are cleared in the same write.
func (m *defaultTOTPManager) Disable(ctx context.Context, userID, password string) error {
	user, err := m.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.TOTPEnabled {
		return ErrTOTPNotEnabled
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return ErrInvalidCredentials
	}
	if err := m.users.DisableTOTP(ctx, userID); err != nil {
		return fmt.Errorf("disable totp: %w", err)
	}
	if err := m.users.UpdateRecoveryCodes(ctx, userID, ""); err != nil {
		return fmt.Errorf("clear recovery codes: %w", err)
	}
	return nil
}

// Verify checks a six-digit code against the user's current TOTP secret
// without changing state. Returns nil on success and ErrTOTPInvalid otherwise.
func (m *defaultTOTPManager) Verify(ctx context.Context, userID, code string) error {
	user, err := m.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if !user.TOTPEnabled || user.TOTPSecret == "" {
		return ErrTOTPNotEnabled
	}
	step, ok := matchCodeStep(user.TOTPSecret, code, m.now())
	if !ok {
		return ErrTOTPInvalid
	}
	// Replay protection: reject any code from a step at or before the last
	// successfully consumed one (a code stays valid for ~90s with skew=1, so
	// the same digits must not be accepted twice).
	if step <= user.TOTPLastStep {
		return ErrTOTPInvalid
	}
	if err := m.users.UpdateTOTPLastStep(ctx, userID, step); err != nil {
		return err
	}
	return nil
}

// validateCode runs the pquerna/otp validator with the project skew.
func validateCode(secret, code string, now time.Time) bool {
	_, ok := matchCodeStep(secret, code, now)
	return ok
}

// matchCodeStep validates code against secret and, on success, returns the
// 30-second time-step (unix/period) the code corresponds to — used for replay
// protection. Checks the current step plus ±totpSkew neighbours.
func matchCodeStep(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return 0, false
	}
	const period = 30
	base := now.Unix() / period
	for d := int64(-totpSkew); d <= int64(totpSkew); d++ {
		step := base + d
		want, err := totp.GenerateCodeCustom(secret, time.Unix(step*period, 0), totp.ValidateOpts{
			Period:    period,
			Skew:      0,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

// renderQR encodes the otpauth URI as a 256x256 PNG.
func renderQR(key *otp.Key) ([]byte, error) {
	img, err := key.Image(totpQRSize, totpQRSize)
	if err != nil {
		return nil, fmt.Errorf("encode qr: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}
