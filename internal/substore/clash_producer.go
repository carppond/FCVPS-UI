package substore

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"shiguang-vps/internal/util"
)

// ProduceClashYAML renders a slice of ParsedNode into a Clash-compatible
// YAML document. The output is mihomo / Clash Meta / Clash Verge Rev /
// Stash compatible (modern Clash forks); these all support reality, hysteria2,
// tuic, anytls etc. We do NOT filter reality vless nodes — the older policy
// of dropping reality was based on the now-defunct original Clash Premium
// and made real-world subscriptions (lots of reality nodes) come out empty.
//
// The output structure is:
//
//	proxies:
//	  - name: ...
//	    type: ...
//	    ...
//
// Field ordering within each proxy entry is canonical (name/type/server/port
// first) via util.ReorderProxyNode.
func ProduceClashYAML(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, error) {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	proxiesKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "proxies"}
	proxiesVal := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	root.Content = append(root.Content, proxiesKey, proxiesVal)

	for _, n := range nodes {
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
	}
	return util.MarshalIndent(root)
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
	if n.SNI != "" {
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
