// Package substore implements multi-protocol subscription URI parsing,
// ACL4SSR (subconverter ini) compatibility and a Clash YAML producer.
//
// The package is the protocol layer for the M-SUB module (T-9). It is kept
// dependency-free from HTTP / DB layers; callers (T-8 sync_service) wrap the
// parsed nodes into the canonical types.Node DTO defined in internal/types.
package substore

// ParsedNode is the internal protocol-agnostic representation of a node
// produced by one of the URI parsers. Caller code is expected to translate
// these into types.Node before persistence (parsed_config_json carries the
// structured fields). Unknown / parser-unsupported fields are preserved in
// Raw so the round-trip is lossless.
type ParsedNode struct {
	Name     string                 // display name (#fragment or remark)
	Protocol string                 // vmess / vless / ss / ...
	Server   string                 // hostname or IP
	Port     int                    // TCP / UDP port
	UUID     string                 // vmess / vless / tuic
	Password string                 // ss / trojan / tuic / etc.
	Method   string                 // ss / ssr cipher
	Network  string                 // ws / tcp / grpc / h2 / quic
	TLS      bool                   // whether TLS is enabled
	SNI      string                 // tls SNI
	ALPN     []string               // tls alpn list
	Path     string                 // ws / h2 path
	Host     string                 // ws / h2 host header
	Reality  bool                   // vless reality marker (used by producer to filter)
	Raw      map[string]interface{} // unsupported fields retained verbatim
	Tag      string                 // raw fragment text (URL #frag)
}

// ACLConfig is the parsed representation of an ACL4SSR / subconverter INI
// file. Only the sections relevant to Clash producer routing are surfaced.
type ACLConfig struct {
	General  map[string]string
	Proxy    []ParsedNode
	Groups   []ACLProxyGroup
	Rules    []ACLRule
	Override map[string]string
}

// ACLProxyGroup represents a [Proxy Group] entry in an ACL4SSR / subconverter
// INI file. Type is one of select / url-test / fallback / load-balance / relay.
type ACLProxyGroup struct {
	Name      string
	Type      string
	Members   []string
	URL       string
	Interval  int
	Tolerance int
}

// ACLRule represents a single [Rule] entry. Type is the Clash rule kind such
// as DOMAIN / DOMAIN-SUFFIX / IP-CIDR. NoResolve is true when the rule trails
// with the `no-resolve` flag.
type ACLRule struct {
	Type      string
	Value     string
	Target    string
	NoResolve bool
}

// ClashProducerOpts controls behaviour of ProduceClashYAML.
type ClashProducerOpts struct {
	// OnWarning is invoked for every node that the producer chose to drop
	// (e.g. vless+reality). It is safe to leave nil; warnings are then
	// silently swallowed.
	OnWarning func(node *ParsedNode, reason string)
	// ProxiesOnly skips the auto-seeded proxy-groups / rule-providers /
	// rules sections. Used by the rule editor preview, which wants a bare
	// proxies-only base to layer custom rules on top of via the rule
	// injector — without this the injector would have to fight with the
	// producer's defaults (e.g. duplicated MATCH lines).
	ProxiesOnly bool
}

// ClashRenderInput bundles every domain object the producers need to build a
// complete Clash YAML document — nodes plus the user-configured custom rules,
// proxy groups and rule-set providers. The producer factory dispatches on
// `target`; the Clash producer consumes all four fields, while other targets
// (sing-box / URI list / Surge) currently use only Nodes.
//
// Each Record mirrors the corresponding storage layer projection but is
// declared here so the substore package stays free of an import edge on
// internal/storage (storage already depends transitively on substore via the
// adapter; the reverse would create a cycle).
type ClashRenderInput struct {
	Nodes       []*ParsedNode
	CustomRules []CustomRuleRecord
	ProxyGroups []ProxyGroupRecord
	RuleSets    []RuleSetRecord
}

// CustomRuleRecord mirrors storage.CustomRuleRecord — only the fields the
// producer / rule-injector touch are kept. Mode is one of
// "replace" / "prepend" / "append"; Type is "dns" / "rules" / "rule-providers".
type CustomRuleRecord struct {
	ID      string
	Name    string
	Type    string
	Mode    string
	Content string
	Sort    int32
}

// ProxyGroupRecord mirrors storage.ProxyGroupCategoryRecord. MemberProxies and
// MemberGroups carry the JSON-array text exactly as persisted; the producer
// decodes them lazily at render time so storage stays decoupled from the YAML
// shape.
type ProxyGroupRecord struct {
	ID            string
	Name          string
	Type          string // select / url-test / fallback / load-balance / relay
	Icon          string
	SortOrder     int32
	TestURL       string
	TestInterval  int32
	Filter        string
	IncludeAll    bool
	MemberProxies string // JSON array text
	MemberGroups  string // JSON array text
}

// RuleSetRecord mirrors storage.RuleSetProviderRecord. Only the fields the
// Clash rule-providers section consumes are exposed.
type RuleSetRecord struct {
	ID              string
	Name            string
	Behavior        string // domain / ipcidr / classical
	Format          string // yaml / text / mrs
	URL             string
	IntervalSeconds int32
}
