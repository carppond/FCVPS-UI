package substore

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseHysteria2 parses a hy2:// or hysteria2:// URI.
//
// Wire format: hysteria2://password@host:port?sni=&insecure=&obfs=&obfs-password=#name
// `hy2://` is the canonical short alias.
func ParseHysteria2(uri string) (*ParsedNode, error) {
	uri = strings.TrimSpace(uri)
	switch {
	case strings.HasPrefix(uri, "hysteria2://"):
		// already canonical
	case strings.HasPrefix(uri, "hy2://"):
		uri = "hysteria2://" + strings.TrimPrefix(uri, "hy2://")
	default:
		return nil, fmt.Errorf("hysteria2: %w: expected hy2:// or hysteria2://", ErrInvalidURI)
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("hysteria2: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("hysteria2: %w: password", ErrMissingField)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("hysteria2: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("hysteria2: %w", err)
	}
	q := u.Query()
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "hysteria2",
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
