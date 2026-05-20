package substore

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseSS_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/ss.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	for _, ln := range lines {
		node, err := ParseSS(strings.TrimSpace(ln))
		if err != nil {
			t.Fatalf("parse %q: %v", ln, err)
		}
		if node.Server != "example.com" || node.Port != 8388 {
			t.Errorf("server/port mismatch: %s:%d", node.Server, node.Port)
		}
		if node.Method != "aes-256-gcm" {
			t.Errorf("method: %s", node.Method)
		}
		if node.Password == "" {
			t.Error("password empty")
		}
	}
}

func TestParseSS_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "sip002 with plugin",
			uri:  "ss://" + b64("aes-256-gcm:sample") + "@1.2.3.4:8388/?plugin=obfs-local%3Bobfs%3Dhttp#sip",
		},
		{
			name: "padding-less base64",
			uri:  "ss://" + strings.TrimRight(b64("aes-256-gcm:sample"), "=") + "@1.2.3.4:8388#pad",
		},
		{
			name:    "missing port",
			uri:     "ss://" + b64("aes-256-gcm:sample") + "@1.2.3.4",
			wantErr: true,
		},
		{
			name:    "bad base64",
			uri:     "ss://!!!nope!!!#bad",
			wantErr: true,
		},
		{
			name:    "empty",
			uri:     "ss://",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSS(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseSS_InvalidScheme(t *testing.T) {
	_, err := ParseSS("vless://x")
	if !errors.Is(err, ErrInvalidURI) {
		t.Fatalf("want ErrInvalidURI, got %v", err)
	}
}
