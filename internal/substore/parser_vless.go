package substore

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseVless parses a vless://uuid@host:port?security=&type=&...#name URI.
//
// Both standard TLS and Reality variants are accepted. A Reality node is
// flagged via ParsedNode.Reality=true so the Clash producer can filter it
// (Clash core does not yet stably support reality; PRD M-SUB.2).
func ParseVless(uri string) (*ParsedNode, error) {
	if _, err := stripScheme(uri, "vless"); err != nil {
		return nil, err
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("vless: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("vless: %w: uuid", ErrMissingField)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("vless: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("vless: %w", err)
	}
	q := u.Query()
	security := strings.ToLower(q.Get("security"))
	netw := strings.ToLower(q.Get("type"))
	if netw == "" {
		netw = "tcp"
	}
	frag := decodeFragment(u)

	node := &ParsedNode{
		Name:     pickName(frag, u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "vless",
		Server:   u.Hostname(),
		Port:     port,
		UUID:     u.User.Username(),
		Network:  netw,
		TLS:      security == "tls" || security == "reality" || security == "xtls",
		SNI:      q.Get("sni"),
		ALPN:     splitALPN(q.Get("alpn")),
		Path:     q.Get("path"),
		Host:     q.Get("host"),
		Reality:  security == "reality",
	}
	node.Raw = rawCopy(q, "security", "type", "sni", "alpn", "path", "host")
	return node, nil
}
