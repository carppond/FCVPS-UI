package substore

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// baseClashYAML is a minimal Clash config with dns / rules / rule-providers
// pre-populated so we can exercise both "replace" and "merge" branches.
const baseClashYAML = `proxies: []
dns:
  enable: true
  nameservers:
    - 1.1.1.1
    - 8.8.8.8
  fallback:
    - tls://9.9.9.9
rules:
  - DOMAIN-SUFFIX,cn,DIRECT
  - MATCH,Proxy
rule-providers:
  reject:
    type: http
    behavior: domain
    url: https://example.com/reject.yaml
    path: ./reject.yaml
    interval: 86400
`

// renderToString applies rules to base and returns the rendered YAML string
// for substring assertions.
func renderToString(t *testing.T, base string, rules []*CustomRule) string {
	t.Helper()
	out, err := ApplyToYAML([]byte(base), rules)
	if err != nil {
		t.Fatalf("ApplyToYAML: %v", err)
	}
	return string(out)
}

// parseField re-decodes the rendered YAML and returns the requested field as
// a Go interface for structural assertions.
func parseField(t *testing.T, yamlStr, key string) any {
	t.Helper()
	var root map[string]any
	if err := yaml.Unmarshal([]byte(yamlStr), &root); err != nil {
		t.Fatalf("decode rendered yaml: %v\n---\n%s", err, yamlStr)
	}
	return root[key]
}

// ─── dns × replace / prepend / append ────────────────────────────────────────

