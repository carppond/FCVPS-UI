package substore

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseTUIC parses a tuic://uuid:password@host:port?...#name URI (TUIC v5).
//
// Older v4-style URIs that carry only a UUID (without password) are also
// accepted; the password is left empty in that case.
func ParseTUIC(uri string) (*ParsedNode, error) {
	if _, err := stripScheme(uri, "tuic"); err != nil {
		return nil, err
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("tuic: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("tuic: %w: uuid", ErrMissingField)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("tuic: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("tuic: %w", err)
	}
	password, _ := u.User.Password()
	q := u.Query()
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "tuic",
		Server:   u.Hostname(),
		Port:     port,
		UUID:     u.User.Username(),
		Password: password,
		TLS:      true,
		SNI:      pickName(q.Get("sni"), q.Get("peer")),
		ALPN:     splitALPN(q.Get("alpn")),
	}
	if cc := q.Get("congestion_control"); cc != "" {
		node.Raw = map[string]interface{}{"congestion-control": strings.ToLower(cc)}
	}
	extra := rawCopy(q, "sni", "peer", "alpn", "congestion_control")
	for k, v := range extra {
		if node.Raw == nil {
			node.Raw = map[string]interface{}{}
		}
		node.Raw[k] = v
	}
	return node, nil
}
