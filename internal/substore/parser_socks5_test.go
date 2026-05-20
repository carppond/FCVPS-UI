package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseSocks5_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/socks5.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines")
	}
	for _, ln := range lines {
		node, err := ParseSocks5(ln)
		if err != nil {
			t.Fatalf("parse %q: %v", ln, err)
		}
		if node.Port != 1080 {
			t.Errorf("port: %d", node.Port)
		}
	}
}

func TestParseSocks5_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
		check   func(*testing.T, *ParsedNode)
	}{
		{
			name: "with auth",
			uri:  "socks5://user:pass@example.com:1080",
			check: func(t *testing.T, n *ParsedNode) {
				if n.UUID != "user" || n.Password != "pass" {
					t.Errorf("user/pass: %s/%s", n.UUID, n.Password)
				}
			},
		},
		{
			name: "no auth",
			uri:  "socks5://example.com:1080",
			check: func(t *testing.T, n *ParsedNode) {
				if n.UUID != "" || n.Password != "" {
					t.Errorf("should be empty creds")
				}
			},
		},
		{
			name:    "missing port",
			uri:     "socks5://example.com",
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
			n, err := ParseSocks5(tc.uri)
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
