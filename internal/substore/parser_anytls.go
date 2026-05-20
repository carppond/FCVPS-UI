package substore

import (
	"fmt"
	"net/url"
)

// ParseAnyTLS parses an anytls://password@host:port?...#name URI.
//
// AnyTLS is a relatively new transport; we only persist the well known
// fields (sni / alpn / fingerprint) and stash the rest in Raw for callers.
func ParseAnyTLS(uri string) (*ParsedNode, error) {
	if _, err := stripScheme(uri, "anytls"); err != nil {
		return nil, err
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("anytls: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("anytls: %w: password", ErrMissingField)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("anytls: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("anytls: %w", err)
	}
	q := u.Query()
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "anytls",
		Server:   u.Hostname(),
		Port:     port,
		Password: u.User.Username(),
		TLS:      true,
		SNI:      pickName(q.Get("sni"), q.Get("peer")),
		ALPN:     splitALPN(q.Get("alpn")),
	}
	node.Raw = rawCopy(q, "sni", "peer", "alpn")
	return node, nil
}
