package scriptengine

import (
	"fmt"

	"github.com/dop251/goja"
)

// blockedCallables enumerates symbols user scripts may try to invoke as
// functions (require, fetch, eval, …). We replace each with a stub that
// throws a sandbox-marker error from the call site.
//
// Listing every name explicitly (instead of relying on goja's default empty
// runtime) makes audits trivial: grep this slice to know the full blocklist.
var blockedCallables = []string{
	"require",
	"eval",
	"fetch",
	"importScripts",
	"setTimeout",
	"setInterval",
	"setImmediate",
}

// blockedObjects enumerates symbols user scripts may try to dereference as
// objects (process.exit, fs.readFileSync, …). We replace each with a
// DynamicObject whose Get/Has/Set all throw a sandbox marker — that way
// `process.exit(0)` fails at the property access, not at the call.
var blockedObjects = []string{
	"process",
	"fs",
	"globalThis",
}

// blockedConstructors enumerates symbols user scripts may try to instantiate
// with `new` (XMLHttpRequest, WebSocket). goja recognises functions as
// constructors, so the same throwing-stub pattern as callables works as
// long as the message includes the "sandbox:" marker for classifyRunError.
var blockedConstructors = []string{
	"XMLHttpRequest",
	"WebSocket",
}

// SetupSandbox arms vm with the project's allow-list of capabilities:
//
//   - dangerous globals (require / fetch / process / fs / eval / timers) are
//     replaced by throwing stubs;
//   - console.{log,warn,error,info,debug} is redirected to logCb so the
//     handler can stream stdout back to the UI;
//   - JSON is left intact (it is the ONLY supported boundary; see §6.3).
//
// logCb may be nil — in that case console output is silently discarded
// (useful for benchmarks that do not care about log capture).
//
// The function is safe to call repeatedly on the same vm; later calls
// overwrite the previous bindings. We rely on this when the pool reuses a
// runtime: the engine re-runs SetupSandbox so the logCb closure for the new
// invocation replaces the stale one captured by the previous run.
func SetupSandbox(vm *goja.Runtime, logCb func(level, msg string)) error {
	if vm == nil {
		return fmt.Errorf("scriptengine: nil runtime")
	}

	for _, name := range blockedCallables {
		if err := vm.Set(name, makeThrowingStub(vm, name)); err != nil {
			return fmt.Errorf("scriptengine: block %s: %w", name, err)
		}
	}
	for _, name := range blockedObjects {
		obj := vm.NewDynamicObject(&sandboxObject{vm: vm, name: name})
		if err := vm.Set(name, obj); err != nil {
			return fmt.Errorf("scriptengine: block %s: %w", name, err)
		}
	}
	// Constructors need to be JS-side functions so `new Foo()` works: a
	// raw Go func reaches goja as a value that is not flagged as a
	// constructor. We install them via a small JS prelude that closes
	// over a host-provided thrower.
	throwHook := func(call goja.FunctionCall) goja.Value {
		name := "unknown"
		if len(call.Arguments) > 0 {
			name = call.Arguments[0].String()
		}
		panic(vm.NewGoError(fmt.Errorf("sandbox: %q is not allowed in sandbox", name)))
	}
	if err := vm.Set("__sandboxThrow", throwHook); err != nil {
		return fmt.Errorf("scriptengine: install __sandboxThrow: %w", err)
	}
	for _, name := range blockedConstructors {
		// Wrap in a JS function so it is recognised as a callable
		// constructor; the body re-enters host code via __sandboxThrow.
		js := fmt.Sprintf("var %s = function %s() { __sandboxThrow(%q); };",
			name, name, name)
		if _, err := vm.RunString(js); err != nil {
			return fmt.Errorf("scriptengine: install constructor stub %s: %w", name, err)
		}
	}

	// console.* needs to be an object whose methods all reach into logCb.
	// We construct it programmatically (instead of via RunString) so the
	// callback never escapes Go control flow.
	if logCb == nil {
		logCb = func(string, string) {}
	}
	console := vm.NewObject()
	for _, lvl := range []string{"log", "info", "warn", "error", "debug"} {
		level := lvl
		err := console.Set(level, func(call goja.FunctionCall) goja.Value {
			logCb(level, formatConsoleArgs(call.Arguments))
			return goja.Undefined()
		})
		if err != nil {
			return fmt.Errorf("scriptengine: install console.%s: %w", level, err)
		}
	}
	if err := vm.Set("console", console); err != nil {
		return fmt.Errorf("scriptengine: install console: %w", err)
	}

	return nil
}

// makeThrowingStub returns a JS function that, when invoked or used as a
// constructor, panics with the sandbox-marker error caller code recognises.
// goja converts the panic into a JS exception whose message classifyRunError
// later maps onto ErrSandboxViolation.
func makeThrowingStub(vm *goja.Runtime, name string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		panic(vm.NewGoError(fmt.Errorf("sandbox: %q is not allowed in sandbox", name)))
	}
}

// sandboxObject is the DynamicObject backing process / fs / globalThis. Any
// attempt to read a property (process.exit, fs.readFileSync, …) throws the
// same sandbox-marker error, so the user's call site fails immediately and
// uniformly.
//
// Has/Keys are conservative: `'exit' in process` returns false (no
// properties to enumerate), but `process.exit` still triggers Get, which
// throws. We deliberately do NOT advertise a non-empty key set — that would
// let scripts hot-swap behaviour based on availability checks and is not a
// promise we want to make.
type sandboxObject struct {
	vm   *goja.Runtime
	name string
}

func (s *sandboxObject) Get(key string) goja.Value {
	panic(s.vm.NewGoError(fmt.Errorf("sandbox: %q.%s is not allowed in sandbox", s.name, key)))
}

func (s *sandboxObject) Set(string, goja.Value) bool {
	panic(s.vm.NewGoError(fmt.Errorf("sandbox: %q is not allowed in sandbox", s.name)))
}

func (s *sandboxObject) Has(string) bool { return false }

func (s *sandboxObject) Delete(string) bool { return false }

func (s *sandboxObject) Keys() []string { return nil }

// formatConsoleArgs concatenates the JS console.* arguments using a space
// separator (the de-facto behaviour of every browser / Node implementation).
// Objects are stringified via goja's String() because JSON.stringify is not
// always reachable from this scope and we never want a panic here to abort
// the script.
func formatConsoleArgs(args []goja.Value) string {
	out := make([]byte, 0, 64)
	for i, a := range args {
		if i > 0 {
			out = append(out, ' ')
		}
		if a == nil {
			out = append(out, "undefined"...)
			continue
		}
		out = append(out, a.String()...)
	}
	return string(out)
}
