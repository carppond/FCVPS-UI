package substore

import (
	"encoding/json"
	"fmt"
)

// ProduceSingboxJSON renders a slice of ParsedNode into a sing-box compatible
// JSON document containing just the "outbounds" array.
//
// Output shape:
//
//	{
//	  "outbounds": [
//	    { "tag": "...", "type": "vmess", "server": "...", "server_port": 443, ... },
//	    ...
//	  ]
//	}
//
// sing-box uses distinct type strings and field names (server_port instead of
// port, uuid for vmess/vless/tuic, password for ss/trojan/hy2/tuic) so the
// mapping below is hand-crafted per protocol. Protocols that sing-box does
// not stably support (anytls, naive, ssr) are skipped with a warning rather
// than producing invalid config.
func ProduceSingboxJSON(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, error) {
	outbounds := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		entry, ok := nodeToSingbox(n)
		if !ok {
			if opts.OnWarning != nil {
				opts.OnWarning(n, "sing-box: protocol "+n.Protocol+" not supported, node dropped")
			}
			continue
		}
		outbounds = append(outbounds, entry)
	}
	doc := map[string]interface{}{
		"outbounds": outbounds,
	}
	return json.MarshalIndent(doc, "", "  ")
}

// nodeToSingbox maps a ParsedNode to a sing-box outbound mapping. The
// second return value is false when the protocol is unsupported and the node
// should be skipped.
func nodeToSingbox(n *ParsedNode) (map[string]interface{}, bool) {
	base := func(t string) map[string]interface{} {
		return map[string]interface{}{
			"tag":         n.Name,
			"type":        t,
			"server":      n.Server,
			"server_port": n.Port,
		}
	}
	// Helper: attach a tls block when n.TLS is true.
	attachTLS := func(m map[string]interface{}) {
		if !n.TLS {
			return
		}
		tls := map[string]interface{}{"enabled": true}
		if n.SNI != "" {
			tls["server_name"] = n.SNI
		}
		if len(n.ALPN) > 0 {
			tls["alpn"] = n.ALPN
		}
		m["tls"] = tls
	}
	// Helper: attach a transport block when the node uses ws/grpc/h2.
	attachTransport := func(m map[string]interface{}) {
		switch n.Network {
		case "ws":
			t := map[string]interface{}{"type": "ws"}
			if n.Path != "" {
				t["path"] = n.Path
			}
			if n.Host != "" {
				t["headers"] = map[string]interface{}{"Host": n.Host}
			}
			m["transport"] = t
		case "grpc":
			t := map[string]interface{}{"type": "grpc"}
			if n.Path != "" {
				t["service_name"] = n.Path
			}
			m["transport"] = t
		case "h2", "http":
			t := map[string]interface{}{"type": "http"}
			if n.Path != "" {
				t["path"] = n.Path
			}
			if n.Host != "" {
				t["host"] = []string{n.Host}
			}
			m["transport"] = t
		}
	}

	switch n.Protocol {
	case "vmess":
		m := base("vmess")
		m["uuid"] = n.UUID
		m["security"] = defaultStr(n.Method, "auto")
		// alterId default 0; honour Raw["aid"] if present.
		aid := 0
		if n.Raw != nil {
			if v, ok := n.Raw["aid"].(string); ok {
				_, _ = fmt.Sscanf(v, "%d", &aid)
			}
		}
		m["alter_id"] = aid
		attachTLS(m)
		attachTransport(m)
		return m, true
	case "vless":
		if n.Reality {
			// sing-box supports reality but the parser does not currently
			// retain reality public_key / short_id; drop to keep config valid.
			return nil, false
		}
		m := base("vless")
		m["uuid"] = n.UUID
		// flow defaults to empty; honour Raw["flow"].
		if n.Raw != nil {
			if v, ok := n.Raw["flow"].(string); ok && v != "" {
				m["flow"] = v
			}
		}
		attachTLS(m)
		attachTransport(m)
		return m, true
	case "ss":
		m := base("shadowsocks")
		m["method"] = n.Method
		m["password"] = n.Password
		return m, true
	case "trojan":
		m := base("trojan")
		m["password"] = n.Password
		attachTLS(m)
		attachTransport(m)
		return m, true
	case "hysteria":
		m := base("hysteria")
		// hysteria v1 uses auth_str for password.
		if n.Password != "" {
			m["auth_str"] = n.Password
		}
		attachTLS(m)
		return m, true
	case "hysteria2":
		m := base("hysteria2")
		if n.Password != "" {
			m["password"] = n.Password
		}
		attachTLS(m)
		return m, true
	case "tuic":
		m := base("tuic")
		m["uuid"] = n.UUID
		m["password"] = n.Password
		if n.Raw != nil {
			if v, ok := n.Raw["congestion-control"].(string); ok && v != "" {
				m["congestion_control"] = v
			}
		}
		attachTLS(m)
		return m, true
	case "wireguard":
		m := base("wireguard")
		m["private_key"] = n.Password
		if n.Raw != nil {
			if v, ok := n.Raw["public-key"].(string); ok && v != "" {
				// peer_public_key under "peers[0]" — sing-box requires nested.
				m["peer_public_key"] = v
			}
			if v, ok := n.Raw["address"].(string); ok && v != "" {
				m["local_address"] = []string{v}
			}
			if v, ok := n.Raw["preshared-key"].(string); ok && v != "" {
				m["pre_shared_key"] = v
			}
			if v, ok := n.Raw["mtu"].(string); ok && v != "" {
				var mtu int
				_, _ = fmt.Sscanf(v, "%d", &mtu)
				if mtu > 0 {
					m["mtu"] = mtu
				}
			}
		}
		return m, true
	case "socks5":
		m := base("socks")
		m["version"] = "5"
		if n.UUID != "" {
			m["username"] = n.UUID
		}
		if n.Password != "" {
			m["password"] = n.Password
		}
		return m, true
	default:
		// ssr / anytls / naive — sing-box does not natively support these.
		return nil, false
	}
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
