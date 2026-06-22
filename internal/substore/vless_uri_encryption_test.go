package substore

import (
	"strings"
	"testing"
)

// VLESS share links must carry encryption=none, or Shadowrocket / v2rayN fail
// to import / connect (regression for the user-reported "no network").
func TestVlessToURI_HasEncryptionNone(t *testing.T) {
	n := &ParsedNode{
		Protocol: "vless", Name: "r", Server: "1.2.3.4", Port: 443,
		UUID: "u", Network: "tcp", TLS: true, Reality: true,
		SNI: "www.microsoft.com",
		Raw: map[string]interface{}{
			"flow": "xtls-rprx-vision", "fp": "chrome",
			"pbk": "PUBKEY", "sid": "2d", "spx": "/x",
		},
	}
	uri := vlessToURI(n)
	for _, want := range []string{
		"encryption=none", "security=reality", "flow=xtls-rprx-vision",
		"pbk=PUBKEY", "sid=2d", "sni=www.microsoft.com",
	} {
		if !strings.Contains(uri, want) {
			t.Errorf("vless URI missing %q:\n%s", want, uri)
		}
	}
}
