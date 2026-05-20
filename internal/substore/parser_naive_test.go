package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseNaive_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/naive.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	node, err := ParseNaive(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("ParseNaive: %v", err)
	}
	if node.UUID != "user" || node.Password != "samplepass" {
		t.Errorf("creds: %s/%s", node.UUID, node.Password)
	}
	if node.Raw["transport"] != "https" {
		t.Errorf("transport: %v", node.Raw["transport"])
	}
}

func TestParseNaive_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "https transport",
			uri:  "naive+https://user:pw@example.com:443#n1",
		},
		{
			name: "quic transport",
			uri:  "naive+quic://user:pw@example.com:443#n2",
		},
		{
			name:    "missing user",
			uri:     "naive+https://@example.com:443",
			wantErr: true,
		},
		{
			name:    "missing port",
			uri:     "naive+https://user:pw@example.com",
			wantErr: true,
		},
		{
			name:    "no naive prefix",
			uri:     "https://user:pw@example.com:443",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseNaive(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
