package substore

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseSS parses a Shadowsocks URI in either SIP002 or the legacy plain
// base64 form.
//
//   - SIP002: ss://base64(method:password)@host:port/?plugin=...#name
//   - Legacy: ss://base64(method:password@host:port)#name
//
// Plugins and other unknown query params are preserved in ParsedNode.Raw.
func ParseSS(uri string) (*ParsedNode, error) {
	body, err := stripScheme(uri, "ss")
	if err != nil {
		return nil, err
	}
	// Pull out the fragment so it does not interfere with base64 detection.
	var frag string
	if idx := strings.IndexByte(body, '#'); idx >= 0 {
		frag = body[idx+1:]
		body = body[:idx]
	}
	if decoded, err := url.QueryUnescape(frag); err == nil {
		frag = decoded
	}

	// Detect SIP002 vs legacy: SIP002 contains an '@' separating userinfo
	// from host. Legacy form has no '@' before optional '/' or '?'.
	if strings.Contains(body, "@") {
		return parseSSSIP002("ss://" + body + "#" + frag)
	}
	// Legacy form: decode the whole body as base64(method:password@host:port).
	raw, err := decodeBase64Loose(body)
	if err != nil {
		return nil, fmt.Errorf("ss: %w", err)
	}
	return parseSSPlain(string(raw), frag)
}

func parseSSSIP002(uri string) (*ParsedNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("ss: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil {
		return nil, fmt.Errorf("ss: %w: userinfo", ErrMissingField)
	}
	// SIP002 userinfo is base64(method:password) but may already be plain.
	username := u.User.Username()
	password, hasPwd := u.User.Password()
	var method, pwd string
	if hasPwd {
		method, pwd = username, password
	} else {
		raw, err := decodeBase64Loose(username)
		if err != nil {
			return nil, fmt.Errorf("ss: userinfo: %w", err)
		}
		parts := strings.SplitN(string(raw), ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("ss: %w: method:password", ErrInvalidURI)
		}
		method, pwd = parts[0], parts[1]
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("ss: %w", err)
	}
	q := u.Query()
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "ss",
		Server:   u.Hostname(),
		Port:     port,
		Method:   method,
		Password: pwd,
	}
	node.Raw = rawCopy(q)
	return node, nil
}

func parseSSPlain(decoded, frag string) (*ParsedNode, error) {
	// Expected layout: method:password@host:port
	at := strings.LastIndexByte(decoded, '@')
	if at < 0 {
		return nil, fmt.Errorf("ss: %w: missing @", ErrInvalidURI)
	}
	cred := decoded[:at]
	endpoint := decoded[at+1:]
	mp := strings.SplitN(cred, ":", 2)
	if len(mp) != 2 {
		return nil, fmt.Errorf("ss: %w: method:password", ErrInvalidURI)
	}
	hp := strings.LastIndexByte(endpoint, ':')
	if hp < 0 {
		return nil, fmt.Errorf("ss: %w: host:port", ErrInvalidURI)
	}
	host := endpoint[:hp]
	port, err := parsePort(endpoint[hp+1:])
	if err != nil {
		return nil, fmt.Errorf("ss: %w", err)
	}
	return &ParsedNode{
		Name:     pickName(frag, host),
		Tag:      frag,
		Protocol: "ss",
		Server:   host,
		Port:     port,
		Method:   mp[0],
		Password: mp[1],
	}, nil
}
