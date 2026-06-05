package substore

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestProduceClashYAML_DropsRuleSetWithoutProvider reproduces the CMFA import
// failure "rules[N] [rule-set, google, google] error: rule set [google] no
// found": a custom rule references a rule-set the user never added / enabled.
// The producer must drop the broken rule (keeping the valid ones) so the
// rendered document still boots.
func TestProduceClashYAML_DropsRuleSetWithoutProvider(t *testing.T) {
	input := &ClashRenderInput{
		Nodes: []*ParsedNode{
			{Name: "node-a", Protocol: "ss", Server: "1.1.1.1", Port: 80,
				Method: "aes-256-gcm", Password: "pw"},
		},
		RuleSets: []RuleSetRecord{
			{
				Name: "cn-domain", Behavior: "domain", Format: "mrs",
				URL: "https://example.com/cn.mrs", IntervalSeconds: 86400,
			},
		},
		CustomRules: []CustomRuleRecord{
			{
				Name: "user-rules", Type: "rules", Mode: "prepend", Sort: 1,
				Content: "RULE-SET,cn-domain,DIRECT\nRULE-SET,google,📢 Google",
			},
		},
	}
	var warned []string
	out, err := ProduceClashYAML(input, ClashProducerOpts{
		OnWarning: func(n *ParsedNode, reason string) {
			if n != nil {
				t.Errorf("rule-level warning should carry a nil node, got %v", n)
			}
			warned = append(warned, reason)
		},
	})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	rules := decodeRulesList(t, out)
	got := map[string]bool{}
	for _, r := range rules {
		got[r] = true
	}
	if !got["RULE-SET,cn-domain,DIRECT"] {
		t.Errorf("rule with an existing provider must survive, got %v", rules)
	}
	if got["RULE-SET,google,📢 Google"] {
		t.Errorf("rule with a missing provider must be dropped, got %v", rules)
	}
	if len(warned) != 1 || !strings.Contains(warned[0], `"google"`) {
		t.Errorf("expected one warning naming the missing provider, got %v", warned)
	}
	// The dropped rule's target group must not be auto-created: without the
	// rule nothing references 📢 Google any more.
	var doc struct {
		Groups []map[string]any `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	for _, g := range doc.Groups {
		if g["name"] == "📢 Google" {
			t.Errorf("target group of a dropped rule must not be auto-created, got %v", doc.Groups)
		}
	}
}

// TestProduceClashYAML_DropsAllRuleSetRulesWhenNoProviders covers the
// degenerate case: the user has zero enabled rule-sets, so `rule-providers:`
// is omitted entirely and every RULE-SET rule must go. The MATCH tail and
// non-RULE-SET rules survive.
func TestProduceClashYAML_DropsAllRuleSetRulesWhenNoProviders(t *testing.T) {
	input := &ClashRenderInput{
		CustomRules: []CustomRuleRecord{
			{
				Name: "user-rules", Type: "rules", Mode: "prepend", Sort: 1,
				Content: "RULE-SET,google,📢 Google\nDOMAIN-SUFFIX,example.com,DIRECT",
			},
		},
	}
	out, err := ProduceClashYAML(input, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	rules := decodeRulesList(t, out)
	for _, r := range rules {
		if strings.HasPrefix(r, "RULE-SET,") {
			t.Errorf("no providers configured — every RULE-SET rule must be dropped, got %v", rules)
		}
	}
	got := map[string]bool{}
	for _, r := range rules {
		got[r] = true
	}
	if !got["DOMAIN-SUFFIX,example.com,DIRECT"] {
		t.Errorf("non-RULE-SET rule must survive, got %v", rules)
	}
	if rules[len(rules)-1] != "MATCH,🚀 节点选择" {
		t.Errorf("MATCH tail must survive as the last rule, got %v", rules)
	}
}

// TestRuleSetProviderRef pins the parser used by the guard: type token is
// case-insensitive, provider name is trimmed, other rule types do not match.
func TestRuleSetProviderRef(t *testing.T) {
	cases := []struct {
		rule string
		name string
		ok   bool
	}{
		{"RULE-SET,google,📢 Google", "google", true},
		{"rule-set,google,DIRECT", "google", true},
		{"RULE-SET, cn-domain ,DIRECT,no-resolve", "cn-domain", true},
		{"RULE-SET,solo", "solo", true},
		{"DOMAIN-SUFFIX,google.com,DIRECT", "", false},
		{"MATCH,🚀 节点选择", "", false},
		{"GEOIP,CN,DIRECT", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		name, ok := ruleSetProviderRef(c.rule)
		if name != c.name || ok != c.ok {
			t.Errorf("ruleSetProviderRef(%q) = (%q, %v), want (%q, %v)",
				c.rule, name, ok, c.name, c.ok)
		}
	}
}
