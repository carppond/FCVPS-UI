package util

import (
	"bytes"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ProxyKeyOrder is the canonical key ordering for Clash proxy mapping nodes.
// Keys not listed are appended in their original order. The list is exported
// so external packages can extend it (e.g. for new protocols).
var ProxyKeyOrder = []string{
	"name",
	"type",
	"server",
	"port",
	"uuid",
	"password",
	"cipher",
	"network",
	"tls",
	"sni",
	"servername",
	"alpn",
	"skip-cert-verify",
	"udp",
	"ws-opts",
	"ws-path",
	"ws-headers",
	"grpc-opts",
	"reality-opts",
	"client-fingerprint",
	"flow",
	"plugin",
	"plugin-opts",
}

// Errors surfaced by yaml.go helpers.
var (
	// ErrYAMLNotMapping is returned when a helper expects a mapping node but
	// received another kind (sequence/scalar/document/...).
	ErrYAMLNotMapping = errors.New("yaml node is not a mapping")
	// ErrYAMLNilNode is returned when a nil *yaml.Node is supplied.
	ErrYAMLNilNode = errors.New("yaml node is nil")
)

// CloneNode performs a deep copy of a yaml.Node tree, preserving Tag, Style,
// HeadComment, LineComment, FootComment, Line, Column and all children. The
// returned tree shares no pointers with the input.
func CloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	out := &yaml.Node{
		Kind:        n.Kind,
		Style:       n.Style,
		Tag:         n.Tag,
		Value:       n.Value,
		Anchor:      n.Anchor,
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
		Line:        n.Line,
		Column:      n.Column,
	}
	// Anchors that referenced nodes must be cloned too. yaml.v3 surfaces this
	// via Alias; we deep-copy the target so equality stays structural-only.
	if n.Alias != nil {
		out.Alias = CloneNode(n.Alias)
	}
	if len(n.Content) > 0 {
		out.Content = make([]*yaml.Node, len(n.Content))
		for i, child := range n.Content {
			out.Content[i] = CloneNode(child)
		}
	}
	return out
}

// GetMappingValue looks up `key` in a mapping node and returns the value node.
// Returns ok=false when key is absent or n is nil / not a mapping. The
// returned pointer aliases the underlying tree; callers must Clone before
// mutating to avoid surprising the input.
func GetMappingValue(n *yaml.Node, key string) (*yaml.Node, bool) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return n.Content[i+1], true
		}
	}
	return nil, false
}

// SetMappingValue sets or replaces the value for `key` in mapping n.
//
// val may be:
//   - a *yaml.Node: used directly (NOT cloned; pass CloneNode(node) if needed).
//   - a Go scalar (string/bool/int*/uint*/float*): rendered as a plain or
//     quoted scalar matching the existing key's style when possible.
//   - nil: writes a null scalar.
//
// When key exists, only the value node is replaced; the key node and its
// comments are preserved. When key is absent, a new key/value pair is appended
// in canonical scalar style.
//
// Returns an error if n is nil or not a mapping; never reorders existing
// keys. Use ReorderProxyNode if reordering is desired.
func SetMappingValue(n *yaml.Node, key string, val any) error {
	if n == nil {
		return ErrYAMLNilNode
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("%w: got kind %d", ErrYAMLNotMapping, n.Kind)
	}
	valNode, err := toValueNode(val)
	if err != nil {
		return err
	}

	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			// Preserve key style; replace value but keep its comment slots
			// when the new node has none.
			old := n.Content[i+1]
			if valNode.HeadComment == "" {
				valNode.HeadComment = old.HeadComment
			}
			if valNode.LineComment == "" {
				valNode.LineComment = old.LineComment
			}
			if valNode.FootComment == "" {
				valNode.FootComment = old.FootComment
			}
			n.Content[i+1] = valNode
			return nil
		}
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	n.Content = append(n.Content, keyNode, valNode)
	return nil
}

// ReorderProxyNode reorders the keys of a Clash proxy mapping node so the
// "important" keys (name/type/server/port/uuid/cipher/...) appear in a
// canonical order. Unknown keys are kept in their original order and appended
// after the recognised ones. Comments are preserved with their owning pair.
//
// The function returns a new node (the input is not mutated) so callers can
// safely diff before vs after.
func ReorderProxyNode(n *yaml.Node) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return n
	}
	clone := CloneNode(n)

	type pair struct {
		key   *yaml.Node
		value *yaml.Node
	}
	original := make([]pair, 0, len(clone.Content)/2)
	for i := 0; i+1 < len(clone.Content); i += 2 {
		original = append(original, pair{key: clone.Content[i], value: clone.Content[i+1]})
	}

	// Index pairs by key value for O(1) lookup; remember unseen pairs to
	// append in original order at the end.
	index := make(map[string]int, len(original))
	used := make([]bool, len(original))
	for i, p := range original {
		if p.key.Kind == yaml.ScalarNode {
			if _, ok := index[p.key.Value]; !ok {
				index[p.key.Value] = i
			}
		}
	}

	reordered := make([]*yaml.Node, 0, len(clone.Content))
	for _, k := range ProxyKeyOrder {
		if idx, ok := index[k]; ok && !used[idx] {
			used[idx] = true
			reordered = append(reordered, original[idx].key, original[idx].value)
		}
	}
	for i, p := range original {
		if !used[i] {
			reordered = append(reordered, p.key, p.value)
		}
	}
	clone.Content = reordered
	return clone
}

// MarshalIndent serialises a yaml.Node back to bytes with 2-space indentation
// (project convention). Returns a non-nil error only on encoder failure.
func MarshalIndent(n *yaml.Node) ([]byte, error) {
	if n == nil {
		return nil, ErrYAMLNilNode
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("yaml encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("yaml encoder close: %w", err)
	}
	return buf.Bytes(), nil
}

// toValueNode converts a Go value into a yaml.Node suitable for assignment as
// a mapping value. Existing yaml.Node values pass through unchanged.
func toValueNode(val any) (*yaml.Node, error) {
	switch v := val.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case *yaml.Node:
		if v == nil {
			return nil, ErrYAMLNilNode
		}
		return v, nil
	case yaml.Node:
		return CloneNode(&v), nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}, nil
	case bool:
		if v {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}, nil
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}, nil
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		// Defer formatting to yaml.v3 by marshalling the scalar.
		raw, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal scalar: %w", err)
		}
		var node yaml.Node
		if err := yaml.Unmarshal(raw, &node); err != nil {
			return nil, fmt.Errorf("unmarshal scalar: %w", err)
		}
		// node here is a document; take its first child.
		if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
			return node.Content[0], nil
		}
		return &node, nil
	default:
		return nil, fmt.Errorf("toValueNode: unsupported go type %T", val)
	}
}
