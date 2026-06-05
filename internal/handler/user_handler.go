package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// UserHandler hosts the /api/me/* and /api/admin/users/* endpoints.
type UserHandler struct {
	manager  *auth.Manager
	users    *storage.UserRepo
	sessions *storage.SessionRepo
	totp     auth.TOTPManager
	logger   *slog.Logger
}

// NewUserHandler wires the handler. sessions may be nil; the session endpoints
// degrade to "not implemented" when nil so tests can omit it.
func NewUserHandler(m *auth.Manager, users *storage.UserRepo, sessions *storage.SessionRepo, totp auth.TOTPManager, logger *slog.Logger) *UserHandler {
	return &UserHandler{manager: m, users: users, sessions: sessions, totp: totp, logger: logger}
}

// Me implements GET /api/me.
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		util.RespondError(w, types.ErrAuthTokenInvalid, "no user", nil, traceID)
		return
	}
	// 上下文里的用户来自 token 校验的 60s LRU 缓存(auth/token_store.go)——
	// PATCH /api/me 改完资料立刻 GET 会拿到旧快照,这里始终回源数据库。
	if h.users != nil {
		if fresh, err := h.users.GetByID(r.Context(), user.ID); err == nil {
			user = fresh
		}
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.UserPublicProfile]{
		Data:      userRecordToPublic(user),
		RequestID: traceID,
	})
}

// UpdateMe implements PATCH /api/me.
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.UpdateMeRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if err := h.users.UpdateProfile(r.Context(), user.ID, req.Username, req.Email, req.Locale); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	updated, err := h.users.GetByID(r.Context(), user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.UserPublicProfile]{
		Data:      userRecordToPublic(updated),
		RequestID: traceID,
	})
}

// ChangePassword implements POST /api/me/password.
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.ChangePasswordRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if err := h.manager.ChangePassword(r.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			util.RespondError(w, types.ErrAuthInvalidPassword, "old password incorrect", nil, traceID)
		case errors.Is(err, auth.ErrPasswordTooWeak):
			util.RespondError(w, types.ErrValidationOutOfRange, err.Error(), nil, traceID)
		default:
			h.respondStorageErr(w, traceID, err)
		}
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// DeleteMe implements DELETE /api/me. Requires the password in the body.
//
// Safety rail: if the caller is the last role=admin account the request is
// rejected with ErrConflictLastAdmin so the install always retains at least
// one administrator (mirrors AdminDeleteUser; see Bug-2 in
// docs/06-review-backend-round1.md).
func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req struct {
		Password string `json:"password"`
	}
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		util.RespondError(w, types.ErrAuthInvalidPassword, "password incorrect", nil, traceID)
		return
	}
	if user.Role == string(types.RoleAdmin) {
		count, err := h.users.CountAdmins(r.Context())
		if err != nil {
			h.respondStorageErr(w, traceID, err)
			return
		}
		if count <= 1 {
			util.RespondError(w, types.ErrConflictLastAdmin,
				"cannot delete the last admin account", nil, traceID)
			return
		}
	}
	if err := h.manager.DeleteUser(r.Context(), user.ID); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// TOTPSetup implements POST /api/me/totp/setup (per contract this is also
// exposed as GET in the docs; we mount it as POST to keep the no-CSRF
// requirement when the client uses a state-changing call).
func (h *UserHandler) TOTPSetup(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	setup, err := h.totp.Setup(r.Context(), user.ID, user.Username)
	if err != nil {
		if errors.Is(err, auth.ErrTOTPAlreadyEnabled) {
			util.RespondError(w, types.ErrAuthForbidden, "totp already enabled", nil, traceID)
			return
		}
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.TOTPSetupResponse]{
		Data: types.TOTPSetupResponse{
			Secret:     setup.Secret,
			OTPAuthURI: setup.OTPAuthURI,
			QRCodeURL:  setup.QRCodeBase64,
		},
		RequestID: traceID,
	})
}

// TOTPEnable implements POST /api/me/totp/enable.
func (h *UserHandler) TOTPEnable(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.EnableTOTPRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	codes, err := h.totp.Enable(r.Context(), user.ID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrTOTPInvalid):
			util.RespondError(w, types.ErrAuthTOTPInvalid, "invalid totp code", nil, traceID)
		case errors.Is(err, auth.ErrTOTPAlreadyEnabled):
			util.RespondError(w, types.ErrAuthForbidden, "already enabled", nil, traceID)
		default:
			h.respondStorageErr(w, traceID, err)
		}
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.EnableTOTPResponse]{
		Data:      types.EnableTOTPResponse{BackupCodes: codes},
		RequestID: traceID,
	})
}

