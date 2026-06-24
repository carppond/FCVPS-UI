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
		parts = []string{
			name + " = vmess", server, port,
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
		parts = appendSurgeInsecure(parts, n)
	case "trojan":
		parts = []string{
			name + " = trojan", server, port,
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
		parts = appendSurgeInsecure(parts, n)
	case "ss":
		parts = []string{
			name + " = ss", server, port,
			"encrypt-method=" + n.Method,
			"password=" + n.Password,
		}
		// SIP002 simple-obfs → Surge obfs=/obfs-host=. v2ray-plugin can't be
		// represented in Surge, so skip those nodes rather than emit a broken
		// (plugin-less) line that would silently fail to connect.
		if plugin, ok := stringFromRaw(n.Raw, "plugin"); ok && plugin != "" {
			pname, popts, _ := strings.Cut(plugin, ";")
			if !strings.Contains(pname, "obfs") {
				return "", false
			}
			mode, host := parseSimpleObfs(popts)
			if mode != "" {
				parts = append(parts, "obfs="+mode)
			}
			if host != "" {
				parts = append(parts, "obfs-host="+host)
			}
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

// appendSurgeInsecure adds skip-cert-verify=true when the node carries a
// "skip certificate verification" intent (self-signed / expired upstream cert);
// without it Surge fails TLS verification and the node can't connect.
func appendSurgeInsecure(parts []string, n *ParsedNode) []string {
	if n.TLS && rawBool(n.Raw, "skip-cert-verify", "allowInsecure", "insecure", "allow_insecure") {
		parts = append(parts, "skip-cert-verify=true")
	}
	return parts
}

// parseSimpleObfs extracts obfs mode + host from a SIP002 simple-obfs opts
// string like "obfs=tls;obfs-host=a.com".
func parseSimpleObfs(opts string) (mode, host string) {
	for _, kv := range strings.Split(opts, ";") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "obfs":
			mode = strings.TrimSpace(v)
		case "obfs-host":
			host = strings.TrimSpace(v)
		}
	}
	return mode, host
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
