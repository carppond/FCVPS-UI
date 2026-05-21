// Package scriptengine wraps the goja JavaScript runtime so the rest of the
// server can run user-supplied scripts (M-SCRIPT) without leaking sandbox
// escapes or runaway loops.
//
// Architecture invariants (see docs/03-architecture.md §3.7 + §6.3):
//
//   - One *goja.Runtime per Run; runtimes come from a sync.Pool so the warm
//     ones avoid the ~1ms VM construction cost.
//   - Data crosses the host/guest boundary as JSON strings (vm.RunString
//     parses them with the built-in JSON.parse). Direct vm.Set of Go slices
//     is forbidden because goja's reflection layer drops nested-map types
//     and renames struct fields (architecture §6.3 ruling).
//   - Wall-clock timeouts go through vm.Interrupt — never ctx.Cancel — since
//     goja loops do not cooperatively check the caller's context.
//
// This file owns the lowest-level primitive: RunString, which is the only
// place that actually invokes vm.RunString and arms the interrupt timer.
package scriptengine

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// ErrTimeout is the sentinel returned when the script ran longer than the
// supplied timeout. PRD M-SCRIPT.1 mandates this be observable as a distinct
// error (the handler maps it to ERR_SCRIPT_TIMEOUT → HTTP 408).
var ErrTimeout = errors.New("scriptengine: execution exceeded timeout")

// ErrSandboxViolation is returned when user code tried to touch a globally
// blocked symbol (require / fetch / process / fs / eval). The injected stub
// throws a JS error whose message includes the prefix "sandbox:"; we detect
// that prefix when unwrapping goja.Exception and surface this sentinel so the
// handler can map it to ERR_SCRIPT_SANDBOX_VIOLATION → HTTP 422.
var ErrSandboxViolation = errors.New("scriptengine: sandbox violation")

// timeoutSignal is the value passed to vm.Interrupt. goja preserves it inside
// the returned *goja.InterruptedError, which lets us distinguish a deliberate
// kill from a host-side panic.
const timeoutSignal = "scriptengine.timeout"

// sandboxViolationPrefix is the leading marker inside the JS Error.message
// that SetupSandbox uses for every blocked-symbol stub.  We grep for it in
// the returned exception to map back to ErrSandboxViolation.
const sandboxViolationPrefix = "sandbox:"

// RunString executes src on vm with a hard wall-clock cap. The vm is left in
// whatever state the script produced — callers (Engine.Run) are responsible
// for resetting / returning the runtime to the pool.
//
// Lifecycle:
//
//  1. time.AfterFunc fires vm.Interrupt(timeoutSignal) when the deadline
//     elapses. goja then raises a *goja.InterruptedError from the next
//     bytecode instruction, breaking out of any infinite loop.
//  2. We always call timer.Stop() before returning so completed scripts do
//     not leak goroutines waiting on the timer.
//  3. The vm.ClearInterrupt() call after Stop() is a belt-and-braces reset
//     so the same vm can be reused (the pool reuses runtimes across users).
func RunString(vm *goja.Runtime, src string, timeout time.Duration) (goja.Value, error) {
	if vm == nil {
		return nil, fmt.Errorf("scriptengine: nil runtime")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("scriptengine: non-positive timeout")
	}
	timer := time.AfterFunc(timeout, func() {
		vm.Interrupt(timeoutSignal)
	})
	defer func() {
		timer.Stop()
		vm.ClearInterrupt()
	}()
	val, err := vm.RunString(src)
	if err != nil {
		return nil, classifyRunError(err)
	}
	return val, nil
}

// classifyRunError maps goja's heterogenous error types onto the package's
// sentinels. We unwrap *goja.InterruptedError for the timeout case and
// inspect *goja.Exception messages for the sandbox-stub prefix; everything
// else is passed through wrapped so caller logs preserve the JS stack trace.
func classifyRunError(err error) error {
	var intr *goja.InterruptedError
	if errors.As(err, &intr) {
		if intr.Value() == timeoutSignal {
			return ErrTimeout
		}
		// Some other host called Interrupt with a custom value — surface a
		// generic timeout-ish error so we never silently swallow it.
		return fmt.Errorf("scriptengine: interrupted: %v", intr.Value())
	}
	var exc *goja.Exception
	if errors.As(err, &exc) {
		msg := exc.Value().String()
		// The sandbox marker may show up either at the start of the
		// message (when the JS error is created directly) or partway
		// through (when goja wraps it as "GoError: sandbox: ..."). We
		// substring-match on the prefix so both shapes route to the
		// canonical sentinel.
		if strings.Contains(msg, sandboxViolationPrefix) {
			return fmt.Errorf("%w: %s", ErrSandboxViolation, msg)
		}
		return fmt.Errorf("scriptengine: js exception: %s", msg)
	}
	return fmt.Errorf("scriptengine: run: %w", err)
}
