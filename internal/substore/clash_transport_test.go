package substore

import (
	"strings"
	"testing"
)

// Reproduces: a Clash-format vmess+ws node loses its ws-opts (path/Host),
// servername, skip-cert-verify after parse → re-render.
func TestClashWSNodeTransportPreserved(t *testing.T) {
	clashSub := `proxies:
  - name: hk-ws
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    network: ws
    tls: true
    servername: example.com
    skip-cert-verify: true
    ws-opts:
      path: /mypath
      headers:
        Host: example.com
`
	parsed, err := parseClashYAML([]byte(clashSub))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: parsed}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	s := string(out)
	t.Logf("\n=== RENDERED OUTPUT ===\n%s", s)
	for _, want := range []string{"path: /mypath", "ws-opts", "servername: example.com", "skip-cert-verify: true"} {
		if !strings.Contains(s, want) {
			t.Errorf("LOST in output: %q", want)
		}
	}
}