func TestInject_DNS_Replace(t *testing.T) {
	rule := &CustomRule{
		Name: "dns-replace", Type: "dns", Mode: "replace",
		Content: "enable: false\nnameservers:\n  - 114.114.114.114\n",
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	dns, ok := parseField(t, out, "dns").(map[string]any)
	if !ok {
		t.Fatalf("dns is not a map: %T", parseField(t, out, "dns"))
	}
	if v, _ := dns["enable"].(bool); v {
		t.Fatalf("dns.enable: want false, got true")
	}
	ns, _ := dns["nameservers"].([]any)
	if len(ns) != 1 || ns[0] != "114.114.114.114" {
		t.Fatalf("nameservers replaced incorrectly: %v", ns)
	}
	if _, exists := dns["fallback"]; exists {
		t.Fatalf("fallback should be wiped by replace, still present")
	}
}

func TestInject_DNS_Prepend(t *testing.T) {
	rule := &CustomRule{
		Name: "dns-prepend", Type: "dns", Mode: "prepend",
		Content: "nameservers:\n  - 223.5.5.5\n",
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	dns, _ := parseField(t, out, "dns").(map[string]any)
	ns, _ := dns["nameservers"].([]any)
	if len(ns) != 3 {
		t.Fatalf("want 3 nameservers, got %d (%v)", len(ns), ns)
	}
	if ns[0] != "223.5.5.5" {
		t.Fatalf("prepend should put incoming first; got %v", ns)
	}
	// existing values preserved.
	if ns[1] != "1.1.1.1" || ns[2] != "8.8.8.8" {
		t.Fatalf("tail nameservers altered: %v", ns)
	}
	// fallback (untouched key) survives merge.
	fb, _ := dns["fallback"].([]any)
	if len(fb) != 1 {
		t.Fatalf("fallback should be untouched, got %v", fb)
	}
}

func TestInject_DNS_Append(t *testing.T) {
	rule := &CustomRule{
		Name: "dns-append", Type: "dns", Mode: "append",
		Content: "fallback:\n  - tls://1.0.0.1\n",
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	dns, _ := parseField(t, out, "dns").(map[string]any)
	fb, _ := dns["fallback"].([]any)
	if len(fb) != 2 {
		t.Fatalf("want 2 fallback entries, got %d (%v)", len(fb), fb)
	}
	if fb[0] != "tls://9.9.9.9" || fb[1] != "tls://1.0.0.1" {
		t.Fatalf("append should put incoming last; got %v", fb)
	}
}

// ─── rules × replace / prepend / append ──────────────────────────────────────

func TestInject_Rules_Replace(t *testing.T) {
	rule := &CustomRule{
		Name: "rules-replace", Type: "rules", Mode: "replace",
		Content: "DOMAIN-SUFFIX,example.com,DIRECT\nMATCH,Auto\n",
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	rs, _ := parseField(t, out, "rules").([]any)
	if len(rs) != 2 {
		t.Fatalf("want 2 rules, got %d (%v)", len(rs), rs)
	}
	if rs[0] != "DOMAIN-SUFFIX,example.com,DIRECT" {
		t.Fatalf("rule[0]: %v", rs[0])
	}
	if rs[1] != "MATCH,Auto" {
		t.Fatalf("rule[1]: %v", rs[1])
	}
}

func TestInject_Rules_Prepend(t *testing.T) {
	rule := &CustomRule{
		Name: "rules-prepend", Type: "rules", Mode: "prepend",
		Content: "DOMAIN-KEYWORD,google,Proxy\nDOMAIN-SUFFIX,localhost,DIRECT\n",
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	rs, _ := parseField(t, out, "rules").([]any)
	if len(rs) != 4 {
		t.Fatalf("want 4 rules, got %d (%v)", len(rs), rs)
	}
	if rs[0] != "DOMAIN-KEYWORD,google,Proxy" {
		t.Fatalf("first rule should be the prepended one; got %v", rs[0])
	}
	if rs[2] != "DOMAIN-SUFFIX,cn,DIRECT" || rs[3] != "MATCH,Proxy" {
		t.Fatalf("tail rules altered: %v", rs)
	}
}

func TestInject_Rules_Append(t *testing.T) {
	rule := &CustomRule{
		Name: "rules-append", Type: "rules", Mode: "append",
		Content: "GEOIP,CN,DIRECT\n",
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	rs, _ := parseField(t, out, "rules").([]any)
	if len(rs) != 3 {
		t.Fatalf("want 3 rules, got %d (%v)", len(rs), rs)
	}
	if rs[2] != "GEOIP,CN,DIRECT" {
		t.Fatalf("last rule should be the appended one; got %v", rs[2])
	}
}

// ─── rule-providers × replace / prepend / append ─────────────────────────────

func TestInject_RuleProviders_Replace(t *testing.T) {
	rule := &CustomRule{
		Name: "rp-replace", Type: "rule-providers", Mode: "replace",
		Content: `ads:
  type: http
  behavior: domain
  url: https://example.com/ads.yaml
  path: ./ads.yaml
  interval: 86400
`,
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	rp, _ := parseField(t, out, "rule-providers").(map[string]any)
	if len(rp) != 1 {
		t.Fatalf("want 1 provider after replace, got %d (%v)", len(rp), rp)
	}
	if _, ok := rp["ads"]; !ok {
		t.Fatalf("expected 'ads' provider; got %v", rp)
	}
	if _, ok := rp["reject"]; ok {
		t.Fatalf("replace should wipe pre-existing providers")
	}
}

func TestInject_RuleProviders_Prepend(t *testing.T) {
	rule := &CustomRule{
		Name: "rp-prepend", Type: "rule-providers", Mode: "prepend",
		Content: `reject:
  type: http
  behavior: classical
  url: https://other.example.com/reject.yaml
  path: ./reject2.yaml
  interval: 3600
malware:
  type: http
  behavior: domain
  url: https://example.com/mw.yaml
  path: ./mw.yaml
  interval: 86400
`,
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	rp, _ := parseField(t, out, "rule-providers").(map[string]any)
	if len(rp) != 2 {
		t.Fatalf("want 2 providers (reject + malware), got %d (%v)", len(rp), rp)
	}
	// prepend → incoming overrides on key collision.
	reject, _ := rp["reject"].(map[string]any)
	if reject["behavior"] != "classical" {
		t.Fatalf("prepend should override existing key; got %v", reject)
	}
	if _, ok := rp["malware"]; !ok {
		t.Fatalf("new key 'malware' missing")
	}
}

func TestInject_RuleProviders_Append(t *testing.T) {
	rule := &CustomRule{
		Name: "rp-append", Type: "rule-providers", Mode: "append",
		Content: `reject:
  type: http
  behavior: classical
  url: https://other.example.com/reject.yaml
  path: ./reject2.yaml
  interval: 3600
malware:
  type: http
  behavior: domain
  url: https://example.com/mw.yaml
  path: ./mw.yaml
  interval: 86400
`,
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	rp, _ := parseField(t, out, "rule-providers").(map[string]any)
	if len(rp) != 2 {
		t.Fatalf("want 2 providers, got %d (%v)", len(rp), rp)
	}
	// append → existing wins on key collision.
	reject, _ := rp["reject"].(map[string]any)
	if reject["behavior"] != "domain" {
		t.Fatalf("append should NOT override existing key; got %v", reject)
	}
	if _, ok := rp["malware"]; !ok {
		t.Fatalf("new key 'malware' missing")
	}
}

// ─── Edge cases ──────────────────────────────────────────────────────────────

func TestInject_SortOrderApplied(t *testing.T) {
	// Two rules: first appends "A,DIRECT" then second appends "B,DIRECT".
	// Sort decides the order — engine should already pre-sort, but the
	// injector must not re-sort.
	rules := []*CustomRule{
		{Name: "r1", Type: "rules", Mode: "append", Content: "A,DIRECT\n", Sort: 100},
		{Name: "r2", Type: "rules", Mode: "append", Content: "B,DIRECT\n", Sort: 200},
	}
	out := renderToString(t, baseClashYAML, rules)
	rs, _ := parseField(t, out, "rules").([]any)
	// Expected: original 2 + A + B.
	if len(rs) != 4 {
		t.Fatalf("rule count: %d (%v)", len(rs), rs)
	}
	if rs[2] != "A,DIRECT" || rs[3] != "B,DIRECT" {
		t.Fatalf("sort not preserved: %v", rs)
	}
}

func TestInject_NilBase(t *testing.T) {
	_, err := Inject(nil, nil)
	if err == nil {
		t.Fatalf("expected error on nil base")
	}
}

func TestInject_NoRules(t *testing.T) {
	out := renderToString(t, baseClashYAML, nil)
	if !strings.Contains(out, "1.1.1.1") {
		t.Fatalf("base content lost when no rules applied: %s", out)
	}
}

func TestInject_UnknownType(t *testing.T) {
	rule := &CustomRule{Name: "bad", Type: "garbage", Mode: "replace", Content: "x: y\n"}
	_, err := ApplyToYAML([]byte(baseClashYAML), []*CustomRule{rule})
	if err == nil {
		t.Fatalf("expected error on unknown type")
	}
}

func TestInject_UnknownMode(t *testing.T) {
	rule := &CustomRule{Name: "bad", Type: "rules", Mode: "garbage", Content: "X,DIRECT\n"}
	_, err := ApplyToYAML([]byte(baseClashYAML), []*CustomRule{rule})
	if err == nil {
		t.Fatalf("expected error on unknown mode")
	}
}

func TestInject_AddsDNSWhenMissing(t *testing.T) {
	// Strip dns from base, then prepend → injector should treat as replace.
	stripped := "proxies: []\nrules: [MATCH,Proxy]\n"
	rule := &CustomRule{
		Name: "dns-from-zero", Type: "dns", Mode: "prepend",
		Content: "nameservers: [1.1.1.1]\n",
	}
	out := renderToString(t, stripped, []*CustomRule{rule})
	dns, ok := parseField(t, out, "dns").(map[string]any)
	if !ok {
		t.Fatalf("dns missing after inject: %s", out)
	}
	if ns, _ := dns["nameservers"].([]any); len(ns) != 1 {
		t.Fatalf("nameservers count: %v", ns)
	}
}

func TestInject_RulesContentSupportsListSyntax(t *testing.T) {
	// User pasted a YAML-style list — injector should strip "- " prefix.
	rule := &CustomRule{
		Name: "list-syntax", Type: "rules", Mode: "replace",
		Content: "- 'DOMAIN-SUFFIX,a,DIRECT'\n- DOMAIN-SUFFIX,b,Proxy\n",
	}
	out := renderToString(t, baseClashYAML, []*CustomRule{rule})
	rs, _ := parseField(t, out, "rules").([]any)
	if len(rs) != 2 {
		t.Fatalf("rule count: %d (%v)", len(rs), rs)
	}
	if rs[0] != "DOMAIN-SUFFIX,a,DIRECT" || rs[1] != "DOMAIN-SUFFIX,b,Proxy" {
		t.Fatalf("list-syntax stripping failed: %v", rs)
	}
}
