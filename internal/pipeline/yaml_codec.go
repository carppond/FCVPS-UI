package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// yamlPipeline is the on-disk shape of a pipeline YAML file. We keep the
// field order canonical (api_version → name → operators) by reading and
// writing through dedicated structs (yaml.v3 respects struct field order
// when no custom Marshal* is wired).
type yamlPipeline struct {
	APIVersion string             `yaml:"apiVersion"`
	Name       string             `yaml:"name,omitempty"`
	Operators  []yamlOperatorSpec `yaml:"operators"`
}

type yamlOperatorSpec struct {
	Kind    string    `yaml:"kind"`
	Enabled *bool     `yaml:"enabled,omitempty"`
	Args    yaml.Node `yaml:"args"`
}

// EncodeYAML serialises ast to YAML bytes. APIVersion is forced to APIVersion;
// when caller-supplied value differs we substitute the canonical one (the AST
// is the source of truth, but we never let stale apiVersion slip through).
func EncodeYAML(ast *AST) ([]byte, error) {
	if ast == nil {
		return nil, fmt.Errorf("pipeline: nil AST")
	}
	doc := yamlPipeline{
		APIVersion: APIVersion,
		Name:       ast.Name,
		Operators:  make([]yamlOperatorSpec, 0, len(ast.Operators)),
	}
	for i, op := range ast.Operators {
		argsNode, err := jsonRawToYAMLNode(op.Args)
		if err != nil {
			return nil, fmt.Errorf("pipeline: step %d args: %w", i, err)
		}
		spec := yamlOperatorSpec{Kind: op.Kind, Args: argsNode}
		if !op.Enabled {
			// Only emit "enabled: false" — enabled-by-default keeps YAML lean.
			v := false
			spec.Enabled = &v
		}
		doc.Operators = append(doc.Operators, spec)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("yaml encode: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("yaml close: %w", err)
	}
	return buf.Bytes(), nil
}

// DecodeYAML parses YAML bytes into an AST. The apiVersion field is required
// and must equal APIVersion ("shiguang/v1"); other values return
// ErrSchemaMismatch.
func DecodeYAML(data []byte) (*AST, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("pipeline: empty YAML")
	}
	var doc yamlPipeline
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if doc.APIVersion != APIVersion {
		return nil, fmt.Errorf("%w: got %q, want %q", ErrSchemaMismatch,
			doc.APIVersion, APIVersion)
	}

	ast := &AST{APIVersion: APIVersion, Name: doc.Name}
	ast.Operators = make([]OperatorSpec, 0, len(doc.Operators))
	for i, op := range doc.Operators {
		argsJSON, err := yamlNodeToJSON(&op.Args)
		if err != nil {
			return nil, fmt.Errorf("pipeline: step %d args: %w", i, err)
		}
		enabled := true
		if op.Enabled != nil {
			enabled = *op.Enabled
		}
		ast.Operators = append(ast.Operators, OperatorSpec{
			Kind: op.Kind, Args: argsJSON, Enabled: enabled,
		})
	}
	return ast, nil
}

// jsonRawToYAMLNode converts a json.RawMessage into a yaml.Node. The
// decoder uses json.Unmarshal → map[string]any (or []any) → yaml.Node so we
// piggyback on the same structural mapping yaml.v3 uses internally.
func jsonRawToYAMLNode(raw json.RawMessage) (yaml.Node, error) {
	if len(raw) == 0 {
		return yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return yaml.Node{}, fmt.Errorf("unmarshal args: %w", err)
	}
	var node yaml.Node
	if err := node.Encode(v); err != nil {
		return yaml.Node{}, fmt.Errorf("encode yaml node: %w", err)
	}
	return node, nil
}

// yamlNodeToJSON renders a yaml.Node to a JSON byte slice. The traversal
// honours the canonical types (mappings → objects, sequences → arrays,
// scalars → strings/numbers/booleans/null).
func yamlNodeToJSON(node *yaml.Node) (json.RawMessage, error) {
	if node == nil || node.Kind == 0 {
		return json.RawMessage("{}"), nil
	}
	var v any
	if err := node.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode yaml node: %w", err)
	}
	if v == nil {
		return json.RawMessage("{}"), nil
	}
	v = normaliseJSON(v)
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal args json: %w", err)
	}
	return out, nil
}

// normaliseJSON walks the value tree turning map[interface{}]interface{}
// (yaml.v3 idiom) into map[string]interface{} so encoding/json accepts it.
func normaliseJSON(v any) any {
	switch x := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[fmt.Sprintf("%v", k)] = normaliseJSON(val)
		}
		return out
	case map[string]interface{}:
		for k, val := range x {
			x[k] = normaliseJSON(val)
		}
		return x
	case []interface{}:
		for i, val := range x {
			x[i] = normaliseJSON(val)
		}
		return x
	}
	return v
}
