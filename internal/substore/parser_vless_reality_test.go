package substore

import (
	"encoding/base64"
	"strings"
	"testing"
)

const realityURI = "vless://11111111-1111-1111-1111-111111111111@1.2.3.4:31841?" +
	"encryption=none&flow=xtls-rprx-vision&fp=chrome&pbk=PUBKEYDUMMY&" +
	"security=reality&sid=552369&sni=www.microsoft.com&spx=%2FPa7&type=tcp#x"

// TestParseVlessRealityKeepsParams: the parser retains reality params in Raw.
func TestParseVlessRealityKeepsParams(t *testing.T) {
	n, err := ParseURI(realityURI)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, k := range []string{"flow", "fp", "pbk", "sid", "spx"} {
		if v, ok := n.Raw[k]; !ok || v == "" {
			t.Errorf("Raw missing reality param %q (got %v)", k, n.Raw[k])
		}
	}
}

// TestURIListVlessRealityHasParams guards the regression where the uri-list
// (base64 / Shadowrocket / v2rayN) producer rebuilt the vless link WITHOUT the
// reality params, so the node TCP-pinged but failed the reality handshake.
func TestURIListVlessRealityHasParams(t *testing.T) {
	n, err := ParseURI(realityURI)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, _, err := (ProducerFactory{}).Get("shadowrocket").
		Produce(&ClashRenderInput{Nodes: []*ParsedNode{n}}, ClashProducerOpts{})
	if err != nil {
		t.Fatalf("produce: %v", err)
	}
	// uri-list output is the base64-encoded URI list — decode before asserting.
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("decode base64 output: %v", err)
	}
	decoded := string(raw)
	for _, want := range []string{"security=reality", "flow=", "pbk=", "sid=", "sni="} {
		if !strings.Contains(decoded, want) {
			t.Errorf("uri-list vless missing %q\ndecoded: %s", want, decoded)
		}
	}
}
