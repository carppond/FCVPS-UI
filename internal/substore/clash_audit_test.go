package substore

import (
	"strings"
	"testing"
)

// Regression for "Clash 不稳定 / ping 时有时无,Shadowrocket 正常": the Clash
// output must set udp: true (mihomo leaves UDP off otherwise → QUIC/HTTP-3
// breaks) and must not leave consumed reality/xtls keys under _raw clutter.
func TestClashRealityNodeHasUDPAndCleanRaw(t *testing.T) {
	uri := "vless://d58c8f55-3726-4545-8884-4dfda0899df2@104.194.71.23:31841?flow=xtls-rprx-vision&fp=chrome&pbk=PUBKEY&security=reality&sid=2d&sni=www.microsoft.com&spx=%2Fx&type=tcp#reality-node"
	r, err := ParseVless(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: []*ParsedNode{r}}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "udp: true") {
		t.Errorf("reality node missing udp: true:\n%s", s)
	}
	// reality params emitted properly...
	for _, want := range []string{"tls: true", "reality-opts:", "flow: xtls-rprx-vision", "client-fingerprint: chrome"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	// ...and NOT duplicated as _raw clutter (fp/pbk/sid/spx already consumed).
	if strings.Contains(s, "_raw:") {
		t.Errorf("consumed reality keys should not appear under _raw:\n%s", s)
	}
}
