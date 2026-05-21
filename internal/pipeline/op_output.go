package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"shiguang-vps/internal/substore"
)

// Output format constants. v1 ships clash + raw; clash_meta is accepted as an
// alias for clash (contract §2.10 lists it for forward compat) and surfaced
// verbatim in the run result so callers can branch on it.
const (
	OutputFormatClash     = "clash"
	OutputFormatClashMeta = "clash_meta"
	OutputFormatRaw       = "raw"
)

// OutputArgs are the parameters consumed by the output operator. The shape
// mirrors types.OutputArgs.
type OutputArgs struct {
	Format   string `json:"format"`
	MaxNodes int32  `json:"max_nodes,omitempty"`
}

// outputOp implements Operator for the "output" kind. It does not touch node
// fields; it only enforces MaxNodes (truncation) and stashes Format so the
// engine can surface it on the Result.
type outputOp struct {
	args OutputArgs
}

func init() { Register(KindOutput, newOutputOp) }

var validOutputFormats = map[string]struct{}{
	OutputFormatClash:     {},
	OutputFormatClashMeta: {},
	OutputFormatRaw:       {},
}

// newOutputOp is the registered factory.
func newOutputOp(raw json.RawMessage) (Operator, error) {
	var a OutputArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("%w: output: %v", ErrInvalidArgs, err)
		}
	}
	if a.Format == "" {
		return nil, fmt.Errorf("%w: output.format required", ErrInvalidArgs)
	}
	if _, ok := validOutputFormats[a.Format]; !ok {
		return nil, fmt.Errorf("%w: output.format %q unsupported", ErrInvalidArgs, a.Format)
	}
	if a.MaxNodes < 0 {
		return nil, fmt.Errorf("%w: output.max_nodes must be >= 0", ErrInvalidArgs)
	}
	return &outputOp{args: a}, nil
}

// Kind returns "output".
func (op *outputOp) Kind() string { return KindOutput }

// Format returns the configured output format string.
func (op *outputOp) Format() string { return op.args.Format }

// MaxNodes returns the configured truncation limit (0 = unbounded).
func (op *outputOp) MaxNodes() int32 { return op.args.MaxNodes }

// Apply enforces MaxNodes if set; otherwise passes nodes through. The engine
// uses the Format() / MaxNodes() accessors to populate the run result.
func (op *outputOp) Apply(ctx context.Context, nodes []*substore.ParsedNode) ([]*substore.ParsedNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if op.args.MaxNodes > 0 && int(op.args.MaxNodes) < len(nodes) {
		return nodes[:op.args.MaxNodes], nil
	}
	return nodes, nil
}
