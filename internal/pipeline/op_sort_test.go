package pipeline

import (
	"context"
	"errors"
	"testing"

	"shiguang-vps/internal/substore"
)

func TestSort_ByName(t *testing.T) {
	op := mustOp(t, KindSort, SortArgs{Key: "name", Order: "asc"})
	in := []*substore.ParsedNode{{Name: "c"}, {Name: "a"}, {Name: "b"}}
	out, _ := op.Apply(context.Background(), in)
	got := []string{out[0].Name, out[1].Name, out[2].Name}
	want := []string{"a", "b", "c"}
	for i, n := range want {
		if got[i] != n {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

func TestSort_ByPortDesc(t *testing.T) {
	op := mustOp(t, KindSort, SortArgs{Key: "port", Order: "desc"})
	in := []*substore.ParsedNode{{Port: 443}, {Port: 80}, {Port: 8080}}
	out, _ := op.Apply(context.Background(), in)
	if out[0].Port != 8080 || out[1].Port != 443 || out[2].Port != 80 {
		t.Fatalf("desc port sort failed: %v %v %v", out[0].Port, out[1].Port, out[2].Port)
	}
}

func TestSort_ByServer(t *testing.T) {
	op := mustOp(t, KindSort, SortArgs{Key: "server", Order: "asc"})
	in := []*substore.ParsedNode{
		{Name: "z", Server: "b"},
		{Name: "y", Server: "a"},
	}
	out, _ := op.Apply(context.Background(), in)
	if out[0].Server != "a" {
		t.Fatalf("want server=a first, got %q", out[0].Server)
	}
}

func TestSort_ByProtocol(t *testing.T) {
	op := mustOp(t, KindSort, SortArgs{Key: "protocol", Order: "asc"})
	in := []*substore.ParsedNode{{Protocol: "vmess"}, {Protocol: "ss"}, {Protocol: "trojan"}}
	out, _ := op.Apply(context.Background(), in)
	want := []string{"ss", "trojan", "vmess"}
	for i, w := range want {
		if out[i].Protocol != w {
			t.Fatalf("protocol asc: want %v, got %v", want, []string{out[0].Protocol, out[1].Protocol, out[2].Protocol})
		}
	}
}

func TestSort_ByTag(t *testing.T) {
	op := mustOp(t, KindSort, SortArgs{Key: "tag", Order: "asc"})
	in := []*substore.ParsedNode{{Tag: "JP"}, {Tag: "HK"}, {Tag: "SG"}}
	out, _ := op.Apply(context.Background(), in)
	if out[0].Tag != "HK" || out[1].Tag != "JP" || out[2].Tag != "SG" {
		t.Fatalf("tag asc failed: %v %v %v", out[0].Tag, out[1].Tag, out[2].Tag)
	}
}

func TestSort_StableForEqualKeys(t *testing.T) {
	op := mustOp(t, KindSort, SortArgs{Key: "protocol", Order: "asc"})
	in := []*substore.ParsedNode{
		{Name: "a1", Protocol: "vmess"},
		{Name: "a2", Protocol: "vmess"},
		{Name: "a3", Protocol: "vmess"},
	}
	out, _ := op.Apply(context.Background(), in)
	if out[0].Name != "a1" || out[1].Name != "a2" || out[2].Name != "a3" {
		t.Fatalf("sort not stable: %v", []string{out[0].Name, out[1].Name, out[2].Name})
	}
}

func TestSort_InvalidArgs(t *testing.T) {
	if _, err := New(KindSort, mustJSON(SortArgs{Key: "", Order: "asc"})); !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs on empty key")
	}
	if _, err := New(KindSort, mustJSON(SortArgs{Key: "nope", Order: "asc"})); !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs on unknown key")
	}
	if _, err := New(KindSort, mustJSON(SortArgs{Key: "name", Order: "bogus"})); !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs on bad order")
	}
}
