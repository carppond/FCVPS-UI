package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"shiguang-vps/internal/substore"
)

// MapArgs are the parameters consumed by the map operator. The shape mirrors
// types.MapArgs: a single field/value pair. The value supports a small set of
// template variables ({{.Name}} / {{.Protocol}} / {{.Server}} / {{.Port}} /
// {{.Tag}} / {{.Index}}).
type MapArgs struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// mapOp implements Operator for the "map" kind.
type mapOp struct {
	args MapArgs
}

func init() { Register(KindMap, newMapOp) }

// allowedMapFields lists the writable fields. We intentionally limit the
// surface — silently writing into arbitrary Raw keys would invite surprises.
var allowedMapFields = map[string]struct{}{
	"name":     {},
	"server":   {},
	"port":     {},
	"tag":      {},
	"network":  {},
	"sni":      {},
	"host":     {},
	"path":     {},
	"password": {},
	"uuid":     {},
	"method":   {},
}

// newMapOp is the registered factory.
func newMapOp(raw json.RawMessage) (Operator, error) {
	var a MapArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("%w: map: %v", ErrInvalidArgs, err)
		}
	}
	if a.Field == "" {
		return nil, fmt.Errorf("%w: map.field required", ErrInvalidArgs)
	}
	if _, ok := allowedMapFields[a.Field]; !ok {
		return nil, fmt.Errorf("%w: map.field %q not writable", ErrInvalidArgs, a.Field)
	}
	return &mapOp{args: a}, nil
}

// Kind returns "map".
func (op *mapOp) Kind() string { return KindMap }

// Apply walks the slice, cloning each node and assigning the templated value
// to args.Field. Index expansion uses the position in the *output* slice (i.e.
// starts at 0 in display order).
func (op *mapOp) Apply(ctx context.Context, nodes []*substore.ParsedNode) ([]*substore.ParsedNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]*substore.ParsedNode, len(nodes))
	for i, src := range nodes {
		cp := cloneNode(src)
		val := renderTemplate(op.args.Value, cp, i)
		if err := assignField(cp, op.args.Field, val); err != nil {
			return nil, fmt.Errorf("map assign: %w", err)
		}
		out[i] = cp
	}
	return out, nil
}

// renderTemplate expands a tiny template language. Supported placeholders:
//
//	{{.Name}}  {{.Protocol}}  {{.Server}}  {{.Port}}  {{.Tag}}  {{.Index}}
//
// Unknown placeholders are left in place (so users see the typo).
func renderTemplate(tpl string, node *substore.ParsedNode, idx int) string {
	if !strings.Contains(tpl, "{{") {
		return tpl
	}
	r := strings.NewReplacer(
		"{{.Name}}", node.Name,
		"{{.Protocol}}", node.Protocol,
		"{{.Server}}", node.Server,
		"{{.Port}}", strconv.Itoa(node.Port),
		"{{.Tag}}", node.Tag,
		"{{.Index}}", strconv.Itoa(idx),
	)
	return r.Replace(tpl)
}

// assignField writes value into the named field of node. Numeric fields
// (port) accept the decimal string representation. Boolean fields are not
// currently writable.
func assignField(node *substore.ParsedNode, field, value string) error {
	switch field {
	case "name":
		node.Name = value
	case "server":
		node.Server = value
	case "port":
		p, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("port not numeric: %v", err)
		}
		node.Port = p
	case "tag":
		node.Tag = value
	case "network":
		node.Network = value
	case "sni":
		node.SNI = value
	case "host":
		node.Host = value
	case "path":
		node.Path = value
	case "password":
		node.Password = value
	case "uuid":
		node.UUID = value
	case "method":
		node.Method = value
	default:
		return fmt.Errorf("field %q not writable", field)
	}
	return nil
}
