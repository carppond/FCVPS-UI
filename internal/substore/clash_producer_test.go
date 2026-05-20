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
		{Name: "n4", Protocol: "vless", Server: "d.example.com", Port: 443, UUID: "uuid4", TLS: true, Reality: true}, // filtered
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
	// Expect 4 proxies (n4 dropped).
	for _, n := range []string{"n1", "n2", "n3", "n5"} {
		if !strings.Contains(s, "name: "+n) {
			t.Errorf("missing %s in output:\n%s", n, s)
		}
	}
	if strings.Contains(s, "name: n4") {
		t.Errorf("n4 (vless+reality) should be filtered")
	}
	if len(warned) != 1 || !strings.Contains(warned[0], "n4") {
		t.Errorf("expected one warning about n4, got %v", warned)
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
