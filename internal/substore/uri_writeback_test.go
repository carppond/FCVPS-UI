package substore

import (
	"strings"
	"testing"
)

func TestURIWriteback_PreservesExtras(t *testing.T) {
	cases := []struct {
		name string
		n    *ParsedNode
		want []string
	}{
		{"ss-plugin", &ParsedNode{
			Protocol: "ss", Server: "s", Port: 1, Method: "aes-128-gcm", Password: "p",
			Raw: map[string]interface{}{"plugin": "obfs-local;obfs=tls;obfs-host=a.com"},
		}, []string{"plugin=obfs-local"}},
		{
			"hy2-obfs", &ParsedNode{
				Protocol: "hysteria2", Server: "s", Port: 1, Password: "p",
				Raw: map[string]interface{}{"obfs": "salamander", "obfs-password": "x", "insecure": "1"},
			},
			[]string{"obfs=salamander", "obfs-password=x", "insecure=1"},
		},
		{
			"tuic-udp", &ParsedNode{
				Protocol: "tuic", Server: "s", Port: 1, UUID: "u", Password: "p",
				Raw: map[string]interface{}{"udp_relay_mode": "quic", "allow_insecure": "1"},
			},
			[]string{"udp_relay_mode=quic", "allow_insecure=1"},
		},
		{
			"trojan-fp", &ParsedNode{
				Protocol: "trojan", Server: "s", Port: 1, Password: "p", TLS: true,
				Raw: map[string]interface{}{"fp": "chrome", "allowInsecure": "1"},
			},
			[]string{"fp=chrome", "allowInsecure=1"},
		},
		{"vless-grpc", &ParsedNode{
			Protocol: "vless", Server: "s", Port: 1, UUID: "u", Network: "grpc", TLS: true,
			Raw: map[string]interface{}{"serviceName": "gun"},
		}, []string{"serviceName=gun", "encryption=none"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uri, ok := nodeToURI(c.n)
			if !ok {
				t.Fatalf("nodeToURI returned !ok")
			}
			for _, w := range c.want {
				if !strings.Contains(uri, w) {
					t.Errorf("missing %q:\n%s", w, uri)
				}
			}
		})
	}
}
