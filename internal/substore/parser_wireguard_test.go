package substore

import (
	"os"
	"strings"
	"testing"
)

func TestParseWireguard_FromTestdata_URI(t *testing.T) {
	raw, err := os.ReadFile("testdata/wireguard.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	node, err := ParseWireguard(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("ParseWireguard: %v", err)
	}
	if node.Server != "example.com" || node.Port != 51820 {
		t.Errorf("server/port: %s:%d", node.Server, node.Port)
	}
	if node.Password == "" {
		t.Error("private key empty")
	}
	if node.Raw["public-key"] == "" {
		t.Error("public key missing")
	}
}

func TestParseWireguard_FromTestdata_INI(t *testing.T) {
	raw, err := os.ReadFile("testdata/wireguard.conf")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	node, err := ParseWireguard(string(raw))
	if err != nil {
		t.Fatalf("ParseWireguard (ini): %v", err)
	}
	if node.Server != "example.com" || node.Port != 51820 {
		t.Errorf("server/port: %s:%d", node.Server, node.Port)
	}
	if node.Raw["allowed-ips"] != "0.0.0.0/0" {
		t.Errorf("allowed-ips: %v", node.Raw["allowed-ips"])
	}
}

func TestParseWireguard_Cases(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "uri minimal",
			uri:  "wireguard://PK@1.2.3.4:51820?public_key=PUB",
		},
		{
			name:    "missing private key",
			uri:     "wireguard://@1.2.3.4:51820?public_key=PUB",
			wantErr: true,
		},
		{
			name:    "ini missing endpoint",
			uri:     "[Interface]\nPrivateKey=X\n[Peer]\nPublicKey=Y\n",
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
			_, err := ParseWireguard(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
