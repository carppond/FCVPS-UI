package substore

import (
	"fmt"
	"net/url"
)

// ParseHysteria parses a hysteria://host:port?auth=&peer=&...#name URI
// (Hysteria v1). All non-recognised query parameters are preserved in Raw.
func ParseHysteria(uri string) (*ParsedNode, error) {
	if _, err := stripScheme(uri, "hysteria"); err != nil {
		return nil, err
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("hysteria: %w: %v", ErrInvalidURI, err)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("hysteria: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("hysteria: %w", err)
	}
	q := u.Query()
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "hysteria",
		Server:   u.Hostname(),
		Port:     port,
		Password: q.Get("auth"),
		TLS:      true,
		SNI:      pickName(q.Get("peer"), q.Get("sni")),
		ALPN:     splitALPN(q.Get("alpn")),
	}
	node.Raw = rawCopy(q, "auth", "peer", "sni", "alpn")
	return node, nil
}
