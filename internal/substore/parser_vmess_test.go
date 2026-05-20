package substore

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseVmess_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/vmess.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	uri := strings.TrimSpace(string(raw))
	node, err := ParseVmess(uri)
	if err != nil {
		t.Fatalf("ParseVmess error: %v", err)
	}
	if node.Server != "example.com" || node.Port != 443 {
		t.Errorf("server/port mismatch: got %s:%d", node.Server, node.Port)
	}
	if node.UUID == "" {
		t.Error("uuid empty")
	}
	if !node.TLS {
		t.Error("tls not detected")
	}
	if node.Network != "ws" {
		t.Errorf("network: want ws, got %s", node.Network)
	}
	if node.Name != "test-node" {
		t.Errorf("name: want test-node, got %s", node.Name)
	}
}

func TestParseVmess_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "minimal",
			uri:  buildVmess(`{"v":"2","ps":"min","add":"1.2.3.4","port":"443","id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}`),
		},
		{
			name: "name with emoji",
			uri:  buildVmess(`{"v":"2","ps":"✨ node ✨","add":"a.example.com","port":"443","id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","net":"ws"}`),
		},
		{
			name:    "missing add",
			uri:     buildVmess(`{"v":"2","ps":"x","port":"443","id":"x"}`),
			wantErr: true,
		},
		{
			name:    "bad base64",
			uri:     "vmess://!!!notbase64!!!",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			uri:     "vless://abc@example.com:443",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseVmess(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseVmess_InvalidPort(t *testing.T) {
	_, err := ParseVmess(buildVmess(`{"add":"x","id":"y","port":"abc"}`))
	if !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("want ErrInvalidPort, got %v", err)
	}
}

// buildVmess produces a vmess:// URI by base64-encoding the supplied JSON.
func buildVmess(jsonBody string) string {
	return "vmess://" + b64(jsonBody)
}
