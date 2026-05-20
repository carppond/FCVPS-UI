package substore

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func TestParseSSR_FromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/ssr.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	node, err := ParseSSR(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("ParseSSR: %v", err)
	}
	if node.Server != "example.com" || node.Port != 8388 {
		t.Errorf("server/port: %s:%d", node.Server, node.Port)
	}
	if node.Method != "aes-256-cfb" {
		t.Errorf("method: %s", node.Method)
	}
	if node.Password != "samplepassword123" {
		t.Errorf("password: %s", node.Password)
	}
	if node.Name != "ssr-test" {
		t.Errorf("name: %s", node.Name)
	}
}

func TestParseSSR_Cases(t *testing.T) {
	mkBody := func(host, port string) string {
		pwd := strings.TrimRight(base64.RawURLEncoding.EncodeToString([]byte("pw")), "=")
		return base64.StdEncoding.EncodeToString([]byte(host + ":" + port + ":origin:aes-256-cfb:plain:" + pwd))
	}
	tests := []struct {
		name    string
		uri     string
		wantErr bool
	}{
		{
			name: "minimal",
			uri:  "ssr://" + mkBody("1.2.3.4", "8388"),
		},
		{
			name: "with remarks",
			uri: "ssr://" + base64.StdEncoding.EncodeToString([]byte(
				"1.2.3.4:8388:origin:aes-256-cfb:plain:"+strings.TrimRight(base64.RawURLEncoding.EncodeToString([]byte("pw")), "=")+
					"/?remarks="+base64.RawURLEncoding.EncodeToString([]byte("台湾节点 01")))),
		},
		{
			name:    "not enough fields",
			uri:     "ssr://" + base64.StdEncoding.EncodeToString([]byte("a:b:c")),
			wantErr: true,
		},
		{
			name:    "bad base64",
			uri:     "ssr://!!!",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			uri:     "ss://abc",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseSSR(tc.uri)
			if tc.wantErr && err == nil {
				t.Fatalf("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
