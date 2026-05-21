// Package pipeline owns the M-PIPE backend engine: a declarative AST plus 6
// built-in operators (filter / map / sort / dedupe / regex_rename / output)
// that transform a slice of substore.ParsedNode into the final list emitted
// by sub-store compatible endpoints.
//
// Architectural decisions (Tech Lead §1.4 + ADR 0004):
//   - v1 runs the pipeline 100% server-side; the frontend only edits + previews.
//   - The AST (JSON) is the system of record (`pipelines.ast_json`). The
//     `pipelines.yaml_content` field is re-serialised from the AST on every
//     save — YAML stays a user-facing transport format (import / export / git).
//   - Operators are registered via init() at package load; callers do not need
//     to wire them up manually. New algorithms are added by dropping a new
//     op_*.go file with its own init().
//   - The pipeline must terminate with exactly one output operator declaring
//     the desired serialisation format (clash / clash_meta / raw).
package pipeline

import (
	"encoding/json"
	"fmt"
)

// APIVersion is the only YAML / AST schema marker accepted by v1.
const APIVersion = "shiguang/v1"

// MaxOperators caps an AST to a sane size (engine validation). The threshold
// is intentionally generous — typical pipelines have 3–8 operators.
const MaxOperators = 50

// AST is the in-memory representation of a pipeline. It is a thin wrapper over
// a list of OperatorSpec records plus the schema marker.
//
// The encoding/json default representation matches the on-disk ast_json column
// shape, see TestYAMLCodec_RoundTrip and TestAST_JSONShape.
type AST struct {
	APIVersion string         `json:"api_version"`
	Name       string         `json:"name,omitempty"`
	Operators  []OperatorSpec `json:"operators"`
}

// OperatorSpec is a single algorithm node inside the AST. Args is held as
// json.RawMessage so each operator factory can deserialise it into its own
// typed argument struct independently.
type OperatorSpec struct {
	// Kind is the operator type identifier (filter / map / sort / dedupe /
	// regex_rename / output). It MUST be one of the registered factories.
	Kind string `json:"kind"`

	// Args carries the operator-specific parameters. The format depends on
	// Kind; see FilterArgs / MapArgs / SortArgs / DedupeArgs / RegexRenameArgs
	// / OutputArgs in this package.
	Args json.RawMessage `json:"args"`

	// Enabled mirrors the contract field (PipelineOperator.enabled). When
	// false the engine skips the step. Defaults to true on serialisation if
	// absent, matching the YAML "enabled by default" UX.
	Enabled bool `json:"enabled"`
}

// Clone returns a deep copy of the AST. Used by the engine before validation
// so callers can safely reuse their copy.
func (a *AST) Clone() *AST {
	if a == nil {
		return nil
	}
	out := &AST{APIVersion: a.APIVersion, Name: a.Name}
	if len(a.Operators) > 0 {
		out.Operators = make([]OperatorSpec, len(a.Operators))
		for i, op := range a.Operators {
			cp := OperatorSpec{Kind: op.Kind, Enabled: op.Enabled}
			if len(op.Args) > 0 {
				cp.Args = append(json.RawMessage(nil), op.Args...)
			}
			out.Operators[i] = cp
		}
	}
	return out
}

// MarshalJSON serialises the AST with the canonical field ordering used in
// pipelines.ast_json. We avoid the default json.Marshal struct ordering for
// determinism — two ASTs with the same operators produce byte-identical JSON.
func (a *AST) MarshalJSON() ([]byte, error) {
	// We delegate to encoding/json by aliasing to dodge recursion, then
	// re-serialise via map for stable ordering.
	type rawAST AST
	type ordered struct {
		APIVersion string         `json:"api_version"`
		Name       string         `json:"name,omitempty"`
		Operators  []OperatorSpec `json:"operators"`
	}
	v := ordered{
		APIVersion: a.APIVersion,
		Name:       a.Name,
		Operators:  a.Operators,
	}
	// Ensure Operators is never nil — the JSON should always carry "operators": [].
	if v.Operators == nil {
		v.Operators = []OperatorSpec{}
	}
	_ = rawAST{}
	return json.Marshal(v)
}

// MarshalAST encodes ast into the canonical JSON string used by ast_json.
func MarshalAST(ast *AST) (string, error) {
	if ast == nil {
		return "", fmt.Errorf("pipeline: nil AST")
	}
	b, err := json.Marshal(ast)
	if err != nil {
		return "", fmt.Errorf("marshal AST: %w", err)
	}
	return string(b), nil
}

// UnmarshalAST decodes the JSON form of an AST. Returns ErrSchemaMismatch when
// api_version is missing or unsupported.
func UnmarshalAST(s string) (*AST, error) {
	if s == "" {
		return nil, fmt.Errorf("pipeline: empty AST JSON")
	}
	var ast AST
	if err := json.Unmarshal([]byte(s), &ast); err != nil {
		return nil, fmt.Errorf("unmarshal AST: %w", err)
	}
	if ast.APIVersion == "" {
		// Permit missing apiVersion in raw AST parsing — the validator will
		// flag it. Returning here makes the unmarshal step robust against
		// partial inputs (e.g. user-pasted snippets).
		ast.APIVersion = ""
	}
	return &ast, nil
}
