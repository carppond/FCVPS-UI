package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseTUIC_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/tuic.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	node, err := ParseTUIC(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("ParseTUIC: %v", err)
	}
	if node.UUID != "12345678-1234-1234-1234-123456789abc" {
		t.Errorf("uuid: %s", node.UUID)
	}
	if node.Password != "samplepassword" {
		t.Errorf("password: %s", node.Password)
	}
	if node.Raw["congestion-control"] != "bbr" {
		t.Errorf("congestion: %v", node.Raw["congestion-control"])
	}
}

func TestParseTUIC_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "minimal v4",
			uri:  "tuic://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@1.2.3.4:443",
		},
		{
			name: "v5 with password",
			uri:  "tuic://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:pw@example.com:443?sni=example.com",
		},
		{
			name:    "missing uuid",
			uri:     "tuic://:pw@example.com:443",
			wantErr: true,
		},
		{
			name:    "missing port",
			uri:     "tuic://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:pw@example.com",
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
			_, err := ParseTUIC(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
