package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/ratelimit"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// AuthHandler hosts the public-facing /api/auth/* endpoints.
type AuthHandler struct {
	manager      *auth.Manager
	tokenStore   *auth.TokenStore
	brute        *auth.BruteProtector
	loginLimiter *ratelimit.Limiter
	logger       *slog.Logger
}

// NewAuthHandler wires the handler. loginLimiter, brute, logger may be nil.
func NewAuthHandler(m *auth.Manager, store *auth.TokenStore, brute *auth.BruteProtector, loginLimiter *ratelimit.Limiter, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		manager:      m,
		tokenStore:   store,
		brute:        brute,
		loginLimiter: loginLimiter,
		logger:       logger,
	}
}

// Login implements POST /api/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	var req types.LoginRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	ip := middleware.RemoteIPFromContext(r.Context())
	if !h.checkLoginLimiter(w, traceID, ip, req.Username) {
		return
	}

	res, err := h.manager.Login(r.Context(), req.Username, req.Password, ip, r.UserAgent())
	if err != nil {
		h.respondLoginError(w, traceID, err)
		return
	}
	if res.TOTPRequired {
		util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PendingTOTPResponse]{
			Data: types.PendingTOTPResponse{
				PendingToken: res.PendingToken,
				ExpiresIn:    int(auth.PendingTokenTTL.Seconds()),
				TOTPRequired: true,
			},
			RequestID: traceID,
		})
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.LoginResponse]{
		Data: types.LoginResponse{
			AccessToken: res.AccessToken,
			ExpiresAt:   res.ExpiresAt.UnixMilli(),
			User:        userRecordToPublic(res.User),
		},
		RequestID: traceID,
	})
}

// VerifyTOTP implements POST /api/auth/verify-totp.
func (h *AuthHandler) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	var req types.VerifyTOTPRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	res, err := h.manager.VerifyTOTP(r.Context(), req.PendingToken, req.Code)
	if err != nil {
		h.respondVerifyError(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.LoginResponse]{
		Data: types.LoginResponse{
			AccessToken: res.AccessToken,
			ExpiresAt:   res.ExpiresAt.UnixMilli(),
			User:        userRecordToPublic(res.User),
		},
		RequestID: traceID,
	})
}

// VerifyRecovery implements POST /api/auth/verify-recovery.
func (h *AuthHandler) VerifyRecovery(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	var req types.VerifyRecoveryRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	res, remaining, err := h.manager.VerifyRecovery(r.Context(), req.PendingToken, req.Code)
	if err != nil {
		h.respondVerifyError(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[recoveryLoginResponse]{
		Data: recoveryLoginResponse{
			LoginResponse: types.LoginResponse{
				AccessToken: res.AccessToken,
				ExpiresAt:   res.ExpiresAt.UnixMilli(),
				User:        userRecordToPublic(res.User),
			},
			RecoveryCodesRemaining: remaining,
		},
		RequestID: traceID,
	})
}

// recoveryLoginResponse extends LoginResponse with the leftover code count so
// the frontend can warn the user to regenerate when only a couple remain.
type recoveryLoginResponse struct {
	types.LoginResponse
	RecoveryCodesRemaining int `json:"recovery_codes_remaining"`
}

// Refresh implements POST /api/auth/refresh. It does not mint a new token —
// the sliding expiry is updated transparently by TokenStore.Lookup whenever
// more than half the TTL has elapsed. The endpoint exists so clients can
// explicitly probe whether their token is still valid (e.g. on app focus).
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "no user", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.UserPublicProfile]{
		Data:      userRecordToPublic(user),
		RequestID: traceID,
	})
}

