package substore

import "strings"

// Producer is the common interface implemented by every output format
// (Clash YAML / sing-box JSON / URI list / Surge conf). Callers obtain a
// Producer via ProducerFactory.Get(target).
//
// Each Produce call receives a ClashRenderInput; the Clash producer consumes
// nodes + custom rules + proxy groups + rule-set providers, while every other
// target currently only reads input.Nodes (sing-box / Surge routing rule
// emission is deliberately deferred — see T-fix Clash bug ticket).
type Producer interface {
	// Produce renders the input into the producer's native format and
	// returns (body, contentType, err). contentType is the value to write in
	// the HTTP Content-Type header.
	Produce(input *ClashRenderInput, opts ClashProducerOpts) (body []byte, contentType string, err error)
}

// ProducerFactory routes a string target ("clash" / "singbox" / ...) to a
// concrete Producer. Unknown targets default to ClashProducer so the legacy
// /download/:name endpoint (no target query) stays backward compatible.
type ProducerFactory struct{}

// NewProducerFactory returns the singleton factory; the type itself is
// stateless so the constructor exists purely for symmetry with the rest of
// the package.
func NewProducerFactory() *ProducerFactory { return &ProducerFactory{} }

// Get returns the Producer for the given target. The match is
// case-insensitive and tolerates the common aliases used by sub-store v2
// (clash-verge, mihomo, stash, surge-mac, ...).
func (ProducerFactory) Get(target string) Producer {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "", "clash", "clashmeta", "clash-meta", "clash-verge", "clash-verge-rev",
		"mihomo", "stash", "openclash":
		return clashProducer{}
	case "singbox", "sing-box":
		return singboxProducer{}
	case "surge", "surge-mac", "surgemac", "surge-ios":
		return surgeProducer{}
	case "v2ray", "v2rayn", "v2rayng", "shadowrocket",
		"qx", "quantumult", "quantumult-x", "quantumultx",
		"loon", "uri":
		return uriListProducer{}
	default:
		// Unknown target — keep the legacy default (Clash YAML) so existing
		// consumers do not break.
		return clashProducer{}
	}
}

// TargetContentType returns the canonical Content-Type for a target without
// rendering any nodes. Handy for HEAD-style discovery flows; kept exported
// for future use.
func (ProducerFactory) TargetContentType(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "singbox", "sing-box":
		return "application/json; charset=utf-8"
	case "surge", "surge-mac", "surgemac", "surge-ios":
		return "text/plain; charset=utf-8"
	case "v2ray", "v2rayn", "v2rayng", "shadowrocket",
		"qx", "quantumult", "quantumult-x", "quantumultx",
		"loon", "uri":
		return "text/plain; charset=utf-8"
	default:
		return "text/yaml; charset=utf-8"
	}
}

// nodesFromInput is a defensive helper for producers that only consume the
// node slice. It tolerates a nil input so callers (tests, legacy code paths)
// can pass &ClashRenderInput{Nodes: ...} without populating the rest.
func nodesFromInput(input *ClashRenderInput) []*ParsedNode {
	if input == nil {
		return nil
	}
	return input.Nodes
}

// clashProducer adapts ProduceClashYAML to the Producer interface.
type clashProducer struct{}

func (clashProducer) Produce(input *ClashRenderInput, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceClashYAML(input, opts)
	if err != nil {
		return nil, "", err
	}
	return body, "text/yaml; charset=utf-8", nil
}

// singboxProducer adapts ProduceSingboxJSON to the Producer interface.
// Routing rule emission (rules / proxy-groups equivalent) is deferred;
// callers see only the outbounds array for now.
type singboxProducer struct{}

func (singboxProducer) Produce(input *ClashRenderInput, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceSingboxJSON(nodesFromInput(input), opts)
	if err != nil {
		return nil, "", err
	}
	return body, "application/json; charset=utf-8", nil
}

// uriListProducer adapts ProduceURIList to the Producer interface.
type uriListProducer struct{}

func (uriListProducer) Produce(input *ClashRenderInput, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceURIList(nodesFromInput(input), opts)
	if err != nil {
		return nil, "", err
	}
	return body, "text/plain; charset=utf-8", nil
}

// surgeProducer adapts ProduceSurgeConf to the Producer interface. Rule
// emission for Surge is intentionally deferred (Surge syntax is conf-section
// based and differs significantly from Clash YAML).
type surgeProducer struct{}

func (surgeProducer) Produce(input *ClashRenderInput, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceSurgeConf(nodesFromInput(input), opts)
	if err != nil {
		return nil, "", err
	}
	return body, "text/plain; charset=utf-8", nil
}
