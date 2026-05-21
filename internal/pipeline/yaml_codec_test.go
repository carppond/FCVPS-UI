package pipeline

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const sampleYAML = `apiVersion: shiguang/v1
name: my-pipeline
operators:
  - kind: filter
    args:
      expr: protocol in ["vmess","vless"]
  - kind: sort
    args:
      key: name
      order: asc
  - kind: dedupe
    args:
      fields: [server, port]
  - kind: output
    args:
      format: clash
`

func TestDecodeYAML_ValidatesSchemaVersion(t *testing.T) {
	bad := strings.Replace(sampleYAML, "shiguang/v1", "bogus/v9", 1)
	_, err := DecodeYAML([]byte(bad))
	if !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("want ErrSchemaMismatch, got %v", err)
	}
}

func TestDecodeYAML_HappyPath(t *testing.T) {
	ast, err := DecodeYAML([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ast.APIVersion != APIVersion {
		t.Fatalf("apiVersion: %q", ast.APIVersion)
	}
	if ast.Name != "my-pipeline" {
		t.Fatalf("name: %q", ast.Name)
	}
	if len(ast.Operators) != 4 {
		t.Fatalf("want 4 operators, got %d", len(ast.Operators))
	}
	if ast.Operators[0].Kind != KindFilter {
		t.Fatalf("first kind: %s", ast.Operators[0].Kind)
	}
	// Args is decoded JSON.
	var fa FilterArgs
	if err := json.Unmarshal(ast.Operators[0].Args, &fa); err != nil {
		t.Fatalf("filter args: %v", err)
	}
	if !strings.Contains(fa.Expr, "vmess") {
		t.Fatalf("filter expr: %q", fa.Expr)
	}
}

func TestEncodeYAML_ProducesValidYAML(t *testing.T) {
	ast := &AST{
		APIVersion: APIVersion,
		Name:       "round",
		Operators: []OperatorSpec{
			{Kind: KindFilter, Args: mustJSON(FilterArgs{Expr: "true"}), Enabled: true},
			{Kind: KindOutput, Args: mustJSON(OutputArgs{Format: "clash"}), Enabled: true},
		},
	}
	out, err := EncodeYAML(ast)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "apiVersion: shiguang/v1") {
		t.Fatalf("missing apiVersion: %s", got)
	}
	if !strings.Contains(got, "kind: filter") || !strings.Contains(got, "kind: output") {
		t.Fatalf("missing kind: %s", got)
	}
	if !strings.Contains(got, "format: clash") {
		t.Fatalf("missing format: %s", got)
	}
}

func TestYAMLCodec_RoundTripPreservesShape(t *testing.T) {
	original := &AST{
		APIVersion: APIVersion,
		Name:       "rt",
		Operators: []OperatorSpec{
			{Kind: KindFilter, Args: mustJSON(FilterArgs{Expr: `region in ["hk","jp"]`}), Enabled: true},
			{Kind: KindMap, Args: mustJSON(MapArgs{Field: "name", Value: "{{.Name}}-x"}), Enabled: true},
			{Kind: KindSort, Args: mustJSON(SortArgs{Key: "name", Order: "asc"}), Enabled: true},
			{Kind: KindDedupe, Args: mustJSON(DedupeArgs{Fields: []string{"server", "port"}}), Enabled: true},
			{Kind: KindRegexRename, Args: mustJSON(RegexRenameArgs{Pattern: `\s+`, Replacement: "-"}), Enabled: true},
			{Kind: KindOutput, Args: mustJSON(OutputArgs{Format: "clash"}), Enabled: true},
		},
	}
	yamlBytes, err := EncodeYAML(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	roundTripped, err := DecodeYAML(yamlBytes)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if roundTripped.APIVersion != original.APIVersion {
		t.Fatalf("apiVersion drift: %q vs %q", roundTripped.APIVersion, original.APIVersion)
	}
	if roundTripped.Name != original.Name {
		t.Fatalf("name drift: %q vs %q", roundTripped.Name, original.Name)
	}
	if len(roundTripped.Operators) != len(original.Operators) {
		t.Fatalf("operator count drift: %d vs %d", len(roundTripped.Operators), len(original.Operators))
	}
	for i := range original.Operators {
		if roundTripped.Operators[i].Kind != original.Operators[i].Kind {
			t.Fatalf("kind drift at %d: %s vs %s", i, roundTripped.Operators[i].Kind, original.Operators[i].Kind)
		}
		// Args round-trip via re-unmarshal — strict byte equality is too strict
		// (JSON object key order may differ); compare semantic equality.
		var a, b map[string]any
		_ = json.Unmarshal(roundTripped.Operators[i].Args, &a)
		_ = json.Unmarshal(original.Operators[i].Args, &b)
		if !mapsEqual(a, b) {
			t.Fatalf("args drift at %d: %v vs %v", i, a, b)
		}
	}
}

func TestYAMLCodec_DisabledFlagPreserved(t *testing.T) {
	ast := &AST{
		APIVersion: APIVersion,
		Operators: []OperatorSpec{
			{Kind: KindFilter, Args: mustJSON(FilterArgs{Expr: "true"}), Enabled: false},
			{Kind: KindOutput, Args: mustJSON(OutputArgs{Format: "clash"}), Enabled: true},
		},
	}
	yamlBytes, _ := EncodeYAML(ast)
	if !strings.Contains(string(yamlBytes), "enabled: false") {
		t.Fatalf("expected enabled: false in YAML output:\n%s", yamlBytes)
	}
	roundTripped, err := DecodeYAML(yamlBytes)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if roundTripped.Operators[0].Enabled {
		t.Fatalf("disabled flag lost on decode")
	}
	if !roundTripped.Operators[1].Enabled {
		t.Fatalf("default-enabled lost on decode")
	}
}

func TestDecodeYAML_EmptyInputRejected(t *testing.T) {
	if _, err := DecodeYAML(nil); err == nil {
		t.Fatalf("expected error on empty input")
	}
}

func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		// Comparing nested arrays / objects is tedious; rely on string repr.
		ja, _ := json.Marshal(va)
		jb, _ := json.Marshal(vb)
		if string(ja) != string(jb) {
			return false
		}
	}
	return true
}
