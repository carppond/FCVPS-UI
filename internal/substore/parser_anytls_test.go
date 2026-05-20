package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseAnyTLS_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/anytls.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	node, err := ParseAnyTLS(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("ParseAnyTLS: %v", err)
	}
	if node.Password != "samplepassword" {
		t.Errorf("password: %s", node.Password)
	}
	if !node.TLS {
		t.Error("tls should be true")
	}
	if node.SNI != "example.com" {
		t.Errorf("sni: %s", node.SNI)
	}
}

func TestParseAnyTLS_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "minimal",
			uri:  "anytls://pw@1.2.3.4:8443",
		},
		{
			name: "with sni",
			uri:  "anytls://pw@example.com:8443?sni=example.com&alpn=h2,http/1.1#at",
		},
		{
			name:    "missing password",
			uri:     "anytls://@example.com:8443",
			wantErr: true,
		},
		{
			name:    "missing port",
			uri:     "anytls://pw@example.com",
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
			_, err := ParseAnyTLS(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
