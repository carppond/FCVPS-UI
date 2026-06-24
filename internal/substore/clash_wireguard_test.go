package substore

import (
	"strings"
	"testing"
)

func TestClashWireguard_ProperFields(t *testing.T) {
	n := &ParsedNode{
		Protocol: "wireguard", Name: "wg", Server: "s", Port: 51820,
		Password: "PRIVKEY",
		Raw: map[string]interface{}{
			"public-key": "PUBKEY", "address": "10.0.0.2/32",
			"preshared-key": "PSK", "dns": "1.1.1.1", "mtu": "1408",
		},
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: []*ParsedNode{n}}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	s := string(out)
	for _, want := range []string{"private-key: PRIVKEY", "public-key: PUBKEY", "ip: 10.0.0.2/32", "pre-shared-key: PSK", "mtu: 1408"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "password: PRIVKEY") {
		t.Errorf("private key still emitted as password:\n%s", s)
	}
}
