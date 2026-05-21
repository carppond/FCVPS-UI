package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/substore"
)

func mustAST(t *testing.T, ops ...OperatorSpec) *AST {
	t.Helper()
	return &AST{APIVersion: APIVersion, Operators: ops}
}

func opSpec(t testing.TB, kind string, args any) OperatorSpec {
	if t != nil {
		t.Helper()
	}
	raw, _ := json.Marshal(args)
	return OperatorSpec{Kind: kind, Args: raw, Enabled: true}
}

func TestEngine_HappyPath(t *testing.T) {
	ast := mustAST(t,
		opSpec(t, KindFilter, FilterArgs{Expr: `protocol in ["vmess","vless"]`}),
		opSpec(t, KindSort, SortArgs{Key: "name", Order: "asc"}),
		opSpec(t, KindOutput, OutputArgs{Format: "clash"}),
	)
	in := sampleNodes()
	eng := NewEngine()
	res, err := eng.Run(context.Background(), ast, in)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.OutputFormat != "clash" {
		t.Fatalf("output format: %q", res.OutputFormat)
	}
	if len(res.Output) != 2 {
		t.Fatalf("want 2 nodes (vmess+vless), got %d", len(res.Output))
	}
	if res.Output[0].Name != "HK-1" || res.Output[1].Name != "JP-Tokyo" {
		t.Fatalf("sort wrong: %v", namesOf(res.Output))
	}
	if len(res.Steps) != 3 {
		t.Fatalf("want 3 steps, got %d", len(res.Steps))
	}
	// Filter removed 3 nodes.
	if res.Steps[0].BeforeN != 5 || res.Steps[0].AfterN != 2 {
		t.Fatalf("step 0 diff: before=%d after=%d", res.Steps[0].BeforeN, res.Steps[0].AfterN)
	}
	if len(res.Steps[0].Removed) != 3 {
		t.Fatalf("want 3 removed names, got %d", len(res.Steps[0].Removed))
	}
}

func TestEngine_RejectsMissingOutput(t *testing.T) {
	ast := mustAST(t, opSpec(t, KindFilter, FilterArgs{Expr: ""}))
	_, err := NewEngine().Run(context.Background(), ast, nil)
	if !errors.Is(err, ErrOutputRequired) {
		t.Fatalf("want ErrOutputRequired, got %v", err)
	}
}

func TestEngine_RejectsMisplacedOutput(t *testing.T) {
	ast := mustAST(t,
		opSpec(t, KindOutput, OutputArgs{Format: "clash"}),
		opSpec(t, KindFilter, FilterArgs{}),
	)
	_, err := NewEngine().Run(context.Background(), ast, nil)
	if !errors.Is(err, ErrOutputRequired) {
		t.Fatalf("want ErrOutputRequired, got %v", err)
	}
}

func TestEngine_RejectsSchemaMismatch(t *testing.T) {
	ast := &AST{APIVersion: "bogus/v9", Operators: []OperatorSpec{
		opSpec(t, KindOutput, OutputArgs{Format: "clash"}),
	}}
	_, err := NewEngine().Run(context.Background(), ast, nil)
	if !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("want ErrSchemaMismatch, got %v", err)
	}
}

func TestEngine_RejectsTooManyOperators(t *testing.T) {
	ops := make([]OperatorSpec, 0, MaxOperators+1)
	for i := 0; i < MaxOperators; i++ {
		ops = append(ops, opSpec(t, KindFilter, FilterArgs{}))
	}
	ops = append(ops, opSpec(t, KindOutput, OutputArgs{Format: "clash"}))
	ast := &AST{APIVersion: APIVersion, Operators: ops}
	_, err := NewEngine().Run(context.Background(), ast, nil)
	if !errors.Is(err, ErrTooManyOperators) {
		t.Fatalf("want ErrTooManyOperators, got %v", err)
	}
}

func TestEngine_OperatorFactoryErrorWrapped(t *testing.T) {
	ast := mustAST(t,
		// Output factory blows up because format is unsupported.
		OperatorSpec{Kind: KindOutput, Args: json.RawMessage(`{"format":"weird"}`), Enabled: true},
	)
	// Validate passes (it only checks kind / shape), the engine surfaces the
	// factory error wrapped in PipelineError.
	_, err := NewEngine().Run(context.Background(), ast, nil)
	var pe *PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("want PipelineError, got %v", err)
	}
	if pe.Step != 0 || pe.Kind != KindOutput {
		t.Fatalf("step/kind wrong: %+v", pe)
	}
}

