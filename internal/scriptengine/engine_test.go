package scriptengine_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/scriptengine"
)

// happy path: script multiplies an input integer by two and returns it.
// PRD M-SCRIPT.3 (script can transform input → output).
func TestEngine_Run_HappyPath(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	src := `
		var n = __input.x * 2;
		__output = { result: n };
	`
	res, err := e.Run(context.Background(), src, map[string]any{"x": 5}, 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(res.Output), &got); err != nil {
		t.Fatalf("unmarshal: %v (output=%q)", err, res.Output)
	}
	if v, _ := got["result"].(float64); v != 10 {
		t.Fatalf("result = %v, want 10", got["result"])
	}
}

// PRD M-SCRIPT.1 — runaway scripts must be killed inside 5s.
//
// We allow 6s of headroom: the engine arms a 5s interrupt timer and goja's
// next-instruction check usually fires within a handful of ms after that.
// Anything north of 6s indicates the interrupt did not land.
func TestEngine_Run_Timeout(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	src := `while (true) { var x = 1 + 1; }`
	start := time.Now()
	_, err := e.Run(context.Background(), src, nil, 5*time.Second)
	elapsed := time.Since(start)
	if !scriptengine.IsTimeout(err) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
	if elapsed > 6*time.Second {
		t.Fatalf("interrupt fired late: %v (>6s budget)", elapsed)
	}
	if elapsed < 4*time.Second {
		t.Fatalf("interrupt fired too early: %v (<4s)", elapsed)
	}
}

// Reuse: 100 small scripts share the runtime pool. Spec target: <2s.
// With Engine.NewEngine + sync.Pool the warm path is ~hundreds of ns
// per script; the assert keeps a comfortable budget for slow CI.
func TestEngine_Run_PoolReuse_Throughput(t *testing.T) {
	if testing.Short() {
		t.Skip("throughput test skipped in -short")
	}
	e := scriptengine.NewEngine(nil)
	src := `__output = { y: __input.x + 1 };`
	start := time.Now()
	for i := 0; i < 100; i++ {
		_, err := e.Run(context.Background(), src, map[string]any{"x": i}, time.Second)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("100 runs took %v (>2s); pool not reusing?", d)
	}
}

// Cross-tenant isolation: a script that leaves __output set must NOT pollute
// the next run. Verifies the returnRuntime reset path.
func TestEngine_Run_Isolation(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	if _, err := e.Run(context.Background(), `__output = { leak: 1 };`, nil, time.Second); err != nil {
		t.Fatalf("first run: %v", err)
	}
	res, err := e.Run(context.Background(), `__output = __output;`, nil, time.Second)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if res.Output != "" {
		t.Fatalf("expected fresh __output, got %q", res.Output)
	}
}

// Runtime errors inside user code surface as ordinary error returns (not
// timeout, not sandbox). Logs accumulated before the throw are preserved.
func TestEngine_Run_RuntimeError(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	src := `
		console.log('about to die');
		throw new Error('boom');
	`
	res, err := e.Run(context.Background(), src, nil, time.Second)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if scriptengine.IsTimeout(err) || scriptengine.IsSandboxViolation(err) {
		t.Fatalf("wrong error class: %v", err)
	}
	if res == nil || len(res.Logs) != 1 || !strings.Contains(res.Logs[0], "about to die") {
		t.Fatalf("logs not preserved: %+v", res)
	}
}

// Context cancellation BEFORE the run is honoured: Run returns fast without
// invoking goja.
func TestEngine_Run_ContextCancelled(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.Run(ctx, `__output = 1;`, nil, time.Second)
	if err == nil {
		t.Fatalf("expected error from cancelled context")
	}
}

// Hook: RunPreSaveNodes feeds nodes under __input.nodes and surfaces the
// transformed JSON unchanged.
func TestEngine_RunPreSaveNodes(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	nodes := []map[string]any{
		{"name": "a", "protocol": "ss"},
		{"name": "b", "protocol": "vmess"},
	}
	src := `__output = __input.nodes.filter(function(n){ return n.protocol === 'ss'; });`
	res, err := e.RunPreSaveNodes(context.Background(), src, nodes, time.Second)
	if err != nil {
		t.Fatalf("RunPreSaveNodes: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(res.JSONBody), &got); err != nil {
		t.Fatalf("unmarshal: %v (body=%q)", err, res.JSONBody)
	}
	if len(got) != 1 || got[0]["name"] != "a" {
		t.Fatalf("filter result: %+v", got)
	}
}

// Hook: RunPostFetch round-trips raw text and unwraps the JSON-string
// quoting that Engine.Run otherwise leaves attached.
func TestEngine_RunPostFetch(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	src := `__output = __input.raw.replace('foo', 'bar');`
	out, _, _, err := e.RunPostFetch(context.Background(), src, "foo baz", time.Second)
	if err != nil {
		t.Fatalf("RunPostFetch: %v", err)
	}
	if out != "bar baz" {
		t.Fatalf("rewrite failed: %q", out)
	}
}

// Empty hook code is a programming error — guard rails return early.
func TestEngine_Hooks_EmptyCode(t *testing.T) {
	e := scriptengine.NewEngine(nil)
	if _, err := e.RunPreSaveNodes(context.Background(), "", nil, time.Second); err == nil {
		t.Fatalf("expected error for empty pre-save code")
	}
	if _, _, _, err := e.RunPostFetch(context.Background(), "", "", time.Second); err == nil {
		t.Fatalf("expected error for empty post-fetch code")
	}
}
