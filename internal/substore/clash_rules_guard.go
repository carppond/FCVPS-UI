package substore

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// dropRulesMissingProviders removes RULE-SET rules whose referenced provider
// is absent from the rendered `rule-providers:` mapping. mihomo validates the
// reference at load time and rejects the whole document otherwise
// ("rules[N] [rule-set, X, ...] error: rule set [X] no found"), which bricks
// the subscription for the client. Unlike a missing proxy group (see
// ensureMissingProxyGroups) a provider cannot be auto-created — we do not know
// its URL — so dropping the broken rule is the only way to keep the rendered
// config bootable.
//
// Typical trigger: the user applied a rule template that references a preset
// rule-set (e.g. "RULE-SET,google,📢 Google") without adding / enabling the
// matching rule-set record. Provider names are matched case-sensitively, the
// same way mihomo resolves them.
//
// Must run after Inject (custom rules are in place) and before
// ensureMissingProxyGroups so a dropped rule does not auto-create its
// now-unreferenced target group.
func dropRulesMissingProviders(root *yaml.Node, opts ClashProducerOpts) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	var rulesNode, providersNode *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "rules":
			rulesNode = root.Content[i+1]
		case "rule-providers":
			providersNode = root.Content[i+1]
		}
	}
	if rulesNode == nil || rulesNode.Kind != yaml.SequenceNode {
		return
	}

	// Collect the provider names actually emitted. A missing / non-mapping
	// rule-providers section yields an empty set, so every RULE-SET rule is
	// considered broken (mihomo would reject each of them anyway).
	known := make(map[string]bool)
	if providersNode != nil && providersNode.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(providersNode.Content); i += 2 {
			if providersNode.Content[i].Kind == yaml.ScalarNode {
				known[providersNode.Content[i].Value] = true
			}
		}
	}

	kept := make([]*yaml.Node, 0, len(rulesNode.Content))
	for _, r := range rulesNode.Content {
		if r.Kind == yaml.ScalarNode {
			if name, ok := ruleSetProviderRef(r.Value); ok && !known[name] {
				if opts.OnWarning != nil {
					// node is nil here: the warning concerns a rule, not a proxy.
					opts.OnWarning(nil, fmt.Sprintf(
						"rule dropped: RULE-SET %q has no enabled rule-set provider: %s",
						name, r.Value))
				}
				continue
			}
		}
		kept = append(kept, r)
	}
	rulesNode.Content = kept
}

// ruleSetProviderRef extracts the provider name from a "RULE-SET,<name>,..."
// rule line. ok is false for every other rule type. The type token is matched
// case-insensitively (mihomo normalises it), the provider name is returned
// verbatim apart from surrounding whitespace.
func ruleSetProviderRef(rule string) (string, bool) {
	parts := strings.SplitN(rule, ",", 3)
	if len(parts) < 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "RULE-SET") {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}
