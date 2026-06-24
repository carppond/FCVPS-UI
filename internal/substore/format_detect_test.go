package substore

import (
	"encoding/base64"
	"testing"
)

// base64-wrapped Clash YAML must parse (previously → whole sub 0 nodes).
func TestParseBody_Base64WrappedClashYAML(t *testing.T) {
	clash := `proxies:
  - {name: a, type: vmess, server: 1.2.3.4, port: 443, uuid: u, alterId: 0, cipher: auto}
`
	b64 := base64.StdEncoding.EncodeToString([]byte(clash))
	nodes, errs := parseSubscriptionBody([]byte(b64))
	if len(nodes) != 1 {
		t.Fatalf("base64-clash should yield 1 node, got %d (errs=%v)", len(nodes), errs)
	}
	if nodes[0].Protocol != "vmess" || nodes[0].Server != "1.2.3.4" {
		t.Errorf("bad node: %+v", nodes[0])
	}
}

// Direct plaintext clash + plaintext URI list still work.
func TestParseBody_PlainStillWorks(t *testing.T) {
	if n, _ := parseSubscriptionBody([]byte("proxies:\n  - {name: a, type: vmess, server: s, port: 1, uuid: u, cipher: auto}\n")); len(n) != 1 {
		t.Errorf("plain clash broke: %d", len(n))
	}
	if n, _ := parseSubscriptionBody([]byte("ss://YWVzLTEyOC1nY206cA@s:1#x")); len(n) != 1 {
		t.Errorf("plain uri broke: %d", len(n))
	}
}
