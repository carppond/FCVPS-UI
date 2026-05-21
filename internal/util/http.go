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
	types.ErrConflictLastAdmin:       http.StatusConflict,

	// PIPELINE
	types.ErrPipelineOperatorUnknown: http.StatusUnprocessableEntity,
	types.ErrPipelineOperatorParams:  http.StatusUnprocessableEntity,
	types.ErrPipelineRunTimeout:      http.StatusRequestTimeout,

	// SCRIPT
	types.ErrScriptTimeout:          http.StatusRequestTimeout,
	types.ErrScriptSandboxViolation: http.StatusUnprocessableEntity,
	types.ErrScriptRuntimeError:     http.StatusUnprocessableEntity,

	// AGENT
	types.ErrAgentTokenInvalid:       http.StatusNotFound,
	types.ErrAgentOffline:            http.StatusConflict,
	types.ErrAgentCommandTimeout:     http.StatusRequestTimeout,
	types.ErrAgentVersionUnsupported: http.StatusUpgradeRequired, // 426, §1.8

	// INTERNAL
	types.ErrInternalDatabase: http.StatusInternalServerError,
	types.ErrInternalUnknown:  http.StatusInternalServerError,
}

// StatusRecorder wraps an http.ResponseWriter and remembers the status code
// and number of bytes written so middleware (logging, audit) can observe the
// response after the handler has run. The zero value defaults Status to 200,
// matching Go's stdlib behaviour when a handler writes a body without first
// calling WriteHeader.
type StatusRecorder struct {
	http.ResponseWriter
	Status    int
	BytesSent int64
	written   bool
}

// NewStatusRecorder wraps w. Pass the result down the middleware chain in
// place of the original ResponseWriter.
func NewStatusRecorder(w http.ResponseWriter) *StatusRecorder {
	return &StatusRecorder{ResponseWriter: w, Status: http.StatusOK}
}

// WriteHeader captures the status code and forwards it to the underlying
// writer. Subsequent calls are no-ops (matching stdlib semantics).
func (r *StatusRecorder) WriteHeader(code int) {
	if r.written {
		return
	}
	r.Status = code
	r.written = true
	r.ResponseWriter.WriteHeader(code)
}

// Write forwards data and tracks the cumulative byte count. If the handler
// did not explicitly call WriteHeader, stdlib auto-emits 200; we mirror that
// flag here so duplicate WriteHeader calls are suppressed.
func (r *StatusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.written = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.BytesSent += int64(n)
	return n, err
}

// Flush forwards to the underlying writer if it implements http.Flusher.
// Required for SSE / streaming handlers wrapped by middleware.
func (r *StatusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// MaxJSONBodyBytes caps the size of any JSON request body decoded through
// DecodeJSONBody. 1 MiB is several orders of magnitude larger than any
// legitimate JSON DTO this project carries (the bulkiest payload is the
// pipeline YAML, ~64 KiB worst case) yet small enough to defeat a hostile
// client trying to OOM the hub by streaming an unbounded body.
//
// Multipart uploads (subscription upload + backup restore) handle their
// own limits in their respective handlers; they do NOT flow through
// DecodeJSONBody.
const MaxJSONBodyBytes = 1 << 20 // 1 MiB

// DecodeJSONBody decodes r.Body into dst, enforcing DisallowUnknownFields and
// a hard size limit of MaxJSONBodyBytes (bug-6 of
// docs/06-review-backend-round1.md). Returns a wrapped error suitable for
// logging; handlers translate it into ErrValidationInvalidFormat.
//
// The size limit is applied via http.MaxBytesReader so the connection is
// closed on overflow — preventing a hostile client from holding a
// goroutine open with a slow stream.
func DecodeJSONBody(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return fmt.Errorf("decode json body: nil request or body")
	}
	// MaxBytesReader accepts a nil ResponseWriter; the connection will not
	// be force-closed on overflow but the read will still error out, which
	// is the important guarantee. Handlers reach in via the request scope
	// only (no shared ResponseWriter is required here, keeping the
	// signature unchanged for the 30+ call sites).
	r.Body = http.MaxBytesReader(nil, r.Body, MaxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode json body: %w", err)
	}
	return nil
}
