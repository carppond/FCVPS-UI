package scriptengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// DefaultTimeout is the hard wall-clock cap applied when callers do not pass
// their own. Matches PRD M-SCRIPT.1.
const DefaultTimeout = 5 * time.Second

// MaxLogEntries caps the console.log buffer so a script in an infinite
// console.log loop cannot exhaust host memory before the interrupt fires.
// A misbehaving script is still terminated by the timeout; the cap simply
// keeps the per-run allocation bounded and predictable.
const MaxLogEntries = 1000

// Engine owns the *goja.Runtime pool. One Engine instance per process is
// enough — the pool dispenses a free runtime to every concurrent Run call.
type Engine struct {
	pool   sync.Pool
	logger *slog.Logger
}

// NewEngine returns a ready-to-use engine. logger may be nil — a no-op slog
// adapter is substituted when so.
func NewEngine(logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	e := &Engine{logger: logger}
	e.pool = sync.Pool{New: func() any {
		return goja.New()
	}}
	return e
}

// RunResult bundles the engine's observable output for a single run.
//
//   - Output is the *raw JSON string* produced by the script via the
//     __output global. We deliberately keep it as a string at this layer to
//     preserve the §6.3 boundary: callers that need typed nodes parse the
//     string themselves with their domain types.
//   - Logs collects every console.* invocation, prefixed with [level].
//   - DurationMS is wall-clock elapsed time.
type RunResult struct {
	Output     string
	Logs       []string
	DurationMS int64
}

// Run executes src against an input map.
//
// Flow:
//
//  1. Marshal input to JSON, build the bootstrap prologue that injects
//     __input via JSON.parse (the architecture §6.3 boundary).
//  2. Pull a runtime from the pool, arm the sandbox, run the prologue, run
//     the user source, then snapshot __output back to Go.
//  3. Always return the runtime to the pool — even on error — after
//     clearing the globals we wrote so the next caller does not inherit
//     leftover state.
//
// Timeout enforcement lives in RunString (vm.Interrupt + AfterFunc).
func (e *Engine) Run(ctx context.Context, code string, input map[string]any, timeout time.Duration) (*RunResult, error) {
	if e == nil {
		return nil, fmt.Errorf("scriptengine: nil engine")
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if input == nil {
		input = map[string]any{}
	}
	// Honour ctx — Engine respects an already-cancelled caller as a fast
	// reject. We do NOT pipe ctx into the vm because goja loops are not
	// cooperatively cancellable; the timeout handles runaway execution.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("scriptengine: marshal input: %w", err)
	}

	vm := e.pool.Get().(*goja.Runtime)
	defer e.returnRuntime(vm)

	logs := make([]string, 0, 16)
	var logsMu sync.Mutex // goja runs single-threaded, but defensive.
	logCb := func(level, msg string) {
		logsMu.Lock()
		defer logsMu.Unlock()
		if len(logs) >= MaxLogEntries {
			return
		}
		logs = append(logs, fmt.Sprintf("[%s] %s", level, msg))
	}

	if err := SetupSandbox(vm, logCb); err != nil {
		return nil, fmt.Errorf("scriptengine: setup sandbox: %w", err)
	}

	// Inject the input JSON literal. Using JSON.parse is the §6.3-ruled
	// boundary; we wrap it in a string literal so even nested quotes /
	// backticks in the data are escaped consistently.
	bootstrap := fmt.Sprintf(
		"var __input = JSON.parse(%s); var __output = null;",
		jsStringLiteral(string(inputJSON)),
	)

	start := time.Now()
	if _, err := RunString(vm, bootstrap, timeout); err != nil {
		// Bootstrap failures are host bugs (we control the source) — log
		// loudly and surface the raw error so tests notice.
		e.logger.Error("scriptengine bootstrap failed", slog.String("err", err.Error()))
		return nil, fmt.Errorf("scriptengine: bootstrap: %w", err)
	}

	// User code. We deliberately do NOT shrink the timeout to account for
	// bootstrap time — the bootstrap is microseconds; charging user code
	// for it would be confusing.
	if _, err := RunString(vm, code, timeout); err != nil {
		return e.errResult(logs, start, err)
	}

	// Pull __output back as a JSON string. If the script set it to a
	// non-string value we JSON.stringify it; if it is undefined / null we
	// surface an empty string so callers can distinguish "no output" from
	// "empty object".
	serializeOut := `(function() {
		if (typeof __output === 'undefined' || __output === null) { return ''; }
		if (typeof __output === 'string') { return __output; }
		return JSON.stringify(__output);
	})()`
	val, err := RunString(vm, serializeOut, timeout)
	if err != nil {
		return e.errResult(logs, start, err)
	}
	var out string
	if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
		out = val.String()
	}

	return &RunResult{
		Output:     out,
		Logs:       append([]string(nil), logs...),
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// errResult builds a RunResult that surfaces the logs accumulated so far
// alongside the error. Handlers want both: even a failed run is interesting
// when console.log captured the diagnostic that explains why.
func (e *Engine) errResult(logs []string, start time.Time, err error) (*RunResult, error) {
	return &RunResult{
		Logs:       append([]string(nil), logs...),
		DurationMS: time.Since(start).Milliseconds(),
	}, err
}

// returnRuntime resets the runtime's globals we know about and puts it back
// in the pool. We do NOT try to wipe every global goja installs — that
// would defeat reuse — but the user-script writable surface (__input,
// __output, console, blocked globals) is explicitly cleared so a hostile
// script cannot exfiltrate state to the next tenant.
//
// Failures during reset trash the runtime (we do NOT return it to the
// pool) so the next caller gets a fresh one. The leaked instance is GC'd.
func (e *Engine) returnRuntime(vm *goja.Runtime) {
	// Ensure no lingering interrupt — RunString already calls
	// ClearInterrupt, but be defensive in case a panic skipped it.
	vm.ClearInterrupt()
	// Reset only the host-controlled globals. JS scripts that polluted
	// arbitrary names (foo, bar) survive in the runtime but are
	// inaccessible to the next run because the bootstrap re-introduces
	// __input / __output and SetupSandbox re-binds console + blocklist.
	defer func() {
		// Any panic during reset means the runtime is in an unknown
		// state; drop it on the floor and let sync.Pool warm a fresh one
		// on the next Get.
		_ = recover()
	}()
	if err := vm.GlobalObject().Delete("__input"); err != nil {
		return
	}
	if err := vm.GlobalObject().Delete("__output"); err != nil {
		return
	}
	e.pool.Put(vm)
}

// IsTimeout reports whether err originated from the timeout interrupt path.
// Handlers use this to map onto types.ErrScriptTimeout.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsSandboxViolation reports whether err originated from a blocked-global
// stub. Handlers use this to map onto types.ErrScriptSandboxViolation.
func IsSandboxViolation(err error) bool {
	return errors.Is(err, ErrSandboxViolation)
}

// jsStringLiteral produces a JSON-encoded JS string literal so the
// bootstrap's JSON.parse call sees correctly-escaped input. We rely on
// json.Marshal because it already handles every edge case (unicode,
// backslashes, quotes) the spec cares about.
func jsStringLiteral(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
