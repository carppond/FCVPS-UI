package util_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"shiguang-vps/internal/util"
)

const sampleProxy = `port: 443
type: vmess
server: example.com
name: "🚀 香港 01"
uuid: deadbeef-0000-1111-2222-333344445555
network: ws
ws-opts:
  path: "/ws"
  headers:
    Host: cdn.example.com
remark: kept # trailing comment
`

func mustUnmarshal(t *testing.T, raw string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		t.Fatalf("expected single-doc YAML, got kind=%d content=%d", doc.Kind, len(doc.Content))
	}
	return doc.Content[0]
}

func TestReorderProxyNodeKeyOrder(t *testing.T) {
	node := mustUnmarshal(t, sampleProxy)
	out := util.ReorderProxyNode(node)
	keys := mappingKeys(t, out)

	// Required canonical order for the recognised keys present in input.
	want := []string{"name", "type", "server", "port", "uuid", "network", "ws-opts"}
	for i, w := range want {
		if keys[i] != w {
			t.Fatalf("keys[%d] = %q, want %q (full=%v)", i, keys[i], w, keys)
		}
	}
	// remark is unknown to ProxyKeyOrder; it must appear after the canonical
	// list (i.e. last in this fixture).
	if last := keys[len(keys)-1]; last != "remark" {
		t.Fatalf("expected remark last, got %q (full=%v)", last, keys)
	}
}

func TestReorderProxyNodePreservesQuotedScalar(t *testing.T) {
	node := mustUnmarshal(t, sampleProxy)
	out := util.ReorderProxyNode(node)

	nameVal, ok := util.GetMappingValue(out, "name")
	if !ok {
		t.Fatal("name key missing after reorder")
	}
	if nameVal.Style != yaml.DoubleQuotedStyle {
		t.Fatalf("name style = %d, want DoubleQuotedStyle(%d)", nameVal.Style, yaml.DoubleQuotedStyle)
	}
	if nameVal.Value != "🚀 香港 01" {
		t.Fatalf("name value mutated: %q", nameVal.Value)
	}
}

func TestSetMappingValueReplacePreservesStyleAndComment(t *testing.T) {
	node := mustUnmarshal(t, sampleProxy)

	if err := util.SetMappingValue(node, "remark", "still-here"); err != nil {
		t.Fatalf("SetMappingValue: %v", err)
	}
	val, ok := util.GetMappingValue(node, "remark")
	if !ok {
		t.Fatal("remark missing after set")
	}
	if val.Value != "still-here" {
		t.Fatalf("remark value = %q, want still-here", val.Value)
	}
	// LineComment on the old value should be inherited because we passed a
	// raw string (the new node had no comments).
	if !strings.Contains(val.LineComment, "trailing comment") {
		t.Fatalf("LineComment lost: %q", val.LineComment)
	}
}

func TestSetMappingValueAppendsNewKey(t *testing.T) {
	node := mustUnmarshal(t, sampleProxy)
	if err := util.SetMappingValue(node, "udp", true); err != nil {
		t.Fatalf("SetMappingValue: %v", err)
	}
	val, ok := util.GetMappingValue(node, "udp")
	if !ok {
		t.Fatal("udp not appended")
	}
	if val.Value != "true" {
		t.Fatalf("udp value = %q, want true", val.Value)
	}
}

func TestSetMappingValueRejectsNonMapping(t *testing.T) {
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "oops"}
	if err := util.SetMappingValue(scalar, "x", 1); err == nil {
		t.Fatal("expected ErrYAMLNotMapping on scalar input")
	}
}

func TestCloneNodeDeepCopy(t *testing.T) {
	node := mustUnmarshal(t, sampleProxy)
	clone := util.CloneNode(node)

	// Mutate clone.
	if err := util.SetMappingValue(clone, "server", "mutated.example.com"); err != nil {
		t.Fatalf("SetMappingValue on clone: %v", err)
	}
	origServer, _ := util.GetMappingValue(node, "server")
	cloneServer, _ := util.GetMappingValue(clone, "server")
	if origServer.Value == cloneServer.Value {
		t.Fatalf("clone mutation leaked to original (%q)", origServer.Value)
	}
	if origServer.Value != "example.com" {
		t.Fatalf("original server changed to %q", origServer.Value)
	}
}

func TestMarshalIndentRoundtripQuotedKey(t *testing.T) {
	node := mustUnmarshal(t, sampleProxy)
	out, err := util.MarshalIndent(node)
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	rendered := string(out)
	// yaml.v3 escapes non-ASCII inside DoubleQuoted style; assert the style
	// (and inner ASCII text) was retained.
	if !strings.Contains(rendered, `name: "`) {
		t.Fatalf("name lost DoubleQuoted style: %q", rendered)
	}
	if !strings.Contains(rendered, "香港 01") && !strings.Contains(rendered, `香港 01`) {
		t.Fatalf("name value lost: %q", rendered)
	}
	if !strings.Contains(rendered, "trailing comment") {
		t.Fatalf("trailing comment lost: %q", rendered)
	}
}

// mappingKeys collects keys in document order for assertions.
func mappingKeys(t *testing.T, n *yaml.Node) []string {
	t.Helper()
	if n == nil || n.Kind != yaml.MappingNode {
		t.Fatalf("expected mapping node, got kind=%d", n.Kind)
	}
	out := make([]string, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		out = append(out, n.Content[i].Value)
	}
	return out
}
