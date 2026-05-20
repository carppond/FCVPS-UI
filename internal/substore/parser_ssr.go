package substore

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseSSR parses a ShadowsocksR URI.
//
// Wire format: ssr://base64(host:port:protocol:method:obfs:base64url(password)/?obfsparam=&protoparam=&remarks=&group=)
//
// The fragment-style fields (remarks / group) sit inside the trailing query
// string instead of the URL fragment. We honour both the URL-fragment
// (#name) form for completeness, and the query-string remarks for the
// canonical SSR format.
func ParseSSR(uri string) (*ParsedNode, error) {
	body, err := stripScheme(uri, "ssr")
	if err != nil {
		return nil, err
	}
	raw, err := decodeBase64Loose(body)
	if err != nil {
		return nil, fmt.Errorf("ssr: %w", err)
	}
	full := string(raw)

	// Split body from optional query (/?obfsparam=...&remarks=...&group=...)
	var qs string
	if idx := strings.Index(full, "/?"); idx >= 0 {
		qs = full[idx+2:]
		full = full[:idx]
	} else if idx := strings.IndexByte(full, '?'); idx >= 0 {
		qs = full[idx+1:]
		full = full[:idx]
	}

	parts := strings.Split(full, ":")
	if len(parts) < 6 {
		return nil, fmt.Errorf("ssr: %w: expected 6 colon-separated fields", ErrInvalidURI)
	}
	host := parts[0]
	port, err := parsePort(parts[1])
	if err != nil {
		return nil, fmt.Errorf("ssr: %w", err)
	}
	protocol := parts[2]
	method := parts[3]
	obfs := parts[4]
	pwdRaw, err := decodeBase64Loose(parts[5])
	if err != nil {
		return nil, fmt.Errorf("ssr: password: %w", err)
	}

	q, _ := url.ParseQuery(qs)
	remarks := decodeSSRParam(q.Get("remarks"))
	group := decodeSSRParam(q.Get("group"))
	obfsParam := decodeSSRParam(q.Get("obfsparam"))
	protoParam := decodeSSRParam(q.Get("protoparam"))

	node := &ParsedNode{
		Name:     pickName(remarks, host),
		Tag:      remarks,
		Protocol: "ssr",
		Server:   host,
		Port:     port,
		Method:   method,
		Password: string(pwdRaw),
		Raw: map[string]interface{}{
			"protocol": protocol,
			"obfs":     obfs,
		},
	}
	if obfsParam != "" {
		node.Raw["obfs-param"] = obfsParam
	}
	if protoParam != "" {
		node.Raw["protocol-param"] = protoParam
	}
	if group != "" {
		node.Raw["group"] = group
	}
	return node, nil
}

func decodeSSRParam(v string) string {
	if v == "" {
		return ""
	}
	if raw, err := decodeBase64Loose(v); err == nil {
		return string(raw)
	}
	return v
}
