package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"shiguang-vps/internal/substore"
)

// RegexRenameArgs are the parameters consumed by the regex_rename operator.
// The shape mirrors types.RegexRenameArgs.
type RegexRenameArgs struct {
	// Pattern is a Go RE2 regex applied to the target field.
	Pattern string `json:"pattern"`

	// Replacement is the replacement template (supports $1 / ${name} as in
	// regexp.ReplaceAllString).
	Replacement string `json:"replacement"`

	// Field selects the renaming target. Defaults to "name" when empty.
	// Only "name" / "tag" are supported in v1.
	Field string `json:"field,omitempty"`
}

// regexRenameOp implements Operator for the "regex_rename" kind.
type regexRenameOp struct {
	args RegexRenameArgs
	re   *regexp.Regexp
}

func init() { Register(KindRegexRename, newRegexRenameOp) }

// newRegexRenameOp is the registered factory. Eagerly compiles the regex —
// invalid patterns are caught at AST load (rather than per-node at run time).
func newRegexRenameOp(raw json.RawMessage) (Operator, error) {
	var a RegexRenameArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("%w: regex_rename: %v", ErrInvalidArgs, err)
		}
	}
	if a.Pattern == "" {
		return nil, fmt.Errorf("%w: regex_rename.pattern required", ErrInvalidArgs)
	}
	if a.Field == "" {
		a.Field = "name"
	}
	if a.Field != "name" && a.Field != "tag" {
		return nil, fmt.Errorf("%w: regex_rename.field must be name/tag", ErrInvalidArgs)
	}
	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRegex, err)
	}
	return &regexRenameOp{args: a, re: re}, nil
}

// Kind returns "regex_rename".
func (op *regexRenameOp) Kind() string { return KindRegexRename }

// Apply runs ReplaceAllString on each node's target field. Nodes that do not
// match the pattern pass through unchanged (so users can chain renames safely).
func (op *regexRenameOp) Apply(ctx context.Context, nodes []*substore.ParsedNode) ([]*substore.ParsedNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]*substore.ParsedNode, len(nodes))
	for i, src := range nodes {
		cp := cloneNode(src)
		switch op.args.Field {
		case "name":
			cp.Name = op.re.ReplaceAllString(cp.Name, op.args.Replacement)
		case "tag":
			cp.Tag = op.re.ReplaceAllString(cp.Tag, op.args.Replacement)
		}
		out[i] = cp
	}
	return out, nil
}
