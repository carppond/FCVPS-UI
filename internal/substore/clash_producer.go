package substore

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"shiguang-vps/internal/util"
)

// ProduceClashYAML renders a ClashRenderInput into a Clash-compatible YAML
// document. Output is mihomo / Clash Meta / Clash Verge Rev / Stash compatible
// (modern Clash forks); these all support reality, hysteria2, tuic, anytls,
// rule-set providers, etc.
//
// The output document always carries four top-level keys so a client that
// imports the file can immediately route traffic:
//
//	proxies:        [ ... ]    # node list (always emitted, even when empty)
//	proxy-groups:   [ ... ]    # at least one fallback "🚀 节点选择" group
//	rule-providers: { ... }    # one entry per enabled RuleSetRecord
//	rules:          [ ... ]    # custom_rules applied via rule_injector + MATCH
//
// Bug-fix note (T-fix Clash): previously the producer emitted only `proxies:`.
// Clients (Clash Verge, mihomo) would import the file but see no rules, no
// groups, no rule-sets — effectively the subscription rendered useless.
// ProduceClashYAML now assembles the full document so the same /download
// endpoint delivers a config the client can run as-is.
func ProduceClashYAML(input *ClashRenderInput, opts ClashProducerOpts) ([]byte, error) {
	if input == nil {
		input = &ClashRenderInput{}
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	// proxies: — always first so the rest of the doc can reference node names.
	proxiesVal := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	proxyNames := make([]string, 0, len(input.Nodes))
	for _, n := range input.Nodes {
		if n == nil {
			continue
		}
		entry, err := nodeToYAML(n)
		if err != nil {
			if opts.OnWarning != nil {
				opts.OnWarning(n, fmt.Sprintf("render skipped: %v", err))
			}
			continue
		}
		ordered := util.ReorderProxyNode(entry)
		proxiesVal.Content = append(proxiesVal.Content, ordered)
		proxyNames = append(proxyNames, n.Name)
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "proxies"},
		proxiesVal,
	)

	// Preview / legacy mode: emit just `proxies:` so a downstream caller
	// (rule editor preview) can layer rule_injector output on top of a clean
	// base. Skip both seeding and custom-rule injection here.
	if opts.ProxiesOnly {
		return util.MarshalIndent(root)
	}

	// proxy-groups: — render user config; fall back to a sane default that
	// references every proxy so the implicit MATCH rule below can resolve.
	groupsVal := buildProxyGroups(input.ProxyGroups, proxyNames)
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "proxy-groups"},
		groupsVal,
	)

	// rule-providers: — only emitted when the user has at least one enabled
	// rule-set. mihomo tolerates the key being absent but rejects an empty
	// mapping in some versions, so we skip it instead.
	if providers := buildRuleProviders(input.RuleSets); providers != nil {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "rule-providers"},
			providers,
		)
	}

	// rules: — start with a single MATCH tail so even an empty input still
	// produces a config the client can boot. CustomRules of type=rules are
	// then layered via rule_injector (which honours mode=replace/prepend/append).
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "rules"},
		defaultMatchRule(),
	)

	// Apply user-defined custom rules (rules / dns / rule-providers). The
	// injector mutates a deep copy and returns the new root.
	if rules := toInjectorRules(input.CustomRules); len(rules) > 0 {
		mutated, err := Inject(root, rules)
		if err != nil {
			return nil, fmt.Errorf("apply custom rules: %w", err)
		}
		root = mutated
	}

	// Auto-supplement missing proxy groups: scan rules for referenced group
	// names that don't exist in proxy-groups and create them as select-type
	// groups. Without this, mihomo rejects the config with "proxy not found".
	ensureMissingProxyGroups(root, proxyNames)

	return util.MarshalIndent(root)
}

