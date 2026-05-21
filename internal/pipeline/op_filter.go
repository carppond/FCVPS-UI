package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"shiguang-vps/internal/substore"
)

// FilterArgs are the parameters consumed by the filter operator.
// The shape mirrors types.FilterArgs (single `expr` field, mini-language).
type FilterArgs struct {
	// Expr is the filter expression. See expr.go for grammar. Empty expr
	// keeps every node (no-op).
	Expr string `json:"expr"`
}

// filterOp implements Operator for the "filter" kind.
type filterOp struct {
	args FilterArgs
	expr *FilterExpr
}

func init() { Register(KindFilter, newFilterOp) }

// newFilterOp is the registered factory; performs eager validation so AST
// load surfaces errors before runtime.
func newFilterOp(raw json.RawMessage) (Operator, error) {
	var a FilterArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("%w: filter: %v", ErrInvalidArgs, err)
		}
	}
	expr, err := CompileFilter(a.Expr)
	if err != nil {
		return nil, err
	}
	return &filterOp{args: a, expr: expr}, nil
}

// Kind returns "filter".
func (op *filterOp) Kind() string { return KindFilter }

// Apply keeps the nodes whose expression evaluates truthy. An evaluator error
// (e.g. unknown field) propagates and aborts the run with the step index.
func (op *filterOp) Apply(ctx context.Context, nodes []*substore.ParsedNode) ([]*substore.ParsedNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]*substore.ParsedNode, 0, len(nodes))
	for _, n := range nodes {
		ok, err := op.expr.Eval(n)
		if err != nil {
			return nil, fmt.Errorf("filter eval: %w", err)
		}
		if ok {
			out = append(out, n)
		}
	}
	return out, nil
}
