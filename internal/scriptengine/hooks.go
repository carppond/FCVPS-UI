package scriptengine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// HookResult bundles the output of a hook invocation. Engine.Run already
// returns the canonical RunResult; HookResult adds typed helpers callers can
// reach for when they only want one of the channels.
type HookResult struct {
	// JSONBody is the user-script's __output serialized to JSON. Empty
	// when the script left __output unset.
	JSONBody string
	// Logs is every console.* line emitted, prefixed with [level].
	Logs []string
	// DurationMS is wall-clock elapsed time.
	DurationMS int64
}

// RunPreSaveNodes runs the user code against an opaque node list. nodes
// is encoded to a JSON array under __input.nodes; the script is expected
// to assign its modified list to __output.
//
// The function is intentionally generic: it does NOT depend on a specific
// node type so internal/substore can pass *ParsedNode while tests can pass
// map[string]any payloads. Callers JSON-decode the returned JSONBody into
// their own type.
//
// Example user script:
//
//	const out = __input.nodes.filter(n => n.protocol === 'ss');
//	__output = out;
func (e *Engine) RunPreSaveNodes(ctx context.Context, code string, nodes any, timeout time.Duration) (*HookResult, error) {
	if code == "" {
		return nil, fmt.Errorf("scriptengine: empty hook code")
	}
	input := map[string]any{"nodes": nodes}
	res, err := e.Run(ctx, code, input, timeout)
	if err != nil {
		if res != nil {
			return &HookResult{Logs: res.Logs, DurationMS: res.DurationMS}, err
		}
		return nil, err
	}
	return &HookResult{
		JSONBody:   res.Output,
		Logs:       res.Logs,
		DurationMS: res.DurationMS,
	}, nil
}

// RunPostFetch runs user code against the raw subscription body (typically
// YAML or base64). The string is exposed via __input.raw; user code returns
// the transformed string by assigning to __output (the engine still
// stringifies — assigning a JS string yields the string as-is).
//
// We pass rawContent as a string so user code can treat it as text directly
// (most subscriptions are UTF-8 YAML). When the source is base64 the
// substore caller is expected to decode first.
func (e *Engine) RunPostFetch(ctx context.Context, code string, rawContent string, timeout time.Duration) (string, []string, int64, error) {
	if code == "" {
		return "", nil, 0, fmt.Errorf("scriptengine: empty hook code")
	}
	input := map[string]any{"raw": rawContent}
	res, err := e.Run(ctx, code, input, timeout)
	if err != nil {
		if res != nil {
			return "", res.Logs, res.DurationMS, err
		}
		return "", nil, 0, err
	}
	// The script may have assigned a JS string to __output. Engine.Run
	// returns the raw string when that is the case; otherwise it returns
	// a JSON-serialised object. We surface both flavours: callers expecting
	// "text" will get the JSON literal of an object (e.g. `"foo"`), so we
	// unwrap a leading/trailing quote pair when possible.
	return unwrapJSONString(res.Output), res.Logs, res.DurationMS, nil
}

// unwrapJSONString peels the JSON encoding off a single string value so
// RunPostFetch callers see the literal payload, not its quoted form. When
// the input is not a JSON string (e.g. the script returned the raw text
// directly) we return it unchanged.
func unwrapJSONString(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	var out string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return s
	}
	return out
}
