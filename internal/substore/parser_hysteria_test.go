package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseHysteria_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/hysteria.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	node, err := ParseHysteria(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("ParseHysteria: %v", err)
	}
	if node.Password != "samplepassword" {
		t.Errorf("password: %s", node.Password)
	}
	if node.SNI != "example.com" {
		t.Errorf("sni: %s", node.SNI)
	}
	if len(node.ALPN) != 1 || node.ALPN[0] != "h3" {
		t.Errorf("alpn: %v", node.ALPN)
	}
}

func TestParseHysteria_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "minimal",
			uri:  "hysteria://1.2.3.4:8443?auth=token",
		},
		{
			name: "obfs preserved in raw",
			uri:  "hysteria://example.com:8443?auth=pw&obfs=salamander&obfs-password=secret&peer=example.com#h1",
		},
		{
			name:    "missing host",
			uri:     "hysteria://:8443?auth=pw",
			wantErr: true,
		},
		{
			name:    "missing port",
			uri:     "hysteria://example.com",
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
			_, err := ParseHysteria(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
