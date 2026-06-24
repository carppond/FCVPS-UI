package substore

import (
	"strings"
	"testing"
)

func surgeLine(t *testing.T, n *ParsedNode) string {
	t.Helper()
	b, err := ProduceSurgeConf([]*ParsedNode{n}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	return string(b)
}

func TestSurge_SkipCertVerifyAndObfs(t *testing.T) {
	vm := surgeLine(t, &ParsedNode{
		Protocol: "vmess", Name: "v", Server: "s", Port: 1, UUID: "u", TLS: true,
		Raw: map[string]interface{}{"skip-cert-verify": true},
	})
	if !strings.Contains(vm, "skip-cert-verify=true") {
		t.Errorf("vmess missing skip-cert-verify:\n%s", vm)
	}
	tj := surgeLine(t, &ParsedNode{
		Protocol: "trojan", Name: "t", Server: "s", Port: 1, Password: "p", TLS: true,
		Raw: map[string]interface{}{"allowInsecure": "1"},
	})
	if !strings.Contains(tj, "skip-cert-verify=true") {
		t.Errorf("trojan missing skip-cert-verify:\n%s", tj)
	}
	ss := surgeLine(t, &ParsedNode{
		Protocol: "ss", Name: "s", Server: "s", Port: 1, Method: "aes-128-gcm", Password: "p",
		Raw: map[string]interface{}{"plugin": "obfs-local;obfs=tls;obfs-host=a.com"},
	})
	if !strings.Contains(ss, "obfs=tls") || !strings.Contains(ss, "obfs-host=a.com") {
		t.Errorf("ss obfs not rendered:\n%s", ss)
	}
	// v2ray-plugin ss → unsupported (skipped, not broken plain ss)
	v2 := surgeLine(t, &ParsedNode{
		Protocol: "ss", Name: "z", Server: "s", Port: 1, Method: "aes-128-gcm", Password: "p",
		Raw: map[string]interface{}{"plugin": "v2ray-plugin;mode=websocket"},
	})
	if !strings.Contains(v2, "# unsupported") {
		t.Errorf("v2ray-plugin ss should be skipped:\n%s", v2)
	}
}
