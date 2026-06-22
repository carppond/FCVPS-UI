package substore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ProduceURIList renders each node as its canonical URI form (vmess:// /
// vless:// / ss:// / trojan:// / hysteria2:// / tuic:// / ...) one per line,
// then base64-encodes the whole document. This is the V2Ray / Shadowrocket /
// Quantumult X / Loon subscription format.
//
// Protocols with no widely deployed URI form (wireguard, anytls) are skipped
// with a warning.
func ProduceURIList(nodes []*ParsedNode, opts ClashProducerOpts) ([]byte, error) {
	var lines []string
	for _, n := range nodes {
		if n == nil {
			continue
		}
		uri, ok := nodeToURI(n)
		if !ok {
			if opts.OnWarning != nil {
				opts.OnWarning(n, "uri-list: protocol "+n.Protocol+" has no URI form, node dropped")
			}
			continue
		}
		lines = append(lines, uri)
	}
	joined := strings.Join(lines, "\n")
	// V2Ray subscription convention: base64-encode the whole list.
	enc := base64.StdEncoding.EncodeToString([]byte(joined))
	return []byte(enc), nil
}

// nodeToURI converts a ParsedNode back to a single-line URI suitable for
// subscription delivery. Returns ok=false when the protocol cannot be
// expressed as a URI.
func nodeToURI(n *ParsedNode) (string, bool) {
	switch n.Protocol {
	case "vmess":
		return vmessToURI(n), true
	case "vless":
		return vlessToURI(n), true
	case "ss":
		return ssToURI(n), true
	case "ssr":
		return ssrToURI(n), true
	case "trojan":
		return trojanToURI(n), true
	case "hysteria":
		return hysteriaToURI(n), true
	case "hysteria2":
		return hysteria2ToURI(n), true
	case "tuic":
		return tuicToURI(n), true
	case "socks5":
		return socks5ToURI(n), true
	default:
		return "", false
	}
}

// vmessToURI emits the V2RayN flavoured vmess://<base64(json)> form.
func vmessToURI(n *ParsedNode) string {
	obj := map[string]interface{}{
		"v":    "2",
		"ps":   n.Name,
		"add":  n.Server,
		"port": strconv.Itoa(n.Port),
		"id":   n.UUID,
		"aid":  "0",
		"scy":  defaultStr(n.Method, "auto"),
		"net":  defaultStr(n.Network, "tcp"),
		"host": n.Host,
		"path": n.Path,
	}
	if n.TLS {
		obj["tls"] = "tls"
	}
	if n.SNI != "" {
		obj["sni"] = n.SNI
	}
	if len(n.ALPN) > 0 {
		obj["alpn"] = strings.Join(n.ALPN, ",")
	}
	if n.Raw != nil {
		if v, ok := n.Raw["aid"].(string); ok && v != "" {
			obj["aid"] = v
		}
		if v, ok := n.Raw["fp"].(string); ok && v != "" {
			obj["fp"] = v
		}
		if v, ok := n.Raw["type"].(string); ok && v != "" {
			obj["type"] = v
		}
	}
	buf, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(buf)
}

// vlessToURI emits vless://uuid@host:port?security=&type=&...#name.
func vlessToURI(n *ParsedNode) string {
	q := url.Values{}
	// VLESS carries no transport encryption of its own, but "encryption=none"
	// is a MANDATORY field in the share-link format — Shadowrocket, v2rayN and
	// others fail to import / connect when it is missing.
	q.Set("encryption", "none")
	if n.TLS {
		if n.Reality {
			q.Set("security", "reality")
		} else {
			q.Set("security", "tls")
		}
	} else {
		q.Set("security", "none")
	}
	q.Set("type", defaultStr(n.Network, "tcp"))
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.Path != "" {
		q.Set("path", n.Path)
	}
	if n.Host != "" {
		q.Set("host", n.Host)
	}
	// Reality / xtls extras the parser keeps verbatim in Raw. Without these
	// (flow / fp / pbk / sid / spx) a reality node TCP-pings but fails the
	// handshake — the node is unusable in clients fed the uri-list format.
	for _, k := range []string{"flow", "fp", "pbk", "sid", "spx"} {
		if v, ok := stringFromRaw(n.Raw, k); ok && v != "" {
			q.Set(k, v)
		}
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		n.UUID, n.Server, n.Port, q.Encode(), url.QueryEscape(n.Name))
}

// ssToURI emits the SIP002 form ss://base64(method:password)@host:port#name.
func ssToURI(n *ParsedNode) string {
	cred := base64.RawURLEncoding.EncodeToString([]byte(n.Method + ":" + n.Password))
	return fmt.Sprintf("ss://%s@%s:%d#%s",
		cred, n.Server, n.Port, url.QueryEscape(n.Name))
}

// ssrToURI emits the canonical ssr://base64(host:port:protocol:method:obfs:base64(password)/?remarks=...) form.
func ssrToURI(n *ParsedNode) string {
	protocol := "origin"
	obfs := "plain"
	var obfsParam, protoParam, group string
	if n.Raw != nil {
		if v, ok := n.Raw["protocol"].(string); ok && v != "" {
			protocol = v
		}
		if v, ok := n.Raw["obfs"].(string); ok && v != "" {
			obfs = v
		}
		if v, ok := n.Raw["obfs-param"].(string); ok {
			obfsParam = v
		}
		if v, ok := n.Raw["protocol-param"].(string); ok {
			protoParam = v
		}
		if v, ok := n.Raw["group"].(string); ok {
			group = v
		}
	}
	pwdEnc := base64.RawURLEncoding.EncodeToString([]byte(n.Password))
	body := fmt.Sprintf("%s:%d:%s:%s:%s:%s", n.Server, n.Port, protocol, n.Method, obfs, pwdEnc)
	q := url.Values{}
	if n.Name != "" {
		q.Set("remarks", base64.RawURLEncoding.EncodeToString([]byte(n.Name)))
	}
	if obfsParam != "" {
		q.Set("obfsparam", base64.RawURLEncoding.EncodeToString([]byte(obfsParam)))
	}
	if protoParam != "" {
		q.Set("protoparam", base64.RawURLEncoding.EncodeToString([]byte(protoParam)))
	}
	if group != "" {
		q.Set("group", base64.RawURLEncoding.EncodeToString([]byte(group)))
	}
	full := body + "/?" + q.Encode()
	return "ssr://" + base64.RawURLEncoding.EncodeToString([]byte(full))
}

// trojanToURI emits trojan://password@host:port?security=tls&...#name.
func trojanToURI(n *ParsedNode) string {
	q := url.Values{}
	if !n.TLS {
		q.Set("security", "none")
	}
	if n.Network != "" && n.Network != "tcp" {
		q.Set("type", n.Network)
	}
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.Path != "" {
		q.Set("path", n.Path)
	}
	if n.Host != "" {
		q.Set("host", n.Host)
	}
	qs := q.Encode()
	if qs != "" {
		qs = "?" + qs
	}
	return fmt.Sprintf("trojan://%s@%s:%d%s#%s",
		url.QueryEscape(n.Password), n.Server, n.Port, qs, url.QueryEscape(n.Name))
}

func hysteriaToURI(n *ParsedNode) string {
	q := url.Values{}
	if n.Password != "" {
		q.Set("auth", n.Password)
	}
	if n.SNI != "" {
		q.Set("peer", n.SNI)
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	return fmt.Sprintf("hysteria://%s:%d?%s#%s",
		n.Server, n.Port, q.Encode(), url.QueryEscape(n.Name))
}

func hysteria2ToURI(n *ParsedNode) string {
	q := url.Values{}
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	qs := q.Encode()
	if qs != "" {
		qs = "?" + qs
	}
	return fmt.Sprintf("hysteria2://%s@%s:%d%s#%s",
		url.QueryEscape(n.Password), n.Server, n.Port, qs, url.QueryEscape(n.Name))
}

func tuicToURI(n *ParsedNode) string {
	q := url.Values{}
	if n.SNI != "" {
		q.Set("sni", n.SNI)
	}
	if len(n.ALPN) > 0 {
		q.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.Raw != nil {
		if v, ok := n.Raw["congestion-control"].(string); ok && v != "" {
			q.Set("congestion_control", v)
		}
	}
	qs := q.Encode()
	if qs != "" {
		qs = "?" + qs
	}
	return fmt.Sprintf("tuic://%s:%s@%s:%d%s#%s",
		n.UUID, url.QueryEscape(n.Password), n.Server, n.Port, qs, url.QueryEscape(n.Name))
}

func socks5ToURI(n *ParsedNode) string {
	cred := ""
	if n.UUID != "" || n.Password != "" {
		cred = url.QueryEscape(n.UUID) + ":" + url.QueryEscape(n.Password) + "@"
	}
	return fmt.Sprintf("socks5://%s%s:%d#%s",
		cred, n.Server, n.Port, url.QueryEscape(n.Name))
}