// buildProxyGroups turns the user-configured proxy groups into the Clash
// `proxy-groups:` sequence. When the input is empty, a single default
// "🚀 节点选择" select group is emitted so any rule that targets a group has a
// definition to resolve against — without this, mihomo refuses to start with
// "proxy group not found".
func buildProxyGroups(records []ProxyGroupRecord, proxyNames []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	if len(records) == 0 {
		seq.Content = append(seq.Content, defaultSelectGroup(proxyNames))
		return seq
	}
	// Sort group-typed entries before plain-node-typed entries when the user
	// has not specified an explicit sort_order. The Reorder API on the repo
	// already returns sort_order ASC, so we just preserve insertion order
	// here.
	sorted := make([]ProxyGroupRecord, len(records))
	copy(sorted, records)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].SortOrder < sorted[j].SortOrder
	})
	for _, rec := range sorted {
		seq.Content = append(seq.Content, proxyGroupToYAML(rec, proxyNames))
	}
	return seq
}

// proxyGroupToYAML renders a single proxy group entry. Field ordering follows
// the convention used by community templates (name → type → proxies → flags).
func proxyGroupToYAML(rec ProxyGroupRecord, proxyNames []string) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	_ = util.SetMappingValue(m, "name", rec.Name)
	_ = util.SetMappingValue(m, "type", rec.Type)

	// Resolve members: group references go first (mihomo style) then node
	// references. Members are intentionally not deduplicated — the user may
	// want a deliberate repeat (e.g. "DIRECT" appearing twice for emphasis).
	members := make([]string, 0, 8)
	if grps := decodeJSONStringArray(rec.MemberGroups); len(grps) > 0 {
		members = append(members, grps...)
	}
	if ps := decodeJSONStringArray(rec.MemberProxies); len(ps) > 0 {
		members = append(members, ps...)
	}
	// Empty members would make Clash reject the file; fall back to DIRECT
	// rather than fail the whole render.
	if len(members) == 0 {
		members = []string{"DIRECT"}
		// When include_all is set we expand to the full node list later via
		// the include-all flag; no need to inline node names here.
		if !rec.IncludeAll && len(proxyNames) > 0 {
			members = append(members, proxyNames...)
		}
	}
	_ = util.SetMappingValue(m, "proxies", stringsToYAMLSeq(members))

	if rec.IncludeAll {
		_ = util.SetMappingValue(m, "include-all", true)
	}
	if rec.Filter != "" {
		_ = util.SetMappingValue(m, "filter", rec.Filter)
	}

	// url-test / fallback / load-balance care about test_url + interval.
	switch rec.Type {
	case "url-test", "fallback", "load-balance":
		url := rec.TestURL
		if url == "" {
			url = "http://www.gstatic.com/generate_204"
		}
		_ = util.SetMappingValue(m, "url", url)
		interval := rec.TestInterval
		if interval <= 0 {
			interval = 300
		}
		_ = util.SetMappingValue(m, "interval", int(interval))
	}

	if rec.Icon != "" {
		_ = util.SetMappingValue(m, "icon", rec.Icon)
	}
	return m
}

// defaultSelectGroup builds the fallback "🚀 节点选择" group used when the user
// has not configured any proxy group of their own. proxies includes DIRECT,
// REJECT and every parsed node name so a freshly-installed user can route
// traffic immediately.
func defaultSelectGroup(proxyNames []string) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	_ = util.SetMappingValue(m, "name", "🚀 节点选择")
	_ = util.SetMappingValue(m, "type", "select")
	members := make([]string, 0, len(proxyNames)+2)
	members = append(members, "DIRECT", "REJECT")
	members = append(members, proxyNames...)
	_ = util.SetMappingValue(m, "proxies", stringsToYAMLSeq(members))
	return m
}

