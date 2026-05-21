package substore

import "strings"

// Producer is the common interface implemented by every output format
// (Clash YAML / sing-box JSON / URI list / Surge conf). Callers obtain a
// Producer via ProducerFactory.Get(target).
type Producer interface {
	// Produce renders the nodes into the producer's native format and returns
	// (body, contentType, err). contentType is the value to write in the
	// HTTP Content-Type header.
	Produce(nodes []*ParsedNode, opts ClashProducerOpts) (body []byte, contentType string, err error)
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

// clashProducer adapts ProduceClashYAML to the Producer interface.
type clashProducer struct{}

func (clashProducer) Produce(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceClashYAML(nodes, opts)
	if err != nil {
		return nil, "", err
	}
	return body, "text/yaml; charset=utf-8", nil
}

// singboxProducer adapts ProduceSingboxJSON to the Producer interface.
type singboxProducer struct{}

func (singboxProducer) Produce(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceSingboxJSON(nodes, opts)
	if err != nil {
		return nil, "", err
	}
	return body, "application/json; charset=utf-8", nil
}

// uriListProducer adapts ProduceURIList to the Producer interface.
type uriListProducer struct{}

func (uriListProducer) Produce(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceURIList(nodes, opts)
	if err != nil {
		return nil, "", err
	}
	return body, "text/plain; charset=utf-8", nil
}

// surgeProducer adapts ProduceSurgeConf to the Producer interface.
type surgeProducer struct{}

func (surgeProducer) Produce(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, string, error) {
	body, err := ProduceSurgeConf(nodes, opts)
	if err != nil {
		return nil, "", err
	}
	return body, "text/plain; charset=utf-8", nil
}