// TOTPDisable implements POST /api/me/totp/disable.
func (h *UserHandler) TOTPDisable(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.DisableTOTPRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if err := h.totp.Disable(r.Context(), user.ID, req.Password); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			util.RespondError(w, types.ErrAuthInvalidPassword, "password incorrect", nil, traceID)
		case errors.Is(err, auth.ErrTOTPNotEnabled):
			util.RespondError(w, types.ErrAuthForbidden, "totp not enabled", nil, traceID)
		default:
			h.respondStorageErr(w, traceID, err)
		}
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// RegenerateRecoveryCodes implements POST /api/me/totp/recovery-codes.
func (h *UserHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req struct {
		Password string `json:"password"`
	}
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	codes, err := h.manager.RegenerateRecoveryCodes(r.Context(), user.ID, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			util.RespondError(w, types.ErrAuthInvalidPassword, "password incorrect", nil, traceID)
		case errors.Is(err, auth.ErrTOTPNotEnabled):
			util.RespondError(w, types.ErrAuthForbidden, "totp not enabled", nil, traceID)
		default:
			h.respondStorageErr(w, traceID, err)
		}
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.EnableTOTPResponse]{
		Data:      types.EnableTOTPResponse{BackupCodes: codes},
		RequestID: traceID,
	})
}

// AdminListUsers implements GET /api/admin/users.
func (h *UserHandler) AdminListUsers(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	page := util.ParsePaginationQuery(r)
	users, total, err := h.users.List(r.Context(), storage.UserListOptions{
		Page:     page.Page,
		PageSize: page.PageSize,
		Keyword:  r.URL.Query().Get("keyword"),
		Role:     r.URL.Query().Get("role"),
	})
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	items := make([]types.User, len(users))
	for i := range users {
		items[i] = userRecordToFull(&users[i])
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.PagedResponse[types.User]]{
		Data: types.PagedResponse[types.User]{
			Items:    items,
			Total:    total,
			Page:     page.Page,
			PageSize: page.PageSize,
		},
		RequestID: traceID,
	})
}

// AdminCreateUser implements POST /api/admin/users.
func (h *UserHandler) AdminCreateUser(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	var req types.CreateUserRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	user, plain, err := h.manager.CreateUser(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUsernameTaken):
			util.RespondError(w, types.ErrConflictUsername, "username taken", nil, traceID)
		case errors.Is(err, auth.ErrPasswordTooWeak):
			util.RespondError(w, types.ErrValidationOutOfRange, err.Error(), nil, traceID)
		default:
			h.respondStorageErr(w, traceID, err)
		}
		return
	}
	resp := struct {
		types.User
		Password string `json:"password,omitempty"`
	}{
		User:     userRecordToFull(user),
		Password: plain,
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{Data: resp, RequestID: traceID})
}

// AdminGetUser implements GET /api/admin/users/:id.
func (h *UserHandler) AdminGetUser(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	id := r.PathValue("id")
	user, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.User]{
		Data:      userRecordToFull(user),
		RequestID: traceID,
	})
}

// AdminUpdateUser implements PATCH /api/admin/users/:id.
func (h *UserHandler) AdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	id := r.PathValue("id")
	var req types.UpdateUserRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Username != "" || req.Email != "" {
		if err := h.users.UpdateProfile(r.Context(), id, req.Username, req.Email, ""); err != nil {
			h.respondStorageErr(w, traceID, err)
			return
		}
	}
	if req.Role != "" {
		role := strings.ToLower(string(req.Role))
		if role != string(types.RoleAdmin) && role != string(types.RoleUser) {
			util.RespondError(w, types.ErrValidationInvalidFormat, "invalid role", nil, traceID)
			return
		}
		if err := h.users.UpdateRole(r.Context(), id, role); err != nil {
			h.respondStorageErr(w, traceID, err)
			return
		}
	}
	if req.IsActive != nil {
		if err := h.users.SetActive(r.Context(), id, *req.IsActive); err != nil {
			h.respondStorageErr(w, traceID, err)
			return
		}
	}
	updated, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.User]{
		Data:      userRecordToFull(updated),
		RequestID: traceID,
	})
}

