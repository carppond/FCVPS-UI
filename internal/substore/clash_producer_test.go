package substore

import (
	"strings"
	"testing"
)

func TestProduceClashYAML_BasicShape(t *testing.T) {
	nodes := []*ParsedNode{
		{Name: "n1", Protocol: "vmess", Server: "a.example.com", Port: 443, UUID: "uuid1", Network: "ws", TLS: true},
		{Name: "n2", Protocol: "ss", Server: "b.example.com", Port: 8388, Method: "aes-256-gcm", Password: "pw"},
		{Name: "n3", Protocol: "trojan", Server: "c.example.com", Port: 443, Password: "pw", TLS: true},
		{
			Name: "n4", Protocol: "vless", Server: "d.example.com", Port: 443,
			UUID: "uuid4", Network: "tcp", TLS: true, Reality: true,
			SNI: "www.microsoft.com",
			Raw: map[string]interface{}{
				"flow": "xtls-rprx-vision",
				"fp":   "chrome",
				"pbk":  "PUBKEY",
				"sid":  "SHORTID",
			},
		},
		{Name: "n5", Protocol: "hysteria2", Server: "e.example.com", Port: 8443, Password: "pw", TLS: true},
	}
	var warned []string
	opts := ClashProducerOpts{
		OnWarning: func(n *ParsedNode, reason string) {
			warned = append(warned, n.Name+":"+reason)
		},
	}
	out, err := ProduceClashYAML(nodes, opts)
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	s := string(out)
	// All 5 proxies must be present — modern Clash forks (mihomo / Verge / Stash)
	// support reality, so n4 is no longer filtered.
	for _, n := range []string{"n1", "n2", "n3", "n4", "n5"} {
		if !strings.Contains(s, "name: "+n) {
			t.Errorf("missing %s in output:\n%s", n, s)
		}
	}
	for _, field := range []string{"reality-opts", "public-key", "short-id", "flow", "client-fingerprint", "servername"} {
		if !strings.Contains(s, field) {
			t.Errorf("reality node missing %q in output:\n%s", field, s)
		}
	}
	if len(warned) != 0 {
		t.Errorf("did not expect warnings, got %v", warned)
	}
	// Validate field ordering: name should appear before type before server
	// before port within each proxy block.
	firstName := strings.Index(s, "name: n1")
	firstType := strings.Index(s, "type: vmess")
	firstServer := strings.Index(s, "server: a.example.com")
	firstPort := strings.Index(s, "port: 443")
	if firstName < 0 || firstType < 0 || firstServer < 0 || firstPort < 0 {
		t.Fatalf("missing canonical fields in output:\n%s", s)
	}
	if !(firstName < firstType && firstType < firstServer && firstServer < firstPort) {
		t.Errorf("field order wrong: name=%d type=%d server=%d port=%d", firstName, firstType, firstServer, firstPort)
	}
}

func TestProduceClashYAML_EmptyNodes(t *testing.T) {
	out, err := ProduceClashYAML(nil, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	if !strings.Contains(string(out), "proxies:") {
		t.Errorf("output should still contain proxies: key, got:\n%s", out)
	}
}

func TestProduceClashYAML_NilNodeSkipped(t *testing.T) {
	nodes := []*ParsedNode{
		nil,
		{Name: "ok", Protocol: "ss", Server: "x", Port: 80, Method: "aes-256-gcm", Password: "pw"},
	}
	out, err := ProduceClashYAML(nodes, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	if !strings.Contains(string(out), "name: ok") {
		t.Errorf("expected ok in output")
	}
}

func TestProduceClashYAML_RawPreserved(t *testing.T) {
	nodes := []*ParsedNode{
		{
			Name:     "raw-test",
			Protocol: "vmess",
			Server:   "x.example.com",
			Port:     443,
			UUID:     "uuid",
			Network:  "tcp",
			Raw: map[string]interface{}{
				"aid":         "0",
				"client-fingerprint": "chrome",
			},
		},
	}
	out, err := ProduceClashYAML(nodes, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceClashYAML: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "_raw:") {
		t.Errorf("_raw section missing")
	}
	if !strings.Contains(s, "client-fingerprint: chrome") {
		t.Errorf("raw value not preserved:\n%s", s)
	}
}
