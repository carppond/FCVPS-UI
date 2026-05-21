package scriptengine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/scriptengine"
)

// Each entry exercises one of the blocklisted globals from sandbox.go.
// PRD M-SCRIPT.2 mandates these all throw with a recognisable "sandbox:"
// prefix so the handler can map them onto ERR_SCRIPT_SANDBOX_VIOLATION.
func TestSandbox_BlockedGlobals(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"require", `require('fs');`},
		{"fetch", `fetch('https://example.com');`},
		{"process_exit", `process.exit(0);`},
		{"fs_read", `fs.readFileSync('/etc/passwd');`},
		{"eval", `eval('1+1');`},
		{"set_timeout", `setTimeout(function(){}, 10);`},
		{"set_interval", `setInterval(function(){}, 10);`},
		{"xhr", `new XMLHttpRequest();`},
		{"websocket", `new WebSocket('wss://x');`},
	}
	e := scriptengine.NewEngine(nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.Run(context.Background(), tc.src, nil, time.Second)
			if err == nil {
				t.Fatalf("expected sandbox violation for %s, got nil", tc.name)
			}
			if !scriptengine.IsSandboxViolation(err) {
				t.Fatalf("expected ErrSandboxViolation, got %v", err)
			}
			if !strings.Contains(err.Error(), "not allowed in sandbox") {
				t.Fatalf("error message missing sandbox marker: %v", err)
			}
		})
	}
}

// console.log / warn / error must all reach the host-supplied callback.
func TestSandbox_ConsoleRedirect(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	src := `
		console.log('hello', 'world');
		console.warn('careful');
		console.error('boom');
		console.info('fyi');
		console.debug('dbg');
		__output = { ok: true };
	`
	res, err := e.Run(context.Background(), src, nil, time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{
		"[log] hello world",
		"[warn] careful",
		"[error] boom",
		"[info] fyi",
		"[debug] dbg",
	}
	if len(res.Logs) != len(want) {
		t.Fatalf("log count = %d, want %d (got %+v)", len(res.Logs), len(want), res.Logs)
	}
	for i, w := range want {
		if res.Logs[i] != w {
			t.Fatalf("log[%d] = %q, want %q", i, res.Logs[i], w)
		}
	}
}

// JSON is the *only* boundary we expose; it must remain reachable from user
// scripts because Engine.Run's bootstrap uses JSON.parse and user scripts
// stringify their output via JSON.stringify when assigning objects.
func TestSandbox_JSONIntact(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	src := `
		var o = JSON.parse('{"a": 1}');
		__output = JSON.stringify({ a: o.a + 1 });
	`
	res, err := e.Run(context.Background(), src, nil, time.Second)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Output != `{"a":2}` {
		t.Fatalf("output = %q, want {\"a\":2}", res.Output)
	}
}

// SetupSandbox must not error when called twice on the same vm — the
// engine's pool path re-runs it for every borrowed runtime.
func TestSandbox_IdempotentInstall(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	src := `__output = 'ok';`
	for i := 0; i < 3; i++ {
		_, err := e.Run(context.Background(), src, nil, time.Second)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
}
