package substore

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// decodeRulesList parses the YAML body produced by ProduceClashYAML and
// returns the `rules:` section as a []string. Tests use this to make
// assertions independent of YAML's emoji-escaping choices (gopkg.in/yaml.v3
// double-quotes scalars that start with non-ASCII characters, so a literal
// substring search on the rendered bytes is fragile).
func decodeRulesList(t *testing.T, body []byte) []string {
	t.Helper()
	var doc struct {
		Rules []string `yaml:"rules"`
	}
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode rules: %v\n%s", err, body)
	}
	return doc.Rules
}

func TestProduceClashYAML_BasicShape(t *testing.T) {
	nodes := []*ParsedNode{
		{Name: "n1", Protocol: "vmess", Server: "a.example.com", Port: 443, UUID: "uuid1", Network: "ws", TLS: true},
		{Name: "n2", Protocol: "ss", Server: "b.example.com", Port: 8388, Method: "aes-256-gcm", Password: "pw"},
		{Name: "n3", Protocol: "trojan", Server: "c.example.com", Port: 443, Password: "pw", TLS: true},
		{
			Name: "n4", Protocol: "vless", Server: "d.example.com", Port: 443,
			UUID: "uuid4", Network: "tcp", TLS: true, Reality: true,
			SNI: "www.microsoft.com",
			Raw: map[string]interface{}{
				"flow": "xtls-rprx-vision",
				"fp":   "chrome",
				"pbk":  "PUBKEY",
				"sid":  "SHORTID",
			},
		},
		{Name: "n5", Protocol: "hysteria2", Server: "e.example.com", Port: 8443, Password: "pw", TLS: true},
	}
	var warned []string
	opts := ClashProducerOpts{
		OnWarning: func(n *ParsedNode, reason string) {
			warned = append(warned, n.Name+":"+reason)
		},
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: nodes}, opts)
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	s := string(out)
	// All 5 proxies must be present — modern Clash forks (mihomo / Verge / Stash)
	// support reality, so n4 is no longer filtered.
	for _, n := range []string{"n1", "n2", "n3", "n4", "n5"} {
		if !strings.Contains(s, "name: "+n) {
			t.Errorf("missing %s in output:\n%s", n, s)
		}
	}
	for _, field := range []string{"reality-opts", "public-key", "short-id", "flow", "client-fingerprint", "servername"} {
		if !strings.Contains(s, field) {
			t.Errorf("reality node missing %q in output:\n%s", field, s)
		}
	}
	if len(warned) != 0 {
		t.Errorf("did not expect warnings, got %v", warned)
	}
	// Validate field ordering: name should appear before type before server
	// before port within each proxy block.
	firstName := strings.Index(s, "name: n1")
	firstType := strings.Index(s, "type: vmess")
	firstServer := strings.Index(s, "server: a.example.com")
	firstPort := strings.Index(s, "port: 443")
	if firstName < 0 || firstType < 0 || firstServer < 0 || firstPort < 0 {
		t.Fatalf("missing canonical fields in output:\n%s", s)
	}
	if !(firstName < firstType && firstType < firstServer && firstServer < firstPort) {
		t.Errorf("field order wrong: name=%d type=%d server=%d port=%d", firstName, firstType, firstServer, firstPort)
	}
}

func TestProduceClashYAML_EmptyNodes(t *testing.T) {
	out, err := ProduceClashYAML(nil, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	if !strings.Contains(string(out), "proxies:") {
		t.Errorf("output should still contain proxies: key, got:\n%s", out)
	}
}

func TestProduceClashYAML_NilNodeSkipped(t *testing.T) {
	nodes := []*ParsedNode{
		nil,
		{Name: "ok", Protocol: "ss", Server: "x", Port: 80, Method: "aes-256-gcm", Password: "pw"},
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: nodes}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	if !strings.Contains(string(out), "name: ok") {
		t.Errorf("expected ok in output")
	}
}

func TestProduceClashYAML_RawPreserved(t *testing.T) {
	nodes := []*ParsedNode{
		{
			Name:     "raw-test",
			Protocol: "vmess",
			Server:   "x.example.com",
			Port:     443,
			UUID:     "uuid",
			Network:  "tcp",
			Raw: map[string]interface{}{
				"aid":                "0",
				"client-fingerprint": "chrome",
			},
		},
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: nodes}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "_raw:") {
		t.Errorf("_raw section missing")
	}
	if !strings.Contains(s, "client-fingerprint: chrome") {
		t.Errorf("raw value not preserved:\n%s", s)
	}
}

