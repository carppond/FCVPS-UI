package util

import (
	"encoding/json"
	"fmt"
	"net/http"

	"shiguang-vps/internal/types"
)

// RespondJSON writes status + data as a JSON-encoded body. Sets the
// Content-Type header. data may be nil (status only).
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		// Body is already partially written; logging here would create a
		// circular dependency (util → logger → ...). Caller-side middleware
		// owns observability for write errors.
		_ = err
	}
}

// RespondError writes the canonical error envelope with the HTTP status
// derived from ErrorCodeToHTTP(code). requestID may be empty.
func RespondError(w http.ResponseWriter, code types.ErrorCode, message string, details any, requestID string) {
	status := ErrorCodeToHTTP(code)
	body := types.APIResponse[any]{
		Code:      string(code),
		Message:   message,
		Details:   details,
		RequestID: requestID,
	}
	RespondJSON(w, status, body)
}

// ErrorCodeToHTTP maps the canonical ErrorCode constants from
// internal/types/api.go to their HTTP status codes per docs/04-api-contract.md
// §3. Unknown codes fall back to 500.
func ErrorCodeToHTTP(code types.ErrorCode) int {
	if status, ok := errorStatusMap[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// errorStatusMap is the single source of truth for ErrorCode → HTTP status.
// Adding a new ErrorCode requires a corresponding entry here.
var errorStatusMap = map[types.ErrorCode]int{
	// AUTH
	types.ErrAuthInvalidPassword:     http.StatusUnauthorized,
	types.ErrAuthUserInactive:        http.StatusForbidden,
	types.ErrAuthTOTPRequired:        http.StatusAccepted,
	types.ErrAuthTOTPInvalid:         http.StatusUnauthorized,
	types.ErrAuthTOTPExpired:         http.StatusUnauthorized,
	types.ErrAuthRecoveryCodeInvalid: http.StatusUnauthorized,
	types.ErrAuthRecoveryExhausted:   http.StatusForbidden,
	types.ErrAuthTokenInvalid:        http.StatusUnauthorized,
	types.ErrAuthTokenExpired:        http.StatusUnauthorized,
	types.ErrAuthRateLimited:         http.StatusTooManyRequests,
	types.ErrAuthBruteForceBlocked:   http.StatusTooManyRequests,
	types.ErrAuthForbidden:           http.StatusForbidden,

	// VALIDATION
	types.ErrValidationRequiredField:  http.StatusBadRequest,
	types.ErrValidationInvalidFormat:  http.StatusBadRequest,
	types.ErrValidationOutOfRange:     http.StatusBadRequest,
	types.ErrValidationRegexCompile:   http.StatusBadRequest,
	types.ErrValidationYAMLParse:      http.StatusBadRequest,
	types.ErrValidationSchemaMismatch: http.StatusBadRequest,

	// NOT_FOUND
	types.ErrNotFoundUser:         http.StatusNotFound,
	types.ErrNotFoundSubscription: http.StatusNotFound,
	types.ErrNotFoundNode:         http.StatusNotFound,
	types.ErrNotFoundPipeline:     http.StatusNotFound,
	types.ErrNotFoundRule:         http.StatusNotFound,
	types.ErrNotFoundScript:       http.StatusNotFound,
	types.ErrNotFoundAgent:        http.StatusNotFound,
	types.ErrNotFoundChannel:      http.StatusNotFound,

	// CONFLICT
	types.ErrConflictUsername:        http.StatusConflict,
	types.ErrConflictPipelineVersion: http.StatusConflict,

	// PIPELINE
	types.ErrPipelineOperatorUnknown: http.StatusUnprocessableEntity,
	types.ErrPipelineOperatorParams:  http.StatusUnprocessableEntity,
	types.ErrPipelineRunTimeout:      http.StatusRequestTimeout,

	// SCRIPT
	types.ErrScriptTimeout:          http.StatusRequestTimeout,
	types.ErrScriptSandboxViolation: http.StatusUnprocessableEntity,
	types.ErrScriptRuntimeError:     http.StatusUnprocessableEntity,

	// AGENT
	types.ErrAgentTokenInvalid:   http.StatusNotFound,
	types.ErrAgentOffline:        http.StatusConflict,
	types.ErrAgentCommandTimeout: http.StatusRequestTimeout,

	// INTERNAL
	types.ErrInternalDatabase: http.StatusInternalServerError,
	types.ErrInternalUnknown:  http.StatusInternalServerError,
}

// DecodeJSONBody decodes r.Body into dst, enforcing DisallowUnknownFields.
// Returns a wrapped error suitable for logging; handlers translate it into
// ErrValidationInvalidFormat.
func DecodeJSONBody(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return fmt.Errorf("decode json body: nil request or body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode json body: %w", err)
	}
	return nil
}
