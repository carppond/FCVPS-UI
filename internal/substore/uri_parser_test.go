package substore

import (
	"errors"
	"strings"
	"testing"
)

func TestParseURI_Dispatch(t *testing.T) {
	cases := []struct {
		uri  string
		want string
	}{
		{buildVmess(`{"v":"2","ps":"a","add":"x.example.com","port":"443","id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}`), "vmess"},
		{"vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com:443?security=tls", "vless"},
		{"ss://" + b64("aes-256-gcm:pw") + "@example.com:8388", "ss"},
		{"trojan://pw@example.com:443", "trojan"},
		{"hysteria://example.com:8443?auth=pw", "hysteria"},
		{"hy2://pw@example.com:8443", "hysteria2"},
		{"hysteria2://pw@example.com:8443", "hysteria2"},
		{"tuic://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:pw@example.com:443", "tuic"},
		{"anytls://pw@example.com:8443", "anytls"},
		{"socks5://example.com:1080", "socks5"},
		{"naive+https://user:pw@example.com:443", "naive"},
		{"wireguard://PK@example.com:51820?public_key=PUB", "wireguard"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			n, err := ParseURI(c.uri)
			if err != nil {
				t.Fatalf("ParseURI %q: %v", c.uri, err)
			}
			if n.Protocol != c.want {
				t.Errorf("protocol: want %s, got %s", c.want, n.Protocol)
			}
		})
	}
}

func TestParseURI_UnsupportedScheme(t *testing.T) {
	_, err := ParseURI("http://example.com")
	if !errors.Is(err, ErrUnsupportedScheme) {
		t.Fatalf("want ErrUnsupportedScheme, got %v", err)
	}
}

func TestParseURI_Empty(t *testing.T) {
	_, err := ParseURI("   ")
	if !errors.Is(err, ErrInvalidURI) {
		t.Fatalf("want ErrInvalidURI, got %v", err)
	}
}

func TestParseBulk(t *testing.T) {
	var b strings.Builder
	// 50 mixed lines: 40 valid + 5 invalid + 5 comments / empty
	valid := []string{
		buildVmess(`{"v":"2","ps":"vmess1","add":"a.example.com","port":"443","id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}`),
		"vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@b.example.com:443?security=tls",
		"ss://" + b64("aes-256-gcm:pw") + "@c.example.com:8388",
		"trojan://pw@d.example.com:443",
		"hysteria2://pw@e.example.com:8443",
		"tuic://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:pw@f.example.com:443",
		"anytls://pw@g.example.com:8443",
		"socks5://h.example.com:1080",
	}
	for i := 0; i < 5; i++ {
		for _, v := range valid {
			b.WriteString(v)
			b.WriteString("\n")
		}
	}
	// invalid lines
	for i := 0; i < 5; i++ {
		b.WriteString("not-a-valid-uri-line\n")
	}
	// comment / blank lines
	b.WriteString("# comment line\n")
	b.WriteString("// another comment\n")
	b.WriteString("\n")
	b.WriteString("   \n")
	b.WriteString("// trailing comment\n")

	nodes, errs := ParseBulk(b.String())
	wantNodes := 8 * 5
	if len(nodes) != wantNodes {
		t.Errorf("want %d nodes, got %d", wantNodes, len(nodes))
	}
	if len(errs) != 5 {
		t.Errorf("want 5 errors, got %d", len(errs))
	}
}

func TestParseBulk_Empty(t *testing.T) {
	nodes, errs := ParseBulk("")
	if len(nodes) != 0 || len(errs) != 0 {
		t.Errorf("empty input should yield nothing, got nodes=%d errs=%d", len(nodes), len(errs))
	}
}

func TestBulkError_Unwrap(t *testing.T) {
	be := &BulkError{Line: 1, URI: "x", Err: ErrInvalidURI}
	if !errors.Is(be, ErrInvalidURI) {
		t.Fatal("Unwrap should expose ErrInvalidURI")
	}
}
