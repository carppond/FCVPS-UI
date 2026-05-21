package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"shiguang-vps/internal/substore"
)

func mustOp(t *testing.T, kind string, args any) Operator {
	t.Helper()
	raw, _ := json.Marshal(args)
	op, err := New(kind, raw)
	if err != nil {
		t.Fatalf("new %s: %v", kind, err)
	}
	return op
}

func TestMap_SetField(t *testing.T) {
	op := mustOp(t, KindMap, MapArgs{Field: "network", Value: "ws"})
	in := []*substore.ParsedNode{
		{Name: "a", Network: "tcp"},
		{Name: "b", Network: ""},
	}
	out, err := op.Apply(context.Background(), in)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	for _, n := range out {
		if n.Network != "ws" {
			t.Fatalf("want network=ws, got %q", n.Network)
		}
	}
	// Inputs unchanged.
	if in[0].Network != "tcp" {
		t.Fatalf("map mutated input: %q", in[0].Network)
	}
}

func TestMap_Template(t *testing.T) {
	op := mustOp(t, KindMap, MapArgs{Field: "name", Value: "{{.Protocol}}-{{.Index}}-{{.Name}}"})
	in := []*substore.ParsedNode{
		{Name: "alpha", Protocol: "vmess"},
		{Name: "beta", Protocol: "vless"},
	}
	out, _ := op.Apply(context.Background(), in)
	if out[0].Name != "vmess-0-alpha" || out[1].Name != "vless-1-beta" {
		t.Fatalf("unexpected names: %v", []string{out[0].Name, out[1].Name})
	}
}

func TestMap_PortNumeric(t *testing.T) {
	op := mustOp(t, KindMap, MapArgs{Field: "port", Value: "9443"})
	in := []*substore.ParsedNode{{Port: 80}}
	out, _ := op.Apply(context.Background(), in)
	if out[0].Port != 9443 {
		t.Fatalf("want 9443, got %d", out[0].Port)
	}
}

func TestMap_PortRejectsNonNumeric(t *testing.T) {
	op := mustOp(t, KindMap, MapArgs{Field: "port", Value: "notnum"})
	_, err := op.Apply(context.Background(), []*substore.ParsedNode{{Port: 80}})
	if err == nil {
		t.Fatalf("expected error on non-numeric port")
	}
}

func TestMap_RejectsUnknownField(t *testing.T) {
	_, err := New(KindMap, mustJSON(MapArgs{Field: "bogus", Value: "x"}))
	if !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
