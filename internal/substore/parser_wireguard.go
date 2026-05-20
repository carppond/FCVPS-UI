package substore

import (
	"bufio"
	"fmt"
	"net/url"
	"strings"
)

// ParseWireguard parses either a custom wireguard:// URI or an INI .conf
// document.
//
// URI form (custom):
//
//	wireguard://privatekey@host:port?public_key=&preshared_key=&address=&dns=&mtu=#name
//
// INI form: a standard wg-quick configuration with [Interface] and [Peer]
// sections; the input must begin with "[" for the parser to choose this path.
func ParseWireguard(uri string) (*ParsedNode, error) {
	trim := strings.TrimSpace(uri)
	if strings.HasPrefix(trim, "[") {
		return parseWireguardINI(trim)
	}
	if _, err := stripScheme(trim, "wireguard"); err != nil {
		return nil, err
	}
	u, err := url.Parse(trim)
	if err != nil {
		return nil, fmt.Errorf("wireguard: %w: %v", ErrInvalidURI, err)
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("wireguard: %w: private_key", ErrMissingField)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("wireguard: %w: host", ErrMissingField)
	}
	port, err := parsePort(u.Port())
	if err != nil {
		return nil, fmt.Errorf("wireguard: %w", err)
	}
	q := u.Query()
	node := &ParsedNode{
		Name:     pickName(decodeFragment(u), u.Hostname()),
		Tag:      u.Fragment,
		Protocol: "wireguard",
		Server:   u.Hostname(),
		Port:     port,
		Password: u.User.Username(), // private key
		Raw: map[string]interface{}{
			"public-key": q.Get("public_key"),
		},
	}
	if v := q.Get("preshared_key"); v != "" {
		node.Raw["preshared-key"] = v
	}
	if v := q.Get("address"); v != "" {
		node.Raw["address"] = v
	}
	if v := q.Get("dns"); v != "" {
		node.Raw["dns"] = v
	}
	if v := q.Get("mtu"); v != "" {
		node.Raw["mtu"] = v
	}
	return node, nil
}

func parseWireguardINI(text string) (*ParsedNode, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	section := ""
	cfg := map[string]map[string]string{
		"Interface": {},
		"Peer":      {},
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if _, ok := cfg[section]; !ok {
				cfg[section] = map[string]string{}
			}
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if section == "" {
			continue
		}
		cfg[section][key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("wireguard: scan ini: %w", err)
	}

	iface := cfg["Interface"]
	peer := cfg["Peer"]
	if iface["PrivateKey"] == "" {
		return nil, fmt.Errorf("wireguard: %w: Interface.PrivateKey", ErrMissingField)
	}
	if peer["Endpoint"] == "" {
		return nil, fmt.Errorf("wireguard: %w: Peer.Endpoint", ErrMissingField)
	}
	host, portStr, ok := strings.Cut(peer["Endpoint"], ":")
	if !ok {
		return nil, fmt.Errorf("wireguard: %w: Peer.Endpoint missing port", ErrInvalidURI)
	}
	port, err := parsePort(portStr)
	if err != nil {
		return nil, fmt.Errorf("wireguard: %w", err)
	}
	node := &ParsedNode{
		Name:     host,
		Protocol: "wireguard",
		Server:   host,
		Port:     port,
		Password: iface["PrivateKey"],
		Raw: map[string]interface{}{
			"public-key": peer["PublicKey"],
		},
	}
	if v := iface["Address"]; v != "" {
		node.Raw["address"] = v
	}
	if v := iface["DNS"]; v != "" {
		node.Raw["dns"] = v
	}
	if v := iface["MTU"]; v != "" {
		node.Raw["mtu"] = v
	}
	if v := peer["PresharedKey"]; v != "" {
		node.Raw["preshared-key"] = v
	}
	if v := peer["AllowedIPs"]; v != "" {
		node.Raw["allowed-ips"] = v
	}
	return node, nil
}
