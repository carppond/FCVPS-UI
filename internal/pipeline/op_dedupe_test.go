package pipeline

import (
	"context"
	"errors"
	"testing"

	"shiguang-vps/internal/substore"
)

func TestDedupe_DefaultByServerPort(t *testing.T) {
	op := mustOp(t, KindDedupe, DedupeArgs{})
	in := []*substore.ParsedNode{
		{Name: "a", Server: "1.1.1.1", Port: 443},
		{Name: "b", Server: "1.1.1.1", Port: 443}, // dup
		{Name: "c", Server: "1.1.1.1", Port: 444},
		{Name: "d", Server: "2.2.2.2", Port: 443},
	}
	out, _ := op.Apply(context.Background(), in)
	if len(out) != 3 {
		t.Fatalf("want 3, got %d names=%v", len(out), namesOf(out))
	}
	// First-seen kept.
	if out[0].Name != "a" || out[1].Name != "c" || out[2].Name != "d" {
		t.Fatalf("first-seen broken: %v", namesOf(out))
	}
}

func TestDedupe_CustomFields(t *testing.T) {
	op := mustOp(t, KindDedupe, DedupeArgs{Fields: []string{"name"}})
	in := []*substore.ParsedNode{
		{Name: "x", Server: "a"},
		{Name: "x", Server: "b"}, // dup by name even though different server
		{Name: "y", Server: "c"},
	}
	out, _ := op.Apply(context.Background(), in)
	if len(out) != 2 {
		t.Fatalf("want 2, got %d", len(out))
	}
}

func TestDedupe_MultiFieldOrder(t *testing.T) {
	op := mustOp(t, KindDedupe, DedupeArgs{Fields: []string{"server", "protocol", "port"}})
	in := []*substore.ParsedNode{
		{Server: "s1", Protocol: "vmess", Port: 443, Name: "a"},
		{Server: "s1", Protocol: "vless", Port: 443, Name: "b"}, // protocol differs → keep
		{Server: "s1", Protocol: "vmess", Port: 443, Name: "c"}, // exact dup
	}
	out, _ := op.Apply(context.Background(), in)
	if len(out) != 2 {
		t.Fatalf("want 2, got %d names=%v", len(out), namesOf(out))
	}
}

func TestDedupe_RejectsUnknownField(t *testing.T) {
	_, err := New(KindDedupe, mustJSON(DedupeArgs{Fields: []string{"bogus"}}))
	if !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func TestDedupe_PreservesOrder(t *testing.T) {
	op := mustOp(t, KindDedupe, DedupeArgs{})
	in := []*substore.ParsedNode{
		{Name: "n1", Server: "a", Port: 1},
		{Name: "n2", Server: "b", Port: 1},
		{Name: "n3", Server: "a", Port: 1}, // dup
		{Name: "n4", Server: "c", Port: 1},
	}
	out, _ := op.Apply(context.Background(), in)
	if len(out) != 3 || out[0].Name != "n1" || out[1].Name != "n2" || out[2].Name != "n4" {
		t.Fatalf("order broken: %v", namesOf(out))
	}
}
