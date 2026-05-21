package substore

import (
	"fmt"
	"strconv"
	"strings"
)

// ProduceSurgeConf renders the supported subset of nodes as a Surge `.conf`
// fragment containing a single `[Proxy]` section.
//
// Surge syntax (from a single line):
//
//	<name> = <type>, <server>, <port>, <key>=<value>, <key>=<value>, ...
//
// Supported types per Surge docs (v4+):
//   - vmess
//   - trojan
//   - ss (shadowsocks)
//   - http
//   - socks5 / socks5-tls
//
// Protocols Surge does not support (hysteria/hysteria2/tuic/wireguard/vless
// /ssr/anytls/naive) are emitted as comment lines so the operator can spot
// them when comparing client output.
func ProduceSurgeConf(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, error) {
	var b strings.Builder
	b.WriteString("[Proxy]\n")
	for _, n := range nodes {
		if n == nil {
			continue
		}
		line, ok := nodeToSurge(n)
		if !ok {
			if opts.OnWarning != nil {
				opts.OnWarning(n, "surge: protocol "+n.Protocol+" not supported")
			}
			b.WriteString("# unsupported: " + n.Name + " (" + n.Protocol + ")\n")
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func nodeToSurge(n *ParsedNode) (string, bool) {
	// Surge does not tolerate commas in the name; replace with hyphen so the
	// rest of the comma-separated record stays parseable.
	name := strings.ReplaceAll(n.Name, ",", "-")
	server := n.Server
	port := strconv.Itoa(n.Port)
	var parts []string
	switch n.Protocol {
	case "vmess":
		parts = []string{name + " = vmess", server, port,
			"username=" + n.UUID,
		}
		if n.TLS {
			parts = append(parts, "tls=true")
		}
		if n.SNI != "" {
			parts = append(parts, "sni="+n.SNI)
		}
		if n.Network == "ws" {
			parts = append(parts, "ws=true")
			if n.Path != "" {
				parts = append(parts, "ws-path="+n.Path)
			}
			if n.Host != "" {
				parts = append(parts, "ws-headers=Host:"+n.Host)
			}
		}
	case "trojan":
		parts = []string{name + " = trojan", server, port,
			"password=" + n.Password,
		}
		if n.SNI != "" {
			parts = append(parts, "sni="+n.SNI)
		}
		if n.Network == "ws" {
			parts = append(parts, "ws=true")
			if n.Path != "" {
				parts = append(parts, "ws-path="+n.Path)
			}
			if n.Host != "" {
				parts = append(parts, "ws-headers=Host:"+n.Host)
			}
		}
	case "ss":
		parts = []string{name + " = ss", server, port,
			"encrypt-method=" + n.Method,
			"password=" + n.Password,
		}
	case "http":
		parts = []string{name + " = http", server, port}
		if n.UUID != "" {
			parts = append(parts, "username="+n.UUID)
		}
		if n.Password != "" {
			parts = append(parts, "password="+n.Password)
		}
	case "socks5":
		tag := "socks5"
		if n.TLS {
			tag = "socks5-tls"
		}
		parts = []string{name + " = " + tag, server, port}
		if n.UUID != "" {
			parts = append(parts, "username="+n.UUID)
		}
		if n.Password != "" {
			parts = append(parts, "password="+n.Password)
		}
	default:
		return "", false
	}
	return strings.Join(parts, ", "), true
}

// quoteIfNeeded wraps a value in quotes when it contains a comma. Surge
// allows quoted values for parameters that may legitimately carry commas
// (e.g. alpn lists). Currently unused but kept for future Surge advanced
// fields.
//
//nolint:unused
func quoteIfNeeded(v string) string {
	if strings.ContainsAny(v, ", ") {
		return fmt.Sprintf("%q", v)
	}
	return v
}
