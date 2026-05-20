package substore

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseVless_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/vless.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	uri := strings.TrimSpace(string(raw))
	node, err := ParseVless(uri)
	if err != nil {
		t.Fatalf("ParseVless: %v", err)
	}
	if node.UUID != "12345678-1234-1234-1234-123456789abc" {
		t.Errorf("uuid mismatch: %s", node.UUID)
	}
	if node.Server != "example.com" || node.Port != 443 {
		t.Errorf("server/port: %s:%d", node.Server, node.Port)
	}
	if !node.TLS || node.Network != "ws" {
		t.Errorf("tls/network: %v / %s", node.TLS, node.Network)
	}
	if node.SNI != "cdn.example.com" {
		t.Errorf("sni: %s", node.SNI)
	}
	if len(node.ALPN) != 2 {
		t.Errorf("alpn: %v", node.ALPN)
	}
	if node.Name != "vless-test" {
		t.Errorf("name: %s", node.Name)
	}
}

func TestParseVless_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
		check   func(*testing.T, *ParsedNode)
	}{
		{
			name: "minimal",
			uri:  "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@1.2.3.4:443",
			check: func(t *testing.T, n *ParsedNode) {
				if n.Network != "tcp" {
					t.Errorf("default net should be tcp, got %s", n.Network)
				}
			},
		},
		{
			name: "reality flag",
			uri:  "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com:443?security=reality&type=tcp&sni=example.com&pbk=fakekey&sid=00#reality-node",
			check: func(t *testing.T, n *ParsedNode) {
				if !n.Reality {
					t.Error("Reality flag not set")
				}
				if !n.TLS {
					t.Error("TLS should be true for reality")
				}
			},
		},
		{
			name: "emoji name",
			uri:  "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com:443?security=tls#%F0%9F%87%BA%F0%9F%87%B8%20US",
			check: func(t *testing.T, n *ParsedNode) {
				if !strings.Contains(n.Name, "US") {
					t.Errorf("decoded name missing US: %q", n.Name)
				}
			},
		},
		{
			name:    "missing uuid",
			uri:     "vless://@example.com:443",
			wantErr: true,
		},
		{
			name:    "missing port",
			uri:     "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com",
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
			n, err := ParseVless(tc.uri)
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

func TestParseVless_InvalidPort(t *testing.T) {
	_, err := ParseVless("vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com:99999")
	if !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("want ErrInvalidPort, got %v", err)
	}
}
