package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"shiguang-vps/internal/substore"
)

// Engine is the M-PIPE runner. It is stateless aside from defaults; create
// once and share across goroutines.
type Engine struct {
	// MaxStepPreview caps how many node names are recorded per diff field
	// (added / removed / modified). Larger inputs are silently truncated.
	MaxStepPreview int

	// Deadline is the per-run default deadline; callers may override via ctx
	// timeout. Zero disables the engine-side timeout (the user-supplied ctx
	// is still honoured).
	Deadline time.Duration
}

// NewEngine returns an Engine with project defaults.
//
//   - MaxStepPreview = 20 (matches PRD M-PIPE.5 "max 20 names per diff slot").
//   - Deadline = 0 (rely on caller-supplied context deadline).
func NewEngine() *Engine {
	return &Engine{MaxStepPreview: 20, Deadline: 0}
}

// StepRecord captures the input/output sizes and the symmetric difference
// between two stages of a pipeline run. Names are truncated to
// engine.MaxStepPreview entries.
type StepRecord struct {
	// Step is the 0-based operator index.
	Step int
	// Kind is the operator type (filter / map / ...).
	Kind string
	// BeforeN is the number of nodes entering the step.
	BeforeN int
	// AfterN is the number of nodes leaving the step.
	AfterN int
	// Added lists node names present after but absent before.
	Added []string
	// Removed lists node names present before but absent after.
	Removed []string
	// Modified lists node names whose attributes changed (same name index).
	Modified []string
	// DurationMS is the time spent inside the step's Apply (excludes diff).
	DurationMS int64
}

// Result is the aggregate engine output.
type Result struct {
	// Output is the final node slice (post-output operator).
	Output []*substore.ParsedNode
	// OutputFormat carries the format requested by the terminal output step
	// (clash / clash_meta / raw).
	OutputFormat string
	// MaxNodes mirrors the OutputArgs.max_nodes field; 0 = no cap.
	MaxNodes int32
	// Steps lists per-step diff records. Always populated (Run + Preview).
	Steps []StepRecord
	// DurationMS is the total wall-clock time for the run.
	DurationMS int64
}

// PipelineError wraps an operator failure with the step index + kind. The
// engine returns it from Run / Preview so handlers can surface the bad step
// to the user (Tech Lead §1.4 RunResult shape).
type PipelineError struct {
	Step int
	Kind string
	Err  error
}

// Error implements the error interface.
func (e *PipelineError) Error() string {
	return fmt.Sprintf("pipeline: step %d (%s): %v", e.Step, e.Kind, e.Err)
}

// Unwrap exposes the wrapped error for errors.Is / errors.As.
func (e *PipelineError) Unwrap() error { return e.Err }

// Run executes the AST against input and returns the final slice + per-step
// diffs. Both Validate and operator instantiation happen here, so callers
// pass a fresh AST; the engine never mutates it.
//
// Run honours ctx cancellation between every step.
func (e *Engine) Run(ctx context.Context, ast *AST, input []*substore.ParsedNode) (*Result, error) {
	if err := Validate(ast); err != nil {
		return nil, err
	}
	if e.Deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.Deadline)
		defer cancel()
	}

	start := time.Now()
	res := &Result{Steps: make([]StepRecord, 0, len(ast.Operators))}
	current := input

	for i, spec := range ast.Operators {
		if !spec.Enabled && spec.Kind != KindOutput {
			// Disabled non-terminal step: passthrough, still record a no-op
			// row so the UI can render it.
			res.Steps = append(res.Steps, StepRecord{
				Step: i, Kind: spec.Kind, BeforeN: len(current), AfterN: len(current),
			})
			continue
		}
		op, err := New(spec.Kind, spec.Args)
		if err != nil {
			return nil, &PipelineError{Step: i, Kind: spec.Kind, Err: err}
		}
		before := cloneNodes(current)
		t0 := time.Now()
		next, err := op.Apply(ctx, current)
		stepMS := time.Since(t0).Milliseconds()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return nil, &PipelineError{Step: i, Kind: spec.Kind, Err: err}
			}
			return nil, &PipelineError{Step: i, Kind: spec.Kind, Err: err}
		}
		rec := buildStepRecord(i, spec.Kind, before, next, e.MaxStepPreview)
		rec.DurationMS = stepMS
		res.Steps = append(res.Steps, rec)
		current = next

		if oop, ok := op.(*outputOp); ok {
			res.OutputFormat = oop.Format()
			res.MaxNodes = oop.MaxNodes()
		}
	}

	res.Output = current
	res.DurationMS = time.Since(start).Milliseconds()
	return res, nil
}

// buildStepRecord computes the symmetric diff and modified set keyed by node
// name. When operators replace nodes (clone-on-write, e.g. map), the names
// stay stable but the node pointer differs — we detect those via field-level
// equality. Detail truncates to maxNames entries per slot.
func buildStepRecord(step int, kind string, before, after []*substore.ParsedNode, maxNames int) StepRecord {
	rec := StepRecord{
		Step: step, Kind: kind,
		BeforeN: len(before), AfterN: len(after),
	}
	beforeByName := indexByName(before)
	afterByName := indexByName(after)

	for name, beforeNode := range beforeByName {
		if afterNode, ok := afterByName[name]; !ok {
			rec.Removed = appendCapped(rec.Removed, name, maxNames)
		} else if !nodesEqual(beforeNode, afterNode) {
			rec.Modified = appendCapped(rec.Modified, name, maxNames)
		}
	}
	for name := range afterByName {
		if _, ok := beforeByName[name]; !ok {
			rec.Added = appendCapped(rec.Added, name, maxNames)
		}
	}
	return rec
}

// indexByName returns a name → first-occurrence map. Duplicate names retain
// the first one (downstream diff still detects "removed" when count shrinks).
func indexByName(nodes []*substore.ParsedNode) map[string]*substore.ParsedNode {
	out := make(map[string]*substore.ParsedNode, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if _, dup := out[n.Name]; !dup {
			out[n.Name] = n
		}
	}
	return out
}

// nodesEqual compares the user-visible fields. Used to detect "modified" in
// the step diff. Raw map is compared via value equality on a best-effort
// basis (deep equality of unknown JSON would be over-kill for diff display).
func nodesEqual(a, b *substore.ParsedNode) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Name == b.Name && a.Protocol == b.Protocol &&
		a.Server == b.Server && a.Port == b.Port &&
		a.UUID == b.UUID && a.Password == b.Password &&
		a.Method == b.Method && a.Network == b.Network &&
		a.TLS == b.TLS && a.SNI == b.SNI &&
		a.Path == b.Path && a.Host == b.Host &&
		a.Tag == b.Tag && a.Reality == b.Reality
}

// appendCapped appends name to slice unless the cap is hit.
func appendCapped(slice []string, name string, cap int) []string {
	if cap > 0 && len(slice) >= cap {
		return slice
	}
	return append(slice, name)
}