// ensureMissingProxyGroups scans the final rules list for proxy group names
// that don't exist in proxy-groups and auto-creates them as select-type groups.
// This prevents mihomo from rejecting configs where a rule template references
// a group the user hasn't manually created (e.g. "🎮 游戏").
func ensureMissingProxyGroups(root *yaml.Node, proxyNames []string) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	var rulesNode, groupsNode *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind == yaml.ScalarNode {
			switch key.Value {
			case "rules":
				rulesNode = root.Content[i+1]
			case "proxy-groups":
				groupsNode = root.Content[i+1]
			}
		}
	}
	if rulesNode == nil || groupsNode == nil {
		return
	}

	// Collect existing group names.
	existing := make(map[string]bool)
	for _, g := range groupsNode.Content {
		if g.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(g.Content); j += 2 {
			if g.Content[j].Value == "name" {
				existing[g.Content[j+1].Value] = true
				break
			}
		}
	}
	// Built-in targets that are not proxy groups.
	existing["DIRECT"] = true
	existing["REJECT"] = true

	// Scan rules for referenced group names.
	missing := make(map[string]bool)
	for _, r := range rulesNode.Content {
		if r.Kind != yaml.ScalarNode {
			continue
		}
		parts := strings.SplitN(r.Value, ",", 3)
		var target string
		switch {
		case len(parts) == 2:
			// 2-part rules like "MATCH,🐟 漏网之鱼"
			target = strings.TrimSpace(parts[1])
		case len(parts) >= 3:
			target = strings.TrimSpace(parts[2])
			// Strip ",no-resolve" suffix (e.g. "🇭🇰 香港节点,no-resolve" → "🇭🇰 香港节点")
			if idx := strings.LastIndex(target, ","); idx >= 0 {
				suffix := strings.TrimSpace(target[idx+1:])
				if strings.EqualFold(suffix, "no-resolve") {
					target = strings.TrimSpace(target[:idx])
				}
			}
		default:
			continue
		}
		if target != "" && !existing[target] {
			missing[target] = true
		}
	}

	// Create missing groups as select type.
	for name := range missing {
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		_ = util.SetMappingValue(m, "name", name)
		_ = util.SetMappingValue(m, "type", "select")
		members := make([]string, 0, len(proxyNames)+2)
		members = append(members, "DIRECT", "REJECT")
		members = append(members, proxyNames...)
		_ = util.SetMappingValue(m, "proxies", stringsToYAMLSeq(members))
		groupsNode.Content = append(groupsNode.Content, m)
		existing[name] = true
	}
}

// buildRuleProviders maps RuleSetRecord -> Clash `rule-providers:` mapping.
// Returns nil when there are zero records so the caller can skip emitting the
// key altogether.
func buildRuleProviders(records []RuleSetRecord) *yaml.Node {
	if len(records) == 0 {
		return nil
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	// Sort by name for deterministic output (storage already filters to
	// enabled=true and returns created_at ASC, but a tied set is more
	// predictable when sorted alphabetically).
	sorted := make([]RuleSetRecord, len(records))
	copy(sorted, records)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, rec := range sorted {
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		_ = util.SetMappingValue(entry, "type", "http")
		_ = util.SetMappingValue(entry, "url", rec.URL)
		_ = util.SetMappingValue(entry, "behavior", rec.Behavior)
		format := rec.Format
		if format == "" {
			format = "yaml"
		}
		_ = util.SetMappingValue(entry, "format", format)
		interval := rec.IntervalSeconds
		if interval <= 0 {
			interval = 86400
		}
		_ = util.SetMappingValue(entry, "interval", int(interval))
		// `path` is required by mihomo; derive it deterministically from the
		// rule-set name so repeated downloads point at the same cache file.
		path := "./rule-sets/" + safeProviderPath(rec.Name) + "." + format
		_ = util.SetMappingValue(entry, "path", path)
		_ = util.SetMappingValue(m, rec.Name, entry)
	}
	return m
}

// safeProviderPath sanitises a rule-set name for use as a file path segment.
// Whitespace and slash characters are collapsed to "-" so an exotic name like
// "🇨🇳 国内 / mainland" still yields a sensible path.
func safeProviderPath(name string) string {
	if name == "" {
		return "rules"
	}
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"\t", "-",
	)
	return replacer.Replace(name)
}