func TestEngine_PreviewBuildsStepSnapshots(t *testing.T) {
	ast := mustAST(t,
		opSpec(t, KindFilter, FilterArgs{Expr: `protocol == "vmess"`}),
		opSpec(t, KindOutput, OutputArgs{Format: "clash"}),
	)
	res, err := NewEngine().Preview(context.Background(), ast, sampleNodes())
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(res.Steps))
	}
	if len(res.Steps[0].BeforeIDs) != 5 || len(res.Steps[0].AfterIDs) != 1 {
		t.Fatalf("preview step 0 IDs: before=%d after=%d", len(res.Steps[0].BeforeIDs), len(res.Steps[0].AfterIDs))
	}
}

func TestEngine_DisabledStepSkipped(t *testing.T) {
	ast := mustAST(t,
		OperatorSpec{Kind: KindFilter, Args: mustJSON(FilterArgs{Expr: "false"}), Enabled: false},
		opSpec(t, KindOutput, OutputArgs{Format: "clash"}),
	)
	res, err := NewEngine().Run(context.Background(), ast, sampleNodes())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.Output) != len(sampleNodes()) {
		t.Fatalf("disabled filter should not drop nodes; got %d", len(res.Output))
	}
}

func TestEngine_ContextCancellationPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ast := mustAST(t,
		opSpec(t, KindFilter, FilterArgs{}),
		opSpec(t, KindOutput, OutputArgs{Format: "clash"}),
	)
	_, err := NewEngine().Run(ctx, ast, sampleNodes())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// TestEngine_100Nodes is the PRD M-PIPE.3 baseline: 100 nodes through 6
// operators must finish well under 500ms. The test logs the measured time
// so engineers can spot regressions in CI output without grepping benches.
func TestEngine_100Nodes(t *testing.T) {
	in := make([]*substore.ParsedNode, 0, 100)
	for i := 0; i < 100; i++ {
		region := []string{"HK", "JP", "SG", "US", "DE"}[i%5]
		proto := []string{"vmess", "vless", "ss", "trojan", "ssr"}[i%5]
		in = append(in, &substore.ParsedNode{
			Name:     fmt.Sprintf("%s-%03d", region, i),
			Protocol: proto,
			Server:   fmt.Sprintf("%d.example.com", i),
			Port:     443 + i,
			TLS:      i%2 == 0,
			Tag:      region,
		})
	}
	ast := mustAST(t,
		opSpec(t, KindFilter, FilterArgs{Expr: `protocol in ["vmess","vless","ss","trojan"]`}),
		opSpec(t, KindMap, MapArgs{Field: "name", Value: "{{.Protocol}}-{{.Index}}-{{.Name}}"}),
		opSpec(t, KindRegexRename, RegexRenameArgs{Pattern: `(\d+)`, Replacement: "n$1"}),
		opSpec(t, KindDedupe, DedupeArgs{Fields: []string{"server", "port"}}),
		opSpec(t, KindSort, SortArgs{Key: "name", Order: "asc"}),
		opSpec(t, KindOutput, OutputArgs{Format: "clash"}),
	)
	t0 := time.Now()
	res, err := NewEngine().Run(context.Background(), ast, in)
	dur := time.Since(t0)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	t.Logf("100-node 6-operator pipeline: %s (output=%d nodes)", dur, len(res.Output))
	if dur > 500*time.Millisecond {
		t.Fatalf("PRD M-PIPE.3 budget exceeded: %s > 500ms", dur)
	}
	if len(res.Output) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func BenchmarkEngine100Nodes(b *testing.B) {
	in := make([]*substore.ParsedNode, 0, 100)
	for i := 0; i < 100; i++ {
		in = append(in, &substore.ParsedNode{
			Name: fmt.Sprintf("n-%03d", i),
			Protocol: []string{"vmess", "vless", "ss", "trojan"}[i%4],
			Server: fmt.Sprintf("%d.example.com", i), Port: 443 + i,
			TLS: i%2 == 0, Tag: strings.ToUpper(string(rune('a' + i%26))),
		})
	}
	ast := &AST{APIVersion: APIVersion, Operators: []OperatorSpec{
		opSpec(nil, KindFilter, FilterArgs{Expr: `protocol == "vmess" || protocol == "vless"`}),
		opSpec(nil, KindMap, MapArgs{Field: "name", Value: "{{.Tag}}-{{.Name}}"}),
		opSpec(nil, KindRegexRename, RegexRenameArgs{Pattern: `(\d)`, Replacement: "x$1"}),
		opSpec(nil, KindDedupe, DedupeArgs{}),
		opSpec(nil, KindSort, SortArgs{Key: "name", Order: "asc"}),
		opSpec(nil, KindOutput, OutputArgs{Format: "clash"}),
	}}
	eng := NewEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := eng.Run(context.Background(), ast, in); err != nil {
			b.Fatal(err)
		}
	}
}
