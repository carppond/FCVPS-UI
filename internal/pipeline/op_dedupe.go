package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"shiguang-vps/internal/substore"
)

// DedupeArgs are the parameters consumed by the dedupe operator.
type DedupeArgs struct {
	// Fields is the ordered list of fields used to build the dedup key. When
	// empty, defaults to ["server", "port"].
	Fields []string `json:"fields"`
}

// dedupeOp implements Operator for the "dedupe" kind.
type dedupeOp struct {
	args DedupeArgs
}

func init() { Register(KindDedupe, newDedupeOp) }

// defaultDedupeFields are used when args.Fields is empty.
var defaultDedupeFields = []string{"server", "port"}

// allowedDedupeFields lists the readable fields. Matches expr.go field set so
// the surface is consistent.
var allowedDedupeFields = map[string]struct{}{
	"name": {}, "server": {}, "port": {}, "protocol": {}, "tag": {},
	"network": {}, "tls": {}, "sni": {}, "password": {}, "uuid": {},
	"method": {}, "host": {}, "path": {}, "region": {},
}

// newDedupeOp is the registered factory.
func newDedupeOp(raw json.RawMessage) (Operator, error) {
	var a DedupeArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("%w: dedupe: %v", ErrInvalidArgs, err)
		}
	}
	if len(a.Fields) == 0 {
		a.Fields = append([]string(nil), defaultDedupeFields...)
	}
	for _, f := range a.Fields {
		if _, ok := allowedDedupeFields[f]; !ok {
			return nil, fmt.Errorf("%w: dedupe.fields: %q not allowed", ErrInvalidArgs, f)
		}
	}
	return &dedupeOp{args: a}, nil
}

// Kind returns "dedupe".
func (op *dedupeOp) Kind() string { return KindDedupe }

// Apply keeps the first node encountered for each (Fields...) tuple. Order is
// preserved (first-seen wins). O(n) using a map.
func (op *dedupeOp) Apply(ctx context.Context, nodes []*substore.ParsedNode) ([]*substore.ParsedNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(nodes))
	out := make([]*substore.ParsedNode, 0, len(nodes))
	for _, n := range nodes {
		key, err := buildDedupeKey(n, op.args.Fields)
		if err != nil {
			return nil, fmt.Errorf("dedupe key: %w", err)
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, n)
	}
	return out, nil
}

// buildDedupeKey concatenates the field values with a NUL separator. The
// resulting string is opaque to callers and used purely for set membership.
func buildDedupeKey(node *substore.ParsedNode, fields []string) (string, error) {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		v, err := resolveField(node, f)
		if err != nil {
			return "", err
		}
		parts = append(parts, formatScalar(v))
	}
	return strings.Join(parts, "\x00"), nil
}

// formatScalar coerces any to a stable string representation. Nil → empty
// string; bools → "true"/"false"; ints → decimal; everything else → fmt %v.
func formatScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", v)
}