// defaultMatchRule returns the single-entry rules sequence "MATCH,🚀 节点选择"
// which acts as a catch-all when the user has no custom rules of their own.
// The rule_injector layers any user rules on top of this seed.
func defaultMatchRule() *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	seq.Content = append(seq.Content, &yaml.Node{
		Kind: yaml.ScalarNode, Tag: "!!str", Value: "MATCH,🚀 节点选择",
	})
	return seq
}

// toInjectorRules converts CustomRuleRecord (storage projection) into the
// *CustomRule slice rule_injector consumes. Records are already filtered to
// enabled=true by the caller (storage.CustomRuleRepo.ListEnabled).
func toInjectorRules(records []CustomRuleRecord) []*CustomRule {
	if len(records) == 0 {
		return nil
	}
	// Stable sort by Sort ASC so prepend/append semantics are deterministic
	// even when the caller forgot to order them.
	sorted := make([]CustomRuleRecord, len(records))
	copy(sorted, records)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Sort < sorted[j].Sort })
	out := make([]*CustomRule, 0, len(sorted))
	for _, rec := range sorted {
		out = append(out, &CustomRule{
			ID: rec.ID, Name: rec.Name, Type: rec.Type, Mode: rec.Mode,
			Content: rec.Content, Sort: rec.Sort,
		})
	}
	return out
}

// decodeJSONStringArray decodes a JSON array string into []string. Empty
// input or a parse failure both yield a nil slice — at render time we treat
// "no members" identically to "no JSON". Logging this failure is not the
// producer's job (the repo / handler already validate JSON on write).
func decodeJSONStringArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	// Filter empty entries to avoid `- ""` survivors in the rendered YAML.
	cleaned := out[:0]
	for _, v := range out {
		if strings.TrimSpace(v) != "" {
			cleaned = append(cleaned, v)
		}
	}
	return cleaned
}

// stringsToYAMLSeq builds a block-style sequence of scalar strings.
func stringsToYAMLSeq(items []string) *yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, s := range items {
		seq.Content = append(seq.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: s,
		})
	}
	return seq
}

