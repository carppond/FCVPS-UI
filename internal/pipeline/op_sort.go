package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"shiguang-vps/internal/substore"
)

// SortArgs are the parameters consumed by the sort operator.
type SortArgs struct {
	// Key is the sort key. Recognised: "name" / "server" / "port" / "tag" /
	// "protocol" / "latency" (latency is reserved for post-TCPing pipelines
	// — currently sorts by name as fallback).
	Key string `json:"key"`

	// Order is "asc" or "desc". Empty defaults to "asc".
	Order string `json:"order"`
}

// sortOp implements Operator for the "sort" kind.
type sortOp struct {
	args SortArgs
}

func init() { Register(KindSort, newSortOp) }

// validSortKeys lists the keys the engine knows how to compare.
var validSortKeys = map[string]struct{}{
	"name":     {},
	"server":   {},
	"port":     {},
	"tag":      {},
	"protocol": {},
	"latency":  {},
}

// newSortOp is the registered factory.
func newSortOp(raw json.RawMessage) (Operator, error) {
	var a SortArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("%w: sort: %v", ErrInvalidArgs, err)
		}
	}
	if a.Key == "" {
		return nil, fmt.Errorf("%w: sort.key required", ErrInvalidArgs)
	}
	if _, ok := validSortKeys[a.Key]; !ok {
		return nil, fmt.Errorf("%w: sort.key %q unsupported", ErrInvalidArgs, a.Key)
	}
	if a.Order == "" {
		a.Order = "asc"
	}
	if a.Order != "asc" && a.Order != "desc" {
		return nil, fmt.Errorf("%w: sort.order must be asc/desc", ErrInvalidArgs)
	}
	return &sortOp{args: a}, nil
}

// Kind returns "sort".
func (op *sortOp) Kind() string { return KindSort }

// Apply returns a freshly sorted slice. The sort is stable: equal keys keep
// their input order, which matters when multiple sorts compose (lexicographic
// sort key precedence is achieved by chaining sort ops from less-significant
// to most-significant).
func (op *sortOp) Apply(ctx context.Context, nodes []*substore.ParsedNode) ([]*substore.ParsedNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := append([]*substore.ParsedNode(nil), nodes...)
	less := op.lessFn()
	sort.SliceStable(out, less(out))
	return out, nil
}

// lessFn returns a closure that, given the slice being sorted, produces the
// sort.SliceStable comparator. The two-step indirection lets us avoid
// capturing the slice in the comparator (Go's escape analysis is happier).
func (op *sortOp) lessFn() func([]*substore.ParsedNode) func(i, j int) bool {
	desc := op.args.Order == "desc"
	key := op.args.Key
	return func(s []*substore.ParsedNode) func(i, j int) bool {
		return func(i, j int) bool {
			a, b := s[i], s[j]
			r := compareNodesByKey(a, b, key)
			if desc {
				return r > 0
			}
			return r < 0
		}
	}
}

// compareNodesByKey returns -1 / 0 / 1 just like strings.Compare. latency is
// not yet recorded on ParsedNode (PRD reserves it for post-TCPing pipelines);
// we fall back to comparing by name so the operator never panics on legacy
// inputs.
func compareNodesByKey(a, b *substore.ParsedNode, key string) int {
	switch key {
	case "name":
		return strings.Compare(a.Name, b.Name)
	case "server":
		return strings.Compare(a.Server, b.Server)
	case "port":
		return cmpInt(a.Port, b.Port)
	case "tag":
		return strings.Compare(a.Tag, b.Tag)
	case "protocol":
		return strings.Compare(a.Protocol, b.Protocol)
	case "latency":
		// Not yet available — keep deterministic order by name.
		return strings.Compare(a.Name, b.Name)
	}
	return 0
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
