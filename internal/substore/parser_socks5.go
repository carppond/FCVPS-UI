package substore

import (
	"fmt"
	"net/url"
)

// ParseSocks5 parses a socks5://user:pass@host:port#name URI.
//
// Credentials are optional; an unauthenticated socks5://host:port URI is
// returned with empty UUID / Password fields.
func ParseSocks5(uri string) (*ParsedNode, error) {
	if _, err := stripScheme(uri, "socks5"); err != nil {
		return nil, err
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("socks5: %w: %v", ErrInvalidURI, err)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("socks5: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("socks5: %w", err)
	}
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "socks5",
		Server:   u.Hostname(),
		Port:     port,
	}
	if u.User != nil {
		node.UUID = u.User.Username() // username
		if pwd, ok := u.User.Password(); ok {
			node.Password = pwd
		}
	}
	node.Raw = rawCopy(u.Query())
	return node, nil
}
