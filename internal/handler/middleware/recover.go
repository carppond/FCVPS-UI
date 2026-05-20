package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// Recover catches panics emitted by downstream handlers and converts them into
// a uniform 500 response so a single bug cannot tear down the whole server.
//
// In production (devMode = false) the stack trace is logged but NOT returned
// to the client to avoid leaking internal paths. In dev mode the stack is
// attached to the response `details` field so engineers can inspect it
// without tailing the log file.
//
// The middleware must be the outermost wrapper in the chain: it can only
// observe panics raised by middlewares it itself wraps.
func Recover(logger *slog.Logger, devMode bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				stack := debug.Stack()
				traceID := TraceIDFromContext(r.Context())
				if logger != nil {
					logger.Error("http panic recovered",
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
						slog.String("trace_id", traceID),
						slog.Any("panic", rec),
						slog.String("stack", string(stack)),
					)
				}
				var details any
				if devMode {
					details = map[string]any{
						"panic": toString(rec),
						"stack": string(stack),
					}
				}
				util.RespondError(w, types.ErrInternalUnknown,
					"internal server error", details, traceID)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// toString renders an arbitrary recover() value into a safe display string.
func toString(v any) string {
	switch x := v.(type) {
	case error:
		return x.Error()
	case string:
		return x
	default:
		// Avoid fmt.Sprintf import cycle scares by keeping the cast simple.
		return "non-error panic"
	}
}
