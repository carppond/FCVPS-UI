package substore

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func sampleNodes() []*ParsedNode {
	return []*ParsedNode{
		{Name: "vmess-1", Protocol: "vmess", Server: "a.example.com", Port: 443,
			UUID: "00000000-0000-0000-0000-000000000001", Method: "auto",
			Network: "ws", TLS: true, SNI: "a.example.com",
			Path: "/v2", Host: "edge.example.com"},
		{Name: "ss-1", Protocol: "ss", Server: "b.example.com", Port: 8388,
			Method: "aes-256-gcm", Password: "pw"},
		{Name: "trojan-1", Protocol: "trojan", Server: "c.example.com", Port: 443,
			Password: "tj-pw", TLS: true, SNI: "c.example.com"},
		{Name: "hy2-1", Protocol: "hysteria2", Server: "d.example.com", Port: 8443,
			Password: "hy-pw", TLS: true},
		{Name: "wg-1", Protocol: "wireguard", Server: "e.example.com", Port: 51820,
			Password: "priv", Raw: map[string]interface{}{"public-key": "pub", "address": "10.0.0.2/32"}},
	}
}

func TestProducerFactoryRoutesToClashByDefault(t *testing.T) {
	factory := NewProducerFactory()
	for _, target := range []string{"", "clash", "clashmeta", "mihomo", "stash", "unknown-target"} {
		p := factory.Get(target)
		if _, ok := p.(clashProducer); !ok {
			t.Errorf("target %q: expected clashProducer, got %T", target, p)
		}
	}
}

func TestProducerFactoryRoutesSingbox(t *testing.T) {
	p := NewProducerFactory().Get("singbox")
	if _, ok := p.(singboxProducer); !ok {
		t.Errorf("singbox routing: got %T", p)
	}
}

func TestProducerFactoryRoutesSurgeAndURIList(t *testing.T) {
	if _, ok := NewProducerFactory().Get("surge").(surgeProducer); !ok {
		t.Errorf("surge routing failed")
	}
	for _, target := range []string{"v2ray", "shadowrocket", "qx", "loon", "uri"} {
		if _, ok := NewProducerFactory().Get(target).(uriListProducer); !ok {
			t.Errorf("%s routing failed", target)
		}
	}
}

func TestProduceSingboxJSON_Shape(t *testing.T) {
	body, err := ProduceSingboxJSON(sampleNodes(), ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceSingboxJSON: %v", err)
	}
	var doc struct {
		Outbounds []map[string]interface{} `json:"outbounds"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, body)
	}
	if len(doc.Outbounds) < 4 {
		t.Errorf("expected >=4 outbounds, got %d", len(doc.Outbounds))
	}
	// vmess entry must use server_port + uuid + alter_id.
	found := false
	for _, o := range doc.Outbounds {
		if o["type"] == "vmess" {
			found = true
			if _, ok := o["server_port"]; !ok {
				t.Errorf("vmess missing server_port: %v", o)
			}
			if _, ok := o["uuid"]; !ok {
				t.Errorf("vmess missing uuid: %v", o)
			}
		}
	}
	if !found {
		t.Errorf("no vmess outbound found")
	}
}

func TestProduceURIList_Base64AndDecode(t *testing.T) {
	body, err := ProduceURIList(sampleNodes(), ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceURIList: %v", err)
	}
	dec, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		t.Fatalf("body is not valid base64: %v", err)
	}
	s := string(dec)
	for _, prefix := range []string{"vmess://", "ss://", "trojan://", "hysteria2://"} {
		if !strings.Contains(s, prefix) {
			t.Errorf("expected %s URI in output:\n%s", prefix, s)
		}
	}
}

func TestProduceSurgeConf_Shape(t *testing.T) {
	body, err := ProduceSurgeConf(sampleNodes(), ClashProducerOpts{})
	if err != nil {
		t.Fatalf("ProduceSurgeConf: %v", err)
	}
	s := string(body)
	if !strings.HasPrefix(s, "[Proxy]\n") {
		t.Errorf("missing [Proxy] header:\n%s", s)
	}
	if !strings.Contains(s, "vmess-1 = vmess, a.example.com, 443") {
		t.Errorf("vmess line missing:\n%s", s)
	}
	if !strings.Contains(s, "ss-1 = ss, b.example.com, 8388") {
		t.Errorf("ss line missing:\n%s", s)
	}
	if !strings.Contains(s, "trojan-1 = trojan, c.example.com, 443") {
		t.Errorf("trojan line missing:\n%s", s)
	}
	// Hysteria2 / wireguard not supported by Surge — must be commented out.
	if !strings.Contains(s, "# unsupported:") {
		t.Errorf("expected unsupported comment for hy2/wg:\n%s", s)
	}
}

func TestProducerInterfaceContentTypes(t *testing.T) {
	factory := NewProducerFactory()
	cases := []struct {
		target      string
		contentType string
	}{
		{"clash", "text/yaml; charset=utf-8"},
		{"singbox", "application/json; charset=utf-8"},
		{"surge", "text/plain; charset=utf-8"},
		{"v2ray", "text/plain; charset=utf-8"},
	}
	for _, c := range cases {
		_, ct, err := factory.Get(c.target).Produce(&ClashRenderInput{Nodes: sampleNodes()}, ClashProducerOpts{})
		if err != nil {
			t.Fatalf("%s Produce: %v", c.target, err)
		}
		if ct != c.contentType {
			t.Errorf("%s content-type: want %q got %q", c.target, c.contentType, ct)
		}
	}
}