// TestProduceClashYAML_DefaultSectionsEmittedWhenEmpty: a render with only
// nodes (no user-configured groups / rules / rule-sets) must still produce a
// usable Clash document with the four canonical sections (proxies +
// proxy-groups + rules; rule-providers is omitted on purpose when there are
// no records since mihomo dislikes an empty mapping).
func TestProduceClashYAML_DefaultSectionsEmittedWhenEmpty(t *testing.T) {
	nodes := []*ParsedNode{
		{
			Name: "a", Protocol: "ss", Server: "1.1.1.1", Port: 80,
			Method: "aes-256-gcm", Password: "pw",
		},
		{
			Name: "b", Protocol: "trojan", Server: "2.2.2.2", Port: 443,
			Password: "pw", TLS: true,
		},
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: nodes}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	s := string(out)
	for _, want := range []string{"proxies:", "proxy-groups:", "rules:"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected fallback marker %q in output:\n%s", want, s)
		}
	}
	if strings.Contains(s, "rule-providers:") {
		t.Errorf("rule-providers must not appear when no rule sets configured:\n%s", s)
	}
	// rules section: assert via decoded YAML to avoid emoji-escape pitfalls.
	rules := decodeRulesList(t, out)
	if len(rules) != 1 || rules[0] != "MATCH,🚀 节点选择" {
		t.Errorf("expected single MATCH fallback rule, got %v", rules)
	}
	// proxy-groups section: at least the default 🚀 节点选择 group must exist.
	var doc struct {
		Groups []map[string]any `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("decode groups: %v", err)
	}
	if len(doc.Groups) != 1 || doc.Groups[0]["name"] != "🚀 节点选择" {
		t.Errorf("expected one default group named 🚀 节点选择, got %v", doc.Groups)
	}
}

// TestProduceClashYAML_FullInputEmitsAllFourSections covers the happy path:
// a render with nodes + custom_rules + proxy-groups + rule-sets must emit
// every section the client expects to import.
func TestProduceClashYAML_FullInputEmitsAllFourSections(t *testing.T) {
	input := &ClashRenderInput{
		Nodes: []*ParsedNode{
			{
				Name: "node-a", Protocol: "ss", Server: "1.1.1.1", Port: 80,
				Method: "aes-256-gcm", Password: "pw",
			},
		},
		ProxyGroups: []ProxyGroupRecord{
			{
				ID: "g1", Name: "🚀 节点选择", Type: "select",
				SortOrder: 1, MemberProxies: `["DIRECT","♻️ 自动选择","node-a"]`,
				MemberGroups: `["♻️ 自动选择"]`,
			},
			{
				ID: "g2", Name: "♻️ 自动选择", Type: "url-test",
				SortOrder: 2, MemberProxies: `["node-a"]`,
				TestURL: "http://www.gstatic.com/generate_204", TestInterval: 300,
				IncludeAll: true, Filter: ".*",
			},
		},
		RuleSets: []RuleSetRecord{
			{
				Name: "cn-domain", Behavior: "domain", Format: "mrs",
				URL: "https://gh-proxy.com/example/cn.mrs", IntervalSeconds: 86400,
			},
		},
		CustomRules: []CustomRuleRecord{
			{
				Name: "user-direct", Type: "rules", Mode: "prepend", Sort: 1,
				Content: "DOMAIN-SUFFIX,openai.com,🚀 节点选择\nRULE-SET,cn-domain,DIRECT",
			},
		},
	}
	out, err := ProduceClashYAML(input, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"proxies:",
		"proxy-groups:",
		"rule-providers:",
		"rules:",
		"name: node-a",
		"type: url-test",
		"cn-domain:",
		"behavior: domain",
		"format: mrs",
		"include-all: true",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in output:\n%s", want, s)
		}
	}
	// rules section: decode YAML so emoji-escaping does not break substring
	// assertions. Order: user prepend lines → seeded MATCH tail.
	rules := decodeRulesList(t, out)
	idxOpenAI := -1
	idxRuleSet := -1
	idxMatch := -1
	for i, r := range rules {
		switch r {
		case "DOMAIN-SUFFIX,openai.com,🚀 节点选择":
			idxOpenAI = i
		case "RULE-SET,cn-domain,DIRECT":
			idxRuleSet = i
		case "MATCH,🚀 节点选择":
			idxMatch = i
		}
	}
	if idxOpenAI < 0 || idxRuleSet < 0 || idxMatch < 0 {
		t.Fatalf("expected user prepend + match tail in rules: %v", rules)
	}
	if !(idxOpenAI < idxMatch && idxRuleSet < idxMatch) {
		t.Errorf("expected user lines before MATCH tail, got rules=%v", rules)
	}
}

// TestProduceClashYAML_ReplaceModeWipesDefaults verifies that a single
// custom_rule with mode=replace fully overrides both the seeded MATCH tail
// and any earlier prepend/append entries that came before it in sort order.
func TestProduceClashYAML_ReplaceModeWipesDefaults(t *testing.T) {
	input := &ClashRenderInput{
		Nodes: []*ParsedNode{
			{
				Name: "x", Protocol: "ss", Server: "1.1.1.1", Port: 80,
				Method: "aes-256-gcm", Password: "pw",
			},
		},
		CustomRules: []CustomRuleRecord{
			// prepend first (Sort=1): will be wiped by the replace at Sort=2.
			{
				Name: "p1", Type: "rules", Mode: "prepend", Sort: 1,
				Content: "DOMAIN-SUFFIX,prepended.example,DIRECT",
			},
			{
				Name: "r1", Type: "rules", Mode: "replace", Sort: 2,
				Content: "DOMAIN-SUFFIX,replace.example,🚀 节点选择\nMATCH,DIRECT",
			},
		},
	}
	out, err := ProduceClashYAML(input, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	rules := decodeRulesList(t, out)
	got := map[string]bool{}
	for _, r := range rules {
		got[r] = true
	}
	if got["DOMAIN-SUFFIX,prepended.example,DIRECT"] {
		t.Errorf("replace mode should have wiped the earlier prepend line, got %v", rules)
	}
	if !got["DOMAIN-SUFFIX,replace.example,🚀 节点选择"] {
		t.Errorf("replace content missing, got %v", rules)
	}
	if !got["MATCH,DIRECT"] {
		t.Errorf("replace MATCH tail missing, got %v", rules)
	}
	if got["MATCH,🚀 节点选择"] {
		t.Errorf("replace should have purged the default MATCH, got %v", rules)
	}
}

// TestProduceClashYAML_MatchAlwaysLast verifies MATCH is always the final
// rule. Clash/mihomo requires MATCH as the catch-all tail.
func TestProduceClashYAML_MatchAlwaysLast(t *testing.T) {
	input := &ClashRenderInput{
		CustomRules: []CustomRuleRecord{
			{
				Name: "tail", Type: "rules", Mode: "append", Sort: 1,
				Content: "DOMAIN-SUFFIX,tail.example,DIRECT",
			},
		},
	}
	out, err := ProduceClashYAML(input, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	rules := decodeRulesList(t, out)
	if len(rules) < 2 {
		t.Fatalf("expected at least 2 rules, got %d", len(rules))
	}
	last := rules[len(rules)-1]
	if last != "MATCH,🚀 节点选择" {
		t.Errorf("MATCH should be last rule, got %q; rules: %v", last, rules)
	}
	found := false
	for _, r := range rules {
		if r == "DOMAIN-SUFFIX,tail.example,DIRECT" {
			found = true
		}
	}
	if !found {
		t.Errorf("appended rule missing: %v", rules)
	}
}

// TestProduceClashYAML_ProxiesOnlyOpt suppresses the auto-seeded sections.
// Used by the rule-editor preview to feed the rule injector a clean base.
func TestProduceClashYAML_ProxiesOnlyOpt(t *testing.T) {
	input := &ClashRenderInput{
		Nodes: []*ParsedNode{
			{
				Name: "x", Protocol: "ss", Server: "1.1.1.1", Port: 80,
				Method: "aes-256-gcm", Password: "pw",
			},
		},
	}
	out, err := ProduceClashYAML(input, ClashProducerOpts{ProxiesOnly: true})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "proxies:") {
		t.Errorf("expected proxies block, got:\n%s", s)
	}
	for _, banned := range []string{"proxy-groups:", "rule-providers:", "rules:"} {
		if strings.Contains(s, banned) {
			t.Errorf("ProxiesOnly should suppress %q, got:\n%s", banned, s)
		}
	}
}

// TestProduceClashYAML_SkipCertVerifyPromoted: when the parsed node carries a
// "skip cert verify" intent in Raw (Clash key skip-cert-verify, or URI keys
// allowInsecure / insecure), the producer must emit the real Clash field so
// clients with a self-signed / expired upstream cert can still connect.
func TestProduceClashYAML_SkipCertVerifyPromoted(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]interface{}
	}{
		{"clash-bool", map[string]interface{}{"skip-cert-verify": true}},
		{"uri-allowInsecure", map[string]interface{}{"allowInsecure": "1"}},
		{"uri-insecure-true", map[string]interface{}{"insecure": "true"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nodes := []*ParsedNode{{
				Name: "n", Protocol: "trojan", Server: "x.example.com",
				Port: 443, Password: "p", TLS: true, SNI: "x.example.com",
				Raw: c.raw,
			}}
			out, err := ProduceClashYAML(&ClashRenderInput{Nodes: nodes}, ClashProducerOpts{})
			if err != nil {
				t.Fatalf("ProduceClashYAML: %v", err)
			}
			if !strings.Contains(string(out), "skip-cert-verify: true") {
				t.Errorf("skip-cert-verify not emitted for %s:\n%s", c.name, out)
			}
		})
	}
}

// A node WITHOUT any insecure intent must NOT get skip-cert-verify.
func TestProduceClashYAML_NoSkipCertVerifyByDefault(t *testing.T) {
	nodes := []*ParsedNode{{
		Name: "n", Protocol: "trojan", Server: "x", Port: 443,
		Password: "p", TLS: true,
	}}
	out, _ := ProduceClashYAML(&ClashRenderInput{Nodes: nodes}, ClashProducerOpts{})
	if strings.Contains(string(out), "skip-cert-verify") {
		t.Errorf("skip-cert-verify must not appear by default:\n%s", out)
	}
}
