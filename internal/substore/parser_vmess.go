package substore

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// vmessV2RayN mirrors the JSON shape produced by V2RayN style vmess:// URIs.
// Numeric fields are typed as interface{} so we can tolerate hosts that
// emit them as either JSON numbers or strings (V2RayN historically did both).
type vmessV2RayN struct {
	V    string      `json:"v"`
	PS   string      `json:"ps"`
	Add  string      `json:"add"`
	Port interface{} `json:"port"`
	ID   string      `json:"id"`
	Aid  interface{} `json:"aid"`
	SCY  string      `json:"scy"`
	Net  string      `json:"net"`
	Type string      `json:"type"`
	Host string      `json:"host"`
	Path string      `json:"path"`
	TLS  string      `json:"tls"`
	SNI  string      `json:"sni"`
	ALPN string      `json:"alpn"`
	FP   string      `json:"fp"`
}

// ParseVmess parses a vmess:// URI in the V2RayN base64-JSON format.
//
// Wire format: vmess://<base64(json blob)>
// The fragment (#name) of the URI form is not commonly used; the JSON "ps"
// field carries the display name instead.
func ParseVmess(uri string) (*ParsedNode, error) {
	body, err := stripScheme(uri, "vmess")
	if err != nil {
		return nil, err
	}
	// Some implementations append "#name" after the base64 blob; honour it
	// only when present so legitimate base64 padding ("=") is not mistaken
	// for the fragment delimiter.
	var frag string
	if idx := strings.IndexByte(body, '#'); idx >= 0 {
		frag = body[idx+1:]
		body = body[:idx]
	}
	raw, err := decodeBase64Loose(body)
	if err != nil {
		return nil, fmt.Errorf("vmess: %w", err)
	}
	var v vmessV2RayN
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("vmess: decode json: %w", err)
	}

	if v.Add == "" {
		return nil, fmt.Errorf("vmess: %w: add", ErrMissingField)
	}
	if v.ID == "" {
		return nil, fmt.Errorf("vmess: %w: id", ErrMissingField)
	}
	port, err := parsePort(stringify(v.Port))
	if err != nil {
		return nil, fmt.Errorf("vmess: %w", err)
	}

	method := strings.ToLower(v.SCY)
	if method == "" {
		method = "auto"
	}

	node := &ParsedNode{
		Name:     pickName(v.PS, frag, v.Add+":"+strconv.Itoa(port)),
		Tag:      frag,
		Protocol: "vmess",
		Server:   v.Add,
		Port:     port,
		UUID:     v.ID,
		Method:   method,
		Network:  defaultNet(v.Net),
		Host:     v.Host,
		Path:     v.Path,
		TLS:      strings.EqualFold(v.TLS, "tls"),
		SNI:      v.SNI,
		ALPN:     splitALPN(v.ALPN),
	}
	extra := map[string]interface{}{}
	if aid := stringify(v.Aid); aid != "" {
		extra["aid"] = aid
	}
	if v.Type != "" {
		extra["type"] = v.Type
	}
	if v.V != "" {
		extra["v"] = v.V
	}
	if v.FP != "" {
		extra["fp"] = v.FP
	}
	if len(extra) > 0 {
		node.Raw = extra
	}
	return node, nil
}

// pickName returns the first non-empty candidate; useful for picking a name
// from a chain of fallbacks (e.g. ps -> fragment -> host:port).
func pickName(candidates ...string) string {
	for _, c := range candidates {
		if c = strings.TrimSpace(c); c != "" {
			return c
		}
	}
	return ""
}

func defaultNet(s string) string {
	if s == "" {
		return "tcp"
	}
	return strings.ToLower(s)
}

// stringify coerces an arbitrary JSON value into its string representation
// while accommodating the V2RayN vmess quirk where numeric fields can be
// emitted either as JSON numbers or as quoted strings.
func stringify(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", t)
	}
}
