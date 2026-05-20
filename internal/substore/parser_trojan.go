package substore

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseTrojan parses a trojan://password@host:port?...#name URI.
//
// Trojan defaults to TLS; the `security` / `tls` query parameters override
// when the URI specifies otherwise. WebSocket transport (type=ws) is honoured
// alongside the default tcp transport.
func ParseTrojan(uri string) (*ParsedNode, error) {
	if _, err := stripScheme(uri, "trojan"); err != nil {
		return nil, err
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("trojan: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("trojan: %w: password", ErrMissingField)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("trojan: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("trojan: %w", err)
	}
	q := u.Query()
	netw := strings.ToLower(q.Get("type"))
	if netw == "" {
		netw = "tcp"
	}

	tls := true
	if v := strings.ToLower(q.Get("security")); v == "none" || v == "off" {
		tls = false
	}
	if v := strings.ToLower(q.Get("tls")); v == "false" || v == "0" {
		tls = false
	}

	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "trojan",
		Server:   u.Hostname(),
		Port:     port,
		Password: u.User.Username(),
		Network:  netw,
		TLS:      tls,
		SNI:      pickName(q.Get("sni"), q.Get("peer")),
		ALPN:     splitALPN(q.Get("alpn")),
		Path:     q.Get("path"),
		Host:     q.Get("host"),
	}
	node.Raw = rawCopy(q, "security", "tls", "type", "sni", "peer", "alpn", "path", "host")
	return node, nil
}