// nodeToYAML converts a ParsedNode into a yaml.Node mapping suitable for the
// `proxies:` sequence in a Clash config.
func nodeToYAML(n *ParsedNode) (*yaml.Node, error) {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if err := setStr(m, "name", n.Name); err != nil {
		return nil, err
	}
	if err := setStr(m, "type", n.Protocol); err != nil {
		return nil, err
	}
	if err := setStr(m, "server", n.Server); err != nil {
		return nil, err
	}
	if err := util.SetMappingValue(m, "port", n.Port); err != nil {
		return nil, err
	}
	// Protocol-specific fields. We dispatch on Protocol so the field set
	// mirrors what Clash core expects per type.
	switch n.Protocol {
	case "vmess":
		_ = setStr(m, "uuid", n.UUID)
		_ = setStr(m, "cipher", n.Method)
		_ = setStr(m, "network", n.Network)
		_ = setBool(m, "tls", n.TLS)
	case "vless":
		_ = setStr(m, "uuid", n.UUID)
		_ = setStr(m, "network", n.Network)
		_ = setBool(m, "tls", n.TLS)
		// vless extras commonly populated by reality / xtls subscriptions.
		// Source these from Raw because the parser keeps them there verbatim.
		if flow, ok := stringFromRaw(n.Raw, "flow"); ok {
			_ = setStr(m, "flow", flow)
		}
		if fp, ok := stringFromRaw(n.Raw, "fp", "client-fingerprint"); ok {
			_ = setStr(m, "client-fingerprint", fp)
		}
		if n.SNI != "" {
			_ = setStr(m, "servername", n.SNI)
		}
		// reality-opts: { public-key, short-id, [spider-x] }
		if n.Reality {
			realityOpts := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			if pbk, ok := stringFromRaw(n.Raw, "pbk", "public-key"); ok {
				_ = util.SetMappingValue(realityOpts, "public-key", pbk)
			}
			if sid, ok := stringFromRaw(n.Raw, "sid", "short-id"); ok {
				_ = util.SetMappingValue(realityOpts, "short-id", sid)
			}
			if spx, ok := stringFromRaw(n.Raw, "spx", "spider-x"); ok {
				_ = util.SetMappingValue(realityOpts, "spider-x", spx)
			}
			if len(realityOpts.Content) > 0 {
				m.Content = append(m.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "reality-opts"},
					realityOpts,
				)
			}
		}
	case "ss", "ssr":
		_ = setStr(m, "cipher", n.Method)
		_ = setStr(m, "password", n.Password)
	case "trojan", "hysteria", "hysteria2", "anytls":
		_ = setStr(m, "password", n.Password)
		_ = setBool(m, "tls", n.TLS)
	case "tuic":
		_ = setStr(m, "uuid", n.UUID)
		_ = setStr(m, "password", n.Password)
	case "wireguard":
		_ = setStr(m, "password", n.Password)
	case "socks5":
		if n.UUID != "" {
			_ = setStr(m, "username", n.UUID)
		}
		_ = setStr(m, "password", n.Password)
	case "naive":
		_ = setStr(m, "username", n.UUID)
		_ = setStr(m, "password", n.Password)
	}
	// vless already emits SNI as "servername" in its protocol-specific block;
	// skip the shared "sni" key to avoid duplicating the value in the output.
	if n.SNI != "" && n.Protocol != "vless" {
		_ = setStr(m, "sni", n.SNI)
	}
	if len(n.ALPN) > 0 {
		_ = util.SetMappingValue(m, "alpn", strSeq(n.ALPN))
	}
	if n.Path != "" {
		_ = setStr(m, "ws-path", n.Path)
	}
	if n.Host != "" {
		_ = setStr(m, "ws-headers", n.Host)
	}
	// Preserve unsupported fields verbatim under _raw so the parser is
	// lossless (PRD M-SUB.3).
	if len(n.Raw) > 0 {
		_ = util.SetMappingValue(m, "_raw", rawToYAML(n.Raw))
	}
	return m, nil
}

func setStr(m *yaml.Node, key, value string) error {
	if value == "" {
		return nil
	}
	return util.SetMappingValue(m, key, value)
}

// stringFromRaw looks up the first present, non-empty string value among the
// given alternate keys in the raw bag. Returns (value, true) on hit; ("", false)
// when no key resolves to a non-empty string. Used to source vless extras
// (flow / fp / pbk / sid / spx) that the parser keeps under Raw verbatim.
func stringFromRaw(raw map[string]interface{}, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := raw[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, true
			}
		}
	}
	return "", false
}

func setBool(m *yaml.Node, key string, value bool) error {
	return util.SetMappingValue(m, key, value)
}

// strSeq builds an inline yaml sequence node from a slice of strings.
func strSeq(items []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, s := range items {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s})
	}
	return n
}

// rawToYAML converts the Raw map into a yaml mapping node, ordering keys
// lexicographically to keep output deterministic.
func rawToYAML(raw map[string]interface{}) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	keys := sortedKeys(raw)
	for _, k := range keys {
		_ = util.SetMappingValue(m, k, valueToYAML(raw[k]))
	}
	return m
}

func valueToYAML(v interface{}) *yaml.Node {
	switch t := v.(type) {
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}
	case []string:
		return strSeq(t)
	case []interface{}:
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, e := range t {
			seq.Content = append(seq.Content, valueToYAML(e))
		}
		return seq
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("%v", t)}
	}
}

func sortedKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Tiny no-import sort to avoid pulling "sort" just for keys.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
