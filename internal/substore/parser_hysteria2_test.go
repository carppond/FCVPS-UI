package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseHysteria2_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/hysteria2.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines")
	}
	for _, ln := range lines {
		node, err := ParseHysteria2(ln)
		if err != nil {
			t.Fatalf("parse %q: %v", ln, err)
		}
		if node.Protocol != "hysteria2" {
			t.Errorf("protocol normalised? %s", node.Protocol)
		}
		if node.Password == "" {
			t.Error("empty password")
		}
	}
}

func TestParseHysteria2_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "hy2 alias",
			uri:  "hy2://pw@example.com:8443?sni=example.com",
		},
		{
			name: "with port forward",
			uri:  "hysteria2://pw@example.com:8443?sni=example.com&obfs=salamander#h2",
		},
		{
			name:    "missing password",
			uri:     "hy2://@example.com:8443",
			wantErr: true,
		},
		{
			name:    "missing port",
			uri:     "hysteria2://pw@example.com",
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
			_, err := ParseHysteria2(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