// Logout implements POST /api/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	token, ok := bearerFromHeader(r)
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "missing token", nil, traceID)
		return
	}
	if err := h.manager.Logout(r.Context(), token); err != nil && !errors.Is(err, auth.ErrSessionNotFound) {
		util.RespondError(w, types.ErrInternalUnknown, "logout failed", nil, traceID)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// checkLoginLimiter enforces the per-IP/username login bucket. Returns true
// when the request may proceed.
func (h *AuthHandler) checkLoginLimiter(w http.ResponseWriter, traceID, ip, username string) bool {
	if h.loginLimiter != nil {
		key := ip + "|" + username
		if ok, _ := h.loginLimiter.Allow(key); !ok {
			util.RespondError(w, types.ErrAuthRateLimited,
				"too many login attempts", nil, traceID)
			return false
		}
	}
	if h.brute != nil {
		if blocked, _ := h.brute.IsBlocked(ip, username); blocked {
			util.RespondError(w, types.ErrAuthBruteForceBlocked,
				"too many failed attempts", nil, traceID)
			return false
		}
	}
	return true
}

// respondLoginError maps a Manager.Login error into the canonical envelope.
func (h *AuthHandler) respondLoginError(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		util.RespondError(w, types.ErrAuthInvalidPassword,
			"invalid username or password", nil, traceID)
	case errors.Is(err, auth.ErrAccountDisabled):
		util.RespondError(w, types.ErrAuthUserInactive,
			"account disabled", nil, traceID)
	case errors.Is(err, auth.ErrBruteForceBlocked):
		util.RespondError(w, types.ErrAuthBruteForceBlocked,
			"too many failed attempts", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("login failed", slog.String("err", err.Error()),
				slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalUnknown, "login failed", nil, traceID)
	}
}

// respondVerifyError maps a verify-totp / verify-recovery error.
func (h *AuthHandler) respondVerifyError(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, auth.ErrPendingTokenInvalid):
		util.RespondError(w, types.ErrAuthTokenInvalid,
			"pending token invalid or expired", nil, traceID)
	case errors.Is(err, auth.ErrTOTPInvalid), errors.Is(err, auth.ErrTOTPNotEnabled):
		util.RespondError(w, types.ErrAuthTOTPInvalid,
			"invalid totp code", nil, traceID)
	case errors.Is(err, auth.ErrRecoveryCodeInvalid):
		util.RespondError(w, types.ErrAuthRecoveryCodeInvalid,
			"invalid recovery code", nil, traceID)
	case errors.Is(err, auth.ErrRecoveryExhausted):
		util.RespondError(w, types.ErrAuthRecoveryExhausted,
			"all recovery codes consumed", nil, traceID)
	case errors.Is(err, auth.ErrAccountDisabled):
		util.RespondError(w, types.ErrAuthUserInactive,
			"account disabled", nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("verify failed", slog.String("err", err.Error()),
				slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalUnknown, "verification failed", nil, traceID)
	}
}

// bearerFromHeader extracts "Bearer <token>" from r without importing the
// auth package (which lives at a higher level in the dependency graph).
func bearerFromHeader(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	v := r.Header.Get("Authorization")
	if len(v) <= len(prefix) || v[:len(prefix)] != prefix {
		return "", false
	}
	tok := v[len(prefix):]
	if tok == "" {
		return "", false
	}
	return tok, true
}

// userRecordToPublic converts the storage projection into the wire DTO.
func userRecordToPublic(u *storage.UserRecord) types.UserPublicProfile {
	if u == nil {
		return types.UserPublicProfile{}
	}
	return types.UserPublicProfile{
		ID:          u.ID,
		Username:    u.Username,
		Role:        types.UserRole(u.Role),
		Email:       u.Email,
		Locale:      u.Locale,
		TOTPEnabled: u.TOTPEnabled,
		CreatedAt:   u.CreatedAt,
	}
}

// userRecordToFull converts to the full User DTO (admin views).
func userRecordToFull(u *storage.UserRecord) types.User {
	if u == nil {
		return types.User{}
	}
	return types.User{
		ID:          u.ID,
		Username:    u.Username,
		Role:        types.UserRole(u.Role),
		IsActive:    u.IsActive,
		Email:       u.Email,
		Locale:      u.Locale,
		TOTPEnabled: u.TOTPEnabled,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