// AdminDeleteUser implements DELETE /api/admin/users/:id.
//
// Two safety rails:
//   - the caller may not delete their own account (admins routinely have only
//     their own session active; accidental self-delete would lock the install
//     out).
//   - deleting the last remaining admin user is rejected so the install always
//     retains at least one admin to manage the rest.
func (h *UserHandler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	id := r.PathValue("id")
	caller := auth.MustUserFromContext(r.Context())
	if caller.ID == id {
		util.RespondError(w, types.ErrAuthForbidden,
			"cannot delete your own admin account", nil, traceID)
		return
	}
	target, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	if target.Role == string(types.RoleAdmin) {
		count, err := h.users.CountAdmins(r.Context())
		if err != nil {
			h.respondStorageErr(w, traceID, err)
			return
		}
		if count <= 1 {
			util.RespondError(w, types.ErrConflictLastAdmin,
				"cannot delete the last admin user", nil, traceID)
			return
		}
	}
	if err := h.manager.DeleteUser(r.Context(), id); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// AdminResetPassword implements POST /api/admin/users/:id/reset-password.
func (h *UserHandler) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	id := r.PathValue("id")
	plain, err := h.manager.ResetPassword(r.Context(), id)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.ResetPasswordResponse]{
		Data:      types.ResetPasswordResponse{NewPassword: plain},
		RequestID: traceID,
	})
}

// AdminDisableTOTP implements POST /api/admin/users/:id/disable-2fa.
func (h *UserHandler) AdminDisableTOTP(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.manager.AdminForceDisable2FA(r.Context(), id); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// AdminRevokeSessions implements POST /api/admin/users/:id/revoke-sessions.
func (h *UserHandler) AdminRevokeSessions(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.manager.AdminRevokeSessions(r.Context(), id); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// ListMySessions implements GET /api/me/sessions.
func (h *UserHandler) ListMySessions(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if h.sessions == nil {
		util.RespondError(w, types.ErrInternalUnknown, "sessions repo unavailable", nil, traceID)
		return
	}
	rows, err := h.sessions.ListByUser(r.Context(), user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	out := make([]types.Session, 0, len(rows))
	for _, rec := range rows {
		out = append(out, types.Session{
			ID:         rec.ID,
			UserID:     rec.UserID,
			IP:         rec.IP,
			UserAgent:  rec.UserAgent,
			LastUsedAt: rec.LastUsedAt,
			ExpiresAt:  rec.ExpiresAt,
			CreatedAt:  rec.CreatedAt,
		})
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[[]types.Session]{
		Data: out, RequestID: traceID,
	})
}

// RevokeMySession implements DELETE /api/me/sessions/{id}.
//
// The session is identified by its row id (not token hash); users can only
// revoke their own sessions — others 404.
func (h *UserHandler) RevokeMySession(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	if h.sessions == nil {
		util.RespondError(w, types.ErrInternalUnknown, "sessions repo unavailable", nil, traceID)
		return
	}
	target := r.PathValue("id")
	rows, err := h.sessions.ListByUser(r.Context(), user.ID)
	if err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	var tokenHash string
	for _, rec := range rows {
		if rec.ID == target {
			tokenHash = rec.TokenHash
			break
		}
	}
	if tokenHash == "" {
		util.RespondError(w, types.ErrNotFoundUser, "session not found", nil, traceID)
		return
	}
	if err := h.sessions.Delete(r.Context(), tokenHash); err != nil {
		h.respondStorageErr(w, traceID, err)
		return
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[any]{RequestID: traceID})
}

// respondStorageErr translates storage / auth errors into the API envelope.
func (h *UserHandler) respondStorageErr(w http.ResponseWriter, traceID string, err error) {
	switch {
	case errors.Is(err, storage.ErrUserNotFound), errors.Is(err, auth.ErrUserNotFound):
		util.RespondError(w, types.ErrNotFoundUser, "user not found", nil, traceID)
	case errors.Is(err, storage.ErrUsernameTaken), errors.Is(err, auth.ErrUsernameTaken):
		util.RespondError(w, types.ErrConflictUsername, "username taken", nil, traceID)
	case errors.Is(err, auth.ErrPasswordTooWeak):
		util.RespondError(w, types.ErrValidationOutOfRange, err.Error(), nil, traceID)
	default:
		if h.logger != nil {
			h.logger.Error("user handler failed",
				slog.String("err", err.Error()), slog.String("trace_id", traceID))
		}
		util.RespondError(w, types.ErrInternalUnknown, "internal error", nil, traceID)
	}
}
