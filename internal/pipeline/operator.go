package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"shiguang-vps/internal/substore"
)

// Operator kind constants. They mirror types.OperatorType but are declared
// here so the pipeline package stays import-cycle-free with internal/types.
const (
	KindFilter      = "filter"
	KindMap         = "map"
	KindSort        = "sort"
	KindDedupe      = "dedupe"
	KindRegexRename = "regex_rename"
	KindOutput      = "output"
)

// Operator is the contract implemented by every algorithm in the engine.
//
// Apply MUST be pure with respect to its inputs: it MUST NOT mutate the input
// slice header (the engine reuses input snapshots for diff calculation) and
// SHOULD return a freshly allocated slice. Operators that mutate individual
// *ParsedNode fields (e.g. map) MUST clone the offending nodes via
// substore.CloneParsedNode (helper provided in this package) before writing.
//
// Apply MAY honour ctx cancellation; the engine wraps every step with a
// derived context so global deadlines propagate.
type Operator interface {
	Apply(ctx context.Context, nodes []*substore.ParsedNode) ([]*substore.ParsedNode, error)
	Kind() string
}

// OperatorFactory builds an Operator from its serialised args.
// Implementations MUST validate args eagerly and return ErrInvalidArgs (or a
// wrapped version) on failure.
type OperatorFactory func(args json.RawMessage) (Operator, error)

// registry holds the kind → factory mapping populated via init() in each
// op_*.go. registryMu serialises Register / New calls so package init can
// safely race against test setup.
var (
	registry   = map[string]OperatorFactory{}
	registryMu sync.RWMutex
)

// Sentinel errors. Wrap with fmt.Errorf("%w: ...", ErrXxx) for context.
var (
	// ErrUnknownOperator means the AST referenced a kind not present in the
	// registry. Maps to types.ErrPipelineOperatorUnknown at the handler edge.
	ErrUnknownOperator = errors.New("pipeline: unknown operator kind")
	// ErrInvalidArgs is returned by factories when args fail validation.
	// Maps to types.ErrPipelineOperatorParams.
	ErrInvalidArgs = errors.New("pipeline: invalid operator args")
	// ErrInvalidRegex marks a regex compile failure (filter / regex_rename).
	// Maps to types.ErrValidationRegexCompile.
	ErrInvalidRegex = errors.New("pipeline: invalid regex")
	// ErrSchemaMismatch is returned when the api_version is missing /
	// unsupported. Maps to types.ErrValidationSchemaMismatch.
	ErrSchemaMismatch = errors.New("pipeline: schema version mismatch")
	// ErrOutputRequired is returned when the AST does not end with an output
	// operator (or has more than one output step).
	ErrOutputRequired = errors.New("pipeline: ast must end with output")
	// ErrTooManyOperators caps AST size (defence against accidental loops in
	// future GUI work).
	ErrTooManyOperators = errors.New("pipeline: too many operators")
)

// Register installs factory under kind. Calling Register twice with the same
// kind panics — operator names are compile-time stable.
func Register(kind string, factory OperatorFactory) {
	if kind == "" || factory == nil {
		panic("pipeline.Register: empty kind or nil factory")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[kind]; dup {
		panic(fmt.Sprintf("pipeline.Register: duplicate kind %q", kind))
	}
	registry[kind] = factory
}

// New instantiates the operator identified by kind from its raw args.
// Returns ErrUnknownOperator when the kind is not registered.
func New(kind string, args json.RawMessage) (Operator, error) {
	registryMu.RLock()
	factory, ok := registry[kind]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownOperator, kind)
	}
	op, err := factory(args)
	if err != nil {
		return nil, err
	}
	return op, nil
}

// RegisteredKinds returns the sorted list of operator kinds known to the
// registry. Used by the handler-layer GET /api/pipelines/operators endpoint
// (to be wired in T-13) and by tests.
func RegisteredKinds() []string {
	registryMu.RLock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	registryMu.RUnlock()
	sort.Strings(out)
	return out
}

// cloneNode performs a shallow + deep-enough copy of a ParsedNode so that
// downstream operators can mutate it without disturbing the engine's pre-step
// snapshot (used for diff). Slices and maps are deep-copied; scalar fields are
// copied by value. nil input returns nil.
func cloneNode(n *substore.ParsedNode) *substore.ParsedNode {
	if n == nil {
		return nil
	}
	out := *n
	if n.ALPN != nil {
		out.ALPN = append([]string(nil), n.ALPN...)
	}
	if n.Raw != nil {
		out.Raw = make(map[string]interface{}, len(n.Raw))
		for k, v := range n.Raw {
			out.Raw[k] = v
		}
	}
	return &out
}

// cloneNodes returns a fresh slice whose elements are independent clones of
// the input. The engine uses this to snapshot input lists before each step.
func cloneNodes(in []*substore.ParsedNode) []*substore.ParsedNode {
	if in == nil {
		return nil
	}
	out := make([]*substore.ParsedNode, len(in))
	for i, n := range in {
		out[i] = cloneNode(n)
	}
	return out
}
