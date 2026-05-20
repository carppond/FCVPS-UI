package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// ctxKey is an unexported type used for context keys so other packages can't
// accidentally collide.
type ctxKey string

const (
	// CtxKeyUser stores the resolved *storage.UserRecord under the request
	// context once the auth middleware has succeeded.
	CtxKeyUser ctxKey = "auth.user"

	// CtxKeySessionID stores the current sessions.id so handlers can target
	// the active session for revocation (e.g. self-logout).
	CtxKeySessionID ctxKey = "auth.session_id"
)

// UserFromContext returns the authenticated user attached to ctx by Required
// / RequirePending2FA, plus an "ok" flag.
func UserFromContext(ctx context.Context) (*storage.UserRecord, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(CtxKeyUser).(*storage.UserRecord)
	return v, ok
}

// MustUserFromContext panics when no user is attached. Use only in handlers
// downstream of Required / RequireAdmin where the contract guarantees a user.
func MustUserFromContext(ctx context.Context) *storage.UserRecord {
	u, ok := UserFromContext(ctx)
	if !ok {
		panic("auth: no user in context")
	}
	return u
}

// SessionIDFromContext returns the session ID stored by Required.
func SessionIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(CtxKeySessionID).(string); ok {
		return v
	}
	return ""
}

// Required is the standard middleware for routes that need a logged-in user.
// It strips "Bearer <token>" from the Authorization header, resolves the
// session, and either forwards with the user injected into ctx or returns
// 401 ERR_AUTH_TOKEN_INVALID.
func Required(store *TokenStore) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := extractBearerToken(r)
			if !ok {
				respondUnauthorized(w, r)
				return
			}
			result, err := store.Lookup(r.Context(), token)
			if err != nil {
				if errors.Is(err, ErrAccountDisabled) {
					util.RespondError(w, types.ErrAuthUserInactive,
						"account disabled", nil,
						middleware.TraceIDFromContext(r.Context()))
					return
				}
				respondUnauthorized(w, r)
				return
			}
			r = injectUser(r, result.User, result.SessionID)
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin extends Required by checking role=admin. Non-admin callers get
// 403 ERR_AUTH_FORBIDDEN.
func RequireAdmin(store *TokenStore) middleware.Middleware {
	base := Required(store)
	return func(next http.Handler) http.Handler {
		return base(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || user.Role != string(types.RoleAdmin) {
				util.RespondError(w, types.ErrAuthForbidden,
					"admin role required", nil,
					middleware.TraceIDFromContext(r.Context()))
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// RequirePending2FA validates that the supplied bearer token is in the
// pending_2fa state. Used for /api/auth/verify-totp etc.
func RequirePending2FA(store *TokenStore) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := extractBearerToken(r)
			if !ok {
				respondUnauthorized(w, r)
				return
			}
			result, err := store.LookupPending(r.Context(), token)
			if err != nil {
				respondUnauthorized(w, r)
				return
			}
			r = injectUser(r, result.User, result.SessionID)
			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken pulls the value out of Authorization: Bearer <token>.
// Empty / malformed headers return ("", false).
func extractBearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}
	if !strings.HasPrefix(auth, prefix) {
		return "", false
	}
	token := strings.TrimSpace(auth[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

// injectUser stores the resolved user in the request context (under
// CtxKeyUser) and also propagates the user ID into middleware.CtxKeyUserID
// so the logger / audit middleware can pick it up.
func injectUser(r *http.Request, user *storage.UserRecord, sessionID string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, CtxKeyUser, user)
	ctx = context.WithValue(ctx, CtxKeySessionID, sessionID)
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, user.ID)
	return r.WithContext(ctx)
}

// respondUnauthorized centralises the 401 response shape.
func respondUnauthorized(w http.ResponseWriter, r *http.Request) {
	util.RespondError(w, types.ErrAuthTokenInvalid,
		"authentication required", nil,
		middleware.TraceIDFromContext(r.Context()))
}
