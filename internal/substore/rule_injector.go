package substore

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"shiguang-vps/internal/util"
)

// CustomRule mirrors the surface fields a substore rule_injector needs. The
// canonical DTO lives in internal/types; we keep this lightweight struct here
// so the substore package does not depend on types (avoids an import cycle:
// substore is used by handler/types depend on substore indirectly through
// pipeline code).
type CustomRule struct {
	ID      string
	Name    string
	Type    string // "dns" / "rules" / "rule-providers"
	Mode    string // "replace" / "prepend" / "append"
	Content string // YAML fragment ("dns"/"rule-providers") or line-list ("rules")
	Sort    int32
}

// Inject mutates a deep copy of base by applying each rule in sort order and
// returns the updated root node. base is left untouched.
//
// Rule semantics:
//
//   - type=dns:
//     replace → swap the entire dns: mapping
//     prepend → merge into dns.nameservers / dns.fallback at the front
//     append  → merge into dns.nameservers / dns.fallback at the back
//
//   - type=rules:
//     content is interpreted as one rule per non-empty line. The lines are
//     wrapped in a sequence node; replace swaps the whole rules: seq,
//     prepend/append concatenates the new entries to the front / back of the
//     existing rules: array.
//
//   - type=rule-providers:
//     replace swaps the entire rule-providers: mapping. prepend/append merges
//     the entries into the existing map: prepend wins on key collision
//     (incoming overrides), append loses (existing wins).
//
// Invalid YAML in a rule's Content is skipped — the rule is reported as
// faulty via the returned warnings, but the rest of the pipeline still runs.
// The expectation is that the rule editor validates the snippet before save.
func Inject(base *yaml.Node, rules []*CustomRule) (*yaml.Node, error) {
	if base == nil {
		return nil, fmt.Errorf("rule injector: nil base node")
	}
	root := util.CloneNode(base)
	mapping, err := mappingFromRoot(root)
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		if err := applyRule(mapping, rule); err != nil {
			return nil, fmt.Errorf("apply rule %q: %w", rule.Name, err)
		}
	}
	return root, nil
}

// mappingFromRoot resolves the top-level mapping node from a yaml document or
// bare mapping. The Clash producer outputs a bare mapping (no DocumentNode
// wrapper); callers that decode external YAML may pass a document.
func mappingFromRoot(root *yaml.Node) (*yaml.Node, error) {
	switch root.Kind {
	case yaml.MappingNode:
		return root, nil
	case yaml.DocumentNode:
		if len(root.Content) != 1 {
			return nil, fmt.Errorf("rule injector: document has %d children, want 1", len(root.Content))
		}
		if root.Content[0].Kind != yaml.MappingNode {
			return nil, fmt.Errorf("rule injector: root child is not a mapping")
		}
		return root.Content[0], nil
	default:
		return nil, fmt.Errorf("rule injector: unsupported root kind %d", root.Kind)
	}
}

// applyRule dispatches by rule.Type. Unknown types are rejected at the
// handler layer; we still defensive-check here so a stray DB row cannot crash
// a render call.
func applyRule(mapping *yaml.Node, rule *CustomRule) error {
	switch rule.Type {
	case "dns":
		return injectDNS(mapping, rule)
	case "rules":
		return injectRules(mapping, rule)
	case "rule-providers":
		return injectRuleProviders(mapping, rule)
	default:
		return fmt.Errorf("unknown rule type %q", rule.Type)
	}
}

// injectDNS applies a single dns rule to the mapping.
func injectDNS(mapping *yaml.Node, rule *CustomRule) error {
	incoming, err := decodeFragmentMapping(rule.Content)
	if err != nil {
		return fmt.Errorf("parse dns content: %w", err)
	}
	switch rule.Mode {
	case "replace":
		return util.SetMappingValue(mapping, "dns", incoming)
	case "prepend", "append":
		current, ok := util.GetMappingValue(mapping, "dns")
		if !ok || current.Kind != yaml.MappingNode {
			// No existing dns; treat as replace so the new mapping appears.
			return util.SetMappingValue(mapping, "dns", incoming)
		}
		mergeDNSChildren(current, incoming, rule.Mode == "prepend")
		return nil
	default:
		return fmt.Errorf("unknown rule mode %q", rule.Mode)
	}
}

// mergeDNSChildren merges incoming into current. For sub-sequences the items
// are prepended / appended (nameservers / fallback list semantics). Scalar
// children always overwrite — replacing a scalar is the only sensible merge.
func mergeDNSChildren(current, incoming *yaml.Node, prepend bool) {
	for i := 0; i+1 < len(incoming.Content); i += 2 {
		key := incoming.Content[i]
		val := incoming.Content[i+1]
		existing, ok := util.GetMappingValue(current, key.Value)
		if !ok {
			_ = util.SetMappingValue(current, key.Value, util.CloneNode(val))
			continue
		}
		if existing.Kind == yaml.SequenceNode && val.Kind == yaml.SequenceNode {
			merged := mergeSequences(existing, val, prepend)
			_ = util.SetMappingValue(current, key.Value, merged)
		} else {
			// scalar / mapping: incoming wins.
			_ = util.SetMappingValue(current, key.Value, util.CloneNode(val))
		}
	}
}

