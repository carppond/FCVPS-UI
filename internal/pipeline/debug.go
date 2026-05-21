package pipeline

import (
	"context"
	"encoding/json"

	"shiguang-vps/internal/substore"
)

// StepPreview is a verbose snapshot of a single pipeline step. Compared to
// StepRecord (which is intentionally lightweight for production runs), the
// preview keeps the full Args + the before / after node-name lists.
type StepPreview struct {
	Step      int             `json:"step"`
	Kind      string          `json:"kind"`
	Args      json.RawMessage `json:"args,omitempty"`
	BeforeIDs []string        `json:"before_ids"`
	AfterIDs  []string        `json:"after_ids"`
	Added     []string        `json:"added,omitempty"`
	Removed   []string        `json:"removed,omitempty"`
	Modified  []string        `json:"modified,omitempty"`
}

// PreviewResult is returned by Preview. It mirrors Run's diff but does not
// expose the final node slice (the GUI does not need it for diff display).
type PreviewResult struct {
	Steps        []StepPreview `json:"steps"`
	OutputFormat string        `json:"output_format,omitempty"`
	DurationMS   int64         `json:"duration_ms"`
	TotalIn      int           `json:"total_in"`
	TotalOut     int           `json:"total_out"`
}

// Preview executes the AST exactly like Run but emits a richer per-step
// snapshot. The input slice is cloned upfront so the caller's nodes are
// untouched even if an operator (e.g. map) writes to fields in-place.
func (e *Engine) Preview(ctx context.Context, ast *AST, input []*substore.ParsedNode) (*PreviewResult, error) {
	cloned := cloneNodes(input)
	res, err := e.Run(ctx, ast, cloned)
	if err != nil {
		return nil, err
	}

	steps := make([]StepPreview, 0, len(res.Steps))
	// Re-walk the AST to attach args + before/after IDs. The engine already
	// computed diff counts in res.Steps, so we only need the snapshots.
	current := input
	for i, spec := range ast.Operators {
		rec := res.Steps[i]
		before := nodeNames(current)
		op, ferr := New(spec.Kind, spec.Args)
		if ferr != nil {
			// Should not happen: Validate already passed inside Run.
			return nil, ferr
		}
		next, aerr := op.Apply(ctx, cloneNodes(current))
		if aerr != nil {
			return nil, aerr
		}
		after := nodeNames(next)

		steps = append(steps, StepPreview{
			Step:      i,
			Kind:      spec.Kind,
			Args:      append(json.RawMessage(nil), spec.Args...),
			BeforeIDs: before,
			AfterIDs:  after,
			Added:     append([]string(nil), rec.Added...),
			Removed:   append([]string(nil), rec.Removed...),
			Modified:  append([]string(nil), rec.Modified...),
		})
		current = next
	}

	return &PreviewResult{
		Steps:        steps,
		OutputFormat: res.OutputFormat,
		DurationMS:   res.DurationMS,
		TotalIn:      len(input),
		TotalOut:     len(res.Output),
	}, nil
}

// nodeNames extracts the display names for diff rendering.
func nodeNames(in []*substore.ParsedNode) []string {
	out := make([]string, 0, len(in))
	for _, n := range in {
		if n == nil {
			continue
		}
		out = append(out, n.Name)
	}
	return out
}
