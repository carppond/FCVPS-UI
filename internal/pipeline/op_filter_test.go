package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"shiguang-vps/internal/substore"
)

func mustFilter(t *testing.T, expr string) Operator {
	t.Helper()
	raw, _ := json.Marshal(FilterArgs{Expr: expr})
	op, err := New(KindFilter, raw)
	if err != nil {
		t.Fatalf("new filter: %v", err)
	}
	return op
}

func TestFilter_ProtocolIn(t *testing.T) {
	nodes := sampleNodes()
	op := mustFilter(t, `protocol in ["vmess","vless"]`)
	out, err := op.Apply(context.Background(), nodes)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2, got %d", len(out))
	}
	for _, n := range out {
		if n.Protocol != "vmess" && n.Protocol != "vless" {
			t.Fatalf("unexpected protocol %s", n.Protocol)
		}
	}
}

func TestFilter_RegionMatches(t *testing.T) {
	nodes := sampleNodes()
	op := mustFilter(t, `region in ["hk","jp"]`)
	out, _ := op.Apply(context.Background(), nodes)
	if len(out) != 2 {
		t.Fatalf("want 2 (HK + JP), got %d names=%v", len(out), namesOf(out))
	}
}

func TestFilter_NameRegex(t *testing.T) {
	nodes := sampleNodes()
	op := mustFilter(t, `name ~= "HK"`)
	out, _ := op.Apply(context.Background(), nodes)
	if len(out) != 1 || out[0].Name != "HK-1" {
		t.Fatalf("want only HK-1, got %v", namesOf(out))
	}
}

func TestFilter_PortInequality(t *testing.T) {
	nodes := sampleNodes()
	op := mustFilter(t, `port > 1000 && port < 9000`)
	out, _ := op.Apply(context.Background(), nodes)
	if len(out) != 2 {
		t.Fatalf("want 2 (8443+1080), got %d %v", len(out), namesOf(out))
	}
}

func TestFilter_AndOr(t *testing.T) {
	nodes := sampleNodes()
	op := mustFilter(t, `(protocol == "vmess" || protocol == "ss") && tls == false`)
	out, _ := op.Apply(context.Background(), nodes)
	if len(out) != 1 || out[0].Protocol != "ss" {
		t.Fatalf("want only SS (no TLS), got %v", namesOf(out))
	}
}

func TestFilter_NotIn(t *testing.T) {
	nodes := sampleNodes()
	op := mustFilter(t, `protocol not in ["ss","ssr"]`)
	out, _ := op.Apply(context.Background(), nodes)
	// Fixtures contain vmess + vless + ss + trojan + ssr → 3 survive.
	if len(out) != 3 {
		t.Fatalf("want 3 (non-ss/ssr), got %d %v", len(out), namesOf(out))
	}
}

func TestFilter_EmptyExpressionKeepsAll(t *testing.T) {
	nodes := sampleNodes()
	op := mustFilter(t, "")
	out, _ := op.Apply(context.Background(), nodes)
	if len(out) != len(nodes) {
		t.Fatalf("expected passthrough, got %d", len(out))
	}
}

func TestFilter_InvalidExpressionFailsAtCompile(t *testing.T) {
	_, err := New(KindFilter, []byte(`{"expr":"name == "}`))
	if !errors.Is(err, ErrInvalidExpression) {
		t.Fatalf("want ErrInvalidExpression, got %v", err)
	}
}

func TestFilter_InvalidRegexInTilde(t *testing.T) {
	op := mustFilter(t, `name ~= "(unterminated"`)
	_, err := op.Apply(context.Background(), sampleNodes())
	if err == nil || !errors.Is(err, ErrInvalidRegex) {
		t.Fatalf("want ErrInvalidRegex, got %v", err)
	}
}

// sampleNodes returns a fixed fixture of 5 nodes covering the common
// branching cases (multiple protocols, regions, ports, TLS on/off).
func sampleNodes() []*substore.ParsedNode {
	return []*substore.ParsedNode{
		{Name: "HK-1", Protocol: "vmess", Server: "hk.example.com", Port: 8443, TLS: true, Tag: "🇭🇰 HongKong"},
		{Name: "JP-Tokyo", Protocol: "vless", Server: "jp.example.com", Port: 443, TLS: true, Tag: "JP"},
		{Name: "US-1", Protocol: "ss", Server: "us.example.com", Port: 1080, TLS: false, Tag: "US"},
		{Name: "SG-1", Protocol: "trojan", Server: "sg.example.com", Port: 443, TLS: true, Tag: "Singapore"},
		{Name: "Test", Protocol: "ssr", Server: "tt.example.com", Port: 12345, TLS: false, Tag: ""},
	}
}

func namesOf(nodes []*substore.ParsedNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out
}
