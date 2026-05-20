package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseTrojan_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/trojan.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	node, err := ParseTrojan(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("ParseTrojan: %v", err)
	}
	if node.Password != "samplepassword" {
		t.Errorf("password: %s", node.Password)
	}
	if node.Server != "example.com" || node.Port != 443 {
		t.Errorf("server/port: %s:%d", node.Server, node.Port)
	}
	if !node.TLS {
		t.Error("default TLS should be true for trojan")
	}
	if len(node.ALPN) != 2 {
		t.Errorf("alpn: %v", node.ALPN)
	}
}

func TestParseTrojan_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
		check   func(*testing.T, *ParsedNode)
	}{
		{
			name: "minimal",
			uri:  "trojan://pw@1.2.3.4:443",
			check: func(t *testing.T, n *ParsedNode) {
				if !n.TLS {
					t.Error("tls default true")
				}
			},
		},
		{
			name: "ws transport",
			uri:  "trojan://pw@example.com:443?type=ws&path=/wsx&host=cdn.example.com",
			check: func(t *testing.T, n *ParsedNode) {
				if n.Network != "ws" || n.Path != "/wsx" {
					t.Errorf("ws/path: %s/%s", n.Network, n.Path)
				}
			},
		},
		{
			name: "tls disabled",
			uri:  "trojan://pw@example.com:443?security=none",
			check: func(t *testing.T, n *ParsedNode) {
				if n.TLS {
					t.Error("tls should be off")
				}
			},
		},
		{
			name:    "missing password",
			uri:     "trojan://@example.com:443",
			wantErr: true,
		},
		{
			name:    "missing port",
			uri:     "trojan://pw@example.com",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			uri:     "vmess://abc",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, err := ParseTrojan(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, n)
			}
		})
	}
}
