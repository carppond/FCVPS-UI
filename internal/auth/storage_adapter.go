package auth

import (
	"context"

	"shiguang-vps/internal/storage"
)

// StorageUserAdapter adapts *storage.UserRepo to the package-internal
// totpUserRepository interface. It exists because the TOTP manager wants the
// auth-package's StoredUser projection (which includes the password hash) but
// the storage layer naturally returns its own UserRecord type.
//
// The adapter is a thin field-by-field shim; callers should treat it as a
// drop-in replacement for *storage.UserRepo whenever a totpUserRepository is
// required.
type StorageUserAdapter struct {
	Repo *storage.UserRepo
}

// NewStorageUserAdapter wraps repo so it can satisfy totpUserRepository.
func NewStorageUserAdapter(repo *storage.UserRepo) *StorageUserAdapter {
	return &StorageUserAdapter{Repo: repo}
}

// GetByID implements totpUserRepository.GetByID.
func (a *StorageUserAdapter) GetByID(ctx context.Context, id string) (*StoredUser, error) {
	rec, err := a.Repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return recordToStoredUser(rec), nil
}

// UpdateTOTPSecret implements totpUserRepository.UpdateTOTPSecret.
func (a *StorageUserAdapter) UpdateTOTPSecret(ctx context.Context, userID, secret string) error {
	return a.Repo.UpdateTOTPSecret(ctx, userID, secret)
}

// EnableTOTP implements totpUserRepository.EnableTOTP.
func (a *StorageUserAdapter) EnableTOTP(ctx context.Context, userID string) error {
	return a.Repo.EnableTOTP(ctx, userID)
}

// DisableTOTP implements totpUserRepository.DisableTOTP.
func (a *StorageUserAdapter) DisableTOTP(ctx context.Context, userID string) error {
	return a.Repo.DisableTOTP(ctx, userID)
}

// UpdateTOTPLastStep implements totpUserRepository.UpdateTOTPLastStep.
func (a *StorageUserAdapter) UpdateTOTPLastStep(ctx context.Context, userID string, step int64) error {
	return a.Repo.UpdateTOTPLastStep(ctx, userID, step)
}

// GetRecoveryCodesHash implements RecoveryCodeRepository.GetRecoveryCodesHash.
func (a *StorageUserAdapter) GetRecoveryCodesHash(ctx context.Context, userID string) (string, error) {
	return a.Repo.GetRecoveryCodesHash(ctx, userID)
}

// UpdateRecoveryCodes implements RecoveryCodeRepository.UpdateRecoveryCodes.
func (a *StorageUserAdapter) UpdateRecoveryCodes(ctx context.Context, userID, hashesJSON string) error {
	return a.Repo.UpdateRecoveryCodes(ctx, userID, hashesJSON)
}

// recordToStoredUser projects the storage record into the auth-internal type.
func recordToStoredUser(rec *storage.UserRecord) *StoredUser {
	if rec == nil {
		return nil
	}
	return &StoredUser{
		ID:                rec.ID,
		Username:          rec.Username,
		PasswordHash:      rec.PasswordHash,
		Role:              rec.Role,
		IsActive:          rec.IsActive,
		Email:             rec.Email,
		Locale:            rec.Locale,
		TOTPSecret:        rec.TOTPSecret,
		TOTPEnabled:       rec.TOTPEnabled,
		TOTPLastStep:      rec.TOTPLastStep,
		RecoveryCodesHash: rec.RecoveryCodesHash,
		CreatedAt:         rec.CreatedAt,
		UpdatedAt:         rec.UpdatedAt,
	}
}
