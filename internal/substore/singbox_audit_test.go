package substore

import (
	"encoding/json"
	"strings"
	"testing"
)

func sbOut(t *testing.T, n *ParsedNode) string {
	t.Helper()
	b, err := ProduceSingboxJSON([]*ParsedNode{n}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	return string(b)
}

func TestSingbox_RealityNotDropped(t *testing.T) {
	n := &ParsedNode{
		Protocol: "vless", Name: "r", Server: "s", Port: 1, UUID: "u",
		Network: "tcp", TLS: true, Reality: true, SNI: "x",
		Raw: map[string]interface{}{"pbk": "PBK", "sid": "ab", "flow": "xtls-rprx-vision", "fp": "chrome"},
	}
	s := sbOut(t, n)
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	obs := doc["outbounds"].([]interface{})
	if len(obs) != 1 {
		t.Fatalf("reality node dropped! outbounds=%d", len(obs))
	}
	for _, w := range []string{`"reality"`, `"public_key": "PBK"`, `"short_id": "ab"`, `"flow": "xtls-rprx-vision"`, `"utls"`} {
		if !strings.Contains(s, w) {
			t.Errorf("missing %q:\n%s", w, s)
		}
	}
}

func TestSingbox_SSPluginAndHy2Obfs(t *testing.T) {
	ss := sbOut(t, &ParsedNode{
		Protocol: "ss", Name: "s", Server: "s", Port: 1, Method: "aes-128-gcm", Password: "p",
		Raw: map[string]interface{}{"plugin": "obfs-local;obfs=tls;obfs-host=a.com"},
	})
	if !strings.Contains(ss, `"plugin": "obfs-local"`) || !strings.Contains(ss, `"plugin_opts": "obfs=tls;obfs-host=a.com"`) {
		t.Errorf("ss plugin not split:\n%s", ss)
	}
	hy := sbOut(t, &ParsedNode{
		Protocol: "hysteria2", Name: "h", Server: "s", Port: 1, Password: "p",
		Raw: map[string]interface{}{"obfs": "salamander", "obfs-password": "x"},
	})
	if !strings.Contains(hy, `"type": "salamander"`) {
		t.Errorf("hy2 obfs missing:\n%s", hy)
	}
}