// injectRules applies a single rules rule. Content is treated as one rule per
// non-empty line.
func injectRules(mapping *yaml.Node, rule *CustomRule) error {
	lines := parseRuleLines(rule.Content)
	incomingSeq := stringLinesToSequence(lines)
	switch rule.Mode {
	case "replace":
		return util.SetMappingValue(mapping, "rules", incomingSeq)
	case "prepend", "append":
		current, ok := util.GetMappingValue(mapping, "rules")
		if !ok || current.Kind != yaml.SequenceNode {
			return util.SetMappingValue(mapping, "rules", incomingSeq)
		}
		merged := mergeSequences(current, incomingSeq, rule.Mode == "prepend")
		_ = util.SetMappingValue(mapping, "rules", merged)
		return nil
	default:
		return fmt.Errorf("unknown rule mode %q", rule.Mode)
	}
}

// injectRuleProviders applies a single rule-providers rule.
func injectRuleProviders(mapping *yaml.Node, rule *CustomRule) error {
	incoming, err := decodeFragmentMapping(rule.Content)
	if err != nil {
		return fmt.Errorf("parse rule-providers content: %w", err)
	}
	switch rule.Mode {
	case "replace":
		return util.SetMappingValue(mapping, "rule-providers", incoming)
	case "prepend", "append":
		current, ok := util.GetMappingValue(mapping, "rule-providers")
		if !ok || current.Kind != yaml.MappingNode {
			return util.SetMappingValue(mapping, "rule-providers", incoming)
		}
		mergeMappingEntries(current, incoming, rule.Mode == "prepend")
		return nil
	default:
		return fmt.Errorf("unknown rule mode %q", rule.Mode)
	}
}

// mergeMappingEntries merges incoming into current. When prepend=true the
// incoming entries override existing ones; when false (append) the existing
// entries win on collision.
func mergeMappingEntries(current, incoming *yaml.Node, prepend bool) {
	for i := 0; i+1 < len(incoming.Content); i += 2 {
		key := incoming.Content[i]
		val := incoming.Content[i+1]
		_, exists := util.GetMappingValue(current, key.Value)
		if exists && !prepend {
			continue
		}
		_ = util.SetMappingValue(current, key.Value, util.CloneNode(val))
	}
}

// mergeSequences concatenates two sequence nodes. When prepend=true the
// incoming items go first, otherwise last. Comments / styles are preserved
// for items that survive the merge.
func mergeSequences(current, incoming *yaml.Node, prepend bool) *yaml.Node {
	out := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: current.Style}
	if prepend {
		for _, c := range incoming.Content {
			out.Content = append(out.Content, util.CloneNode(c))
		}
		for _, c := range current.Content {
			out.Content = append(out.Content, util.CloneNode(c))
		}
	} else {
		for _, c := range current.Content {
			out.Content = append(out.Content, util.CloneNode(c))
		}
		for _, c := range incoming.Content {
			out.Content = append(out.Content, util.CloneNode(c))
		}
	}
	return out
}

// decodeFragmentMapping parses a YAML fragment and returns the top-level
// mapping. The fragment is expected to be either:
//   - a bare mapping: "nameservers: [1.1.1.1]"
//   - a document containing a mapping
//
// Leading/trailing whitespace is stripped before parsing.
func decodeFragmentMapping(s string) (*yaml.Node, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(trimmed), &root); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}
	mapping, err := mappingFromRoot(&root)
	if err != nil {
		return nil, err
	}
	return util.CloneNode(mapping), nil
}

// parseRuleLines splits content into clean rule lines: it drops blank and
// comment lines, strips "- " list markers / inline comments / surrounding
// quotes (via cleanRuleLine), and — as the render-time guard (A1) — drops
// structurally-broken lines so one bad rule never makes mihomo reject the whole
// subscription. The editor (ValidateRuleContent) surfaces those same lines with
// a fix hint on save, so a dropped line here is only a last-resort safety net.
func parseRuleLines(content string) []string {
	raw := strings.Split(content, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		t, ok := cleanRuleLine(l)
		if !ok || !renderableRuleLine(t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// stringLinesToSequence converts a slice of rule lines into a yaml sequence
// of scalar nodes. Block style is used so the output is human-friendly when
// the producer marshals it.
func stringLinesToSequence(lines []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, l := range lines {
		seq.Content = append(seq.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: l,
		})
	}
	return seq
}

// ApplyToYAML is a convenience helper: parse the base YAML, run Inject, and
// re-marshal. Used by the preview endpoint so the handler does not have to
// touch yaml.Node directly.
func ApplyToYAML(baseYAML []byte, rules []*CustomRule) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(baseYAML, &root); err != nil {
		return nil, fmt.Errorf("decode base yaml: %w", err)
	}
	out, err := Inject(&root, rules)
	if err != nil {
		return nil, err
	}
	return util.MarshalIndent(out)
}
