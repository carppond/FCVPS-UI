package substore

import (
	"strings"
	"testing"
)

func TestClashVmessAlterIdAndGrpc(t *testing.T) {
	// vmess from URI (aid in Raw)
	vm := &ParsedNode{
		Protocol: "vmess", Name: "v", Server: "s", Port: 1, UUID: "u",
		Method: "auto", Network: "ws", TLS: true, Path: "/p", Host: "h",
		Raw: map[string]interface{}{"aid": "0"},
	}
	// vless grpc from URI (serviceName in Raw)
	vg := &ParsedNode{
		Protocol: "vless", Name: "g", Server: "s2", Port: 2, UUID: "u2",
		Network: "grpc", TLS: true, SNI: "x",
		Raw: map[string]interface{}{"serviceName": "gun"},
	}
	out, err := ProduceClashYAML(&ClashRenderInput{Nodes: []*ParsedNode{vm, vg}}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "alterId: 0") {
		t.Errorf("vmess missing alterId:\n%s", s)
	}
	if !strings.Contains(s, "grpc-opts:") || !strings.Contains(s, "grpc-service-name: gun") {
		t.Errorf("vless grpc missing grpc-opts:\n%s", s)
	}
}
