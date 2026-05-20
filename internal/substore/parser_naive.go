package substore

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseNaive parses a naive+https://user:pass@host:port#name URI.
//
// Only the https flavour is honoured; naive+quic / naive+h2 are returned as
// the same parsed shape but with the transport recorded in Raw so the Clash
// producer can choose how to render them.
func ParseNaive(uri string) (*ParsedNode, error) {
	uri = strings.TrimSpace(uri)
	if !strings.HasPrefix(uri, "naive+") {
		return nil, fmt.Errorf("naive: %w: expected naive+...://", ErrInvalidURI)
	}
	rest := strings.TrimPrefix(uri, "naive+")
	transport, _, ok := strings.Cut(rest, "://")
	if !ok {
		return nil, fmt.Errorf("naive: %w: missing transport", ErrInvalidURI)
	}
	// Parse the underlying URL with the synthetic transport scheme.
	u, err := url.Parse(rest)
	if err != nil {
		return nil, fmt.Errorf("naive: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("naive: %w: user", ErrMissingField)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("naive: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("naive: %w", err)
	}
	pwd, _ := u.User.Password()
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "naive",
		Server:   u.Hostname(),
		Port:     port,
		UUID:     u.User.Username(),
		Password: pwd,
		TLS:      transport == "https" || transport == "quic",
		Raw: map[string]interface{}{
			"transport": transport,
		},
	}
	extra := rawCopy(u.Query())
	for k, v := range extra {
		node.Raw[k] = v
	}
	return node, nil
}
