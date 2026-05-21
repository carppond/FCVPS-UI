package pipeline

import (
	"context"
	"errors"
	"testing"

	"shiguang-vps/internal/substore"
)

func TestRegexRename_BasicReplacement(t *testing.T) {
	op := mustOp(t, KindRegexRename, RegexRenameArgs{
		Pattern: `\s+`, Replacement: "-",
	})
	out, _ := op.Apply(context.Background(), []*substore.ParsedNode{
		{Name: "Hong Kong 1"},
	})
	if out[0].Name != "Hong-Kong-1" {
		t.Fatalf("want Hong-Kong-1, got %q", out[0].Name)
	}
}

func TestRegexRename_CaptureGroup(t *testing.T) {
	op := mustOp(t, KindRegexRename, RegexRenameArgs{
		Pattern:     `^([A-Z]+)-(\d+)$`,
		Replacement: "$2-$1",
	})
	out, _ := op.Apply(context.Background(), []*substore.ParsedNode{
		{Name: "HK-1"},
		{Name: "JP-42"},
	})
	if out[0].Name != "1-HK" {
		t.Fatalf("want 1-HK, got %q", out[0].Name)
	}
	if out[1].Name != "42-JP" {
		t.Fatalf("want 42-JP, got %q", out[1].Name)
	}
}

func TestRegexRename_NoMatchPassthrough(t *testing.T) {
	op := mustOp(t, KindRegexRename, RegexRenameArgs{
		Pattern: `xyz`, Replacement: "*",
	})
	out, _ := op.Apply(context.Background(), []*substore.ParsedNode{
		{Name: "Hello"},
	})
	if out[0].Name != "Hello" {
		t.Fatalf("want passthrough, got %q", out[0].Name)
	}
}

func TestRegexRename_InvalidRegexCompileFails(t *testing.T) {
	_, err := New(KindRegexRename, mustJSON(RegexRenameArgs{
		Pattern: `(unterminated`, Replacement: "*",
	}))
	if !errors.Is(err, ErrInvalidRegex) {
		t.Fatalf("want ErrInvalidRegex, got %v", err)
	}
}

func TestRegexRename_TargetTagField(t *testing.T) {
	op := mustOp(t, KindRegexRename, RegexRenameArgs{
		Pattern: `^\s+`, Replacement: "", Field: "tag",
	})
	out, _ := op.Apply(context.Background(), []*substore.ParsedNode{
		{Tag: "  spaced"},
	})
	if out[0].Tag != "spaced" {
		t.Fatalf("want trimmed, got %q", out[0].Tag)
	}
}

func TestRegexRename_RejectsUnsupportedField(t *testing.T) {
	_, err := New(KindRegexRename, mustJSON(RegexRenameArgs{
		Pattern: "x", Replacement: "y", Field: "server",
	}))
	if !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func TestRegexRename_EmptyPatternRejected(t *testing.T) {
	_, err := New(KindRegexRename, mustJSON(RegexRenameArgs{Pattern: "", Replacement: "x"}))
	if !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func TestRegexRename_DoesNotMutateInput(t *testing.T) {
	op := mustOp(t, KindRegexRename, RegexRenameArgs{Pattern: "x", Replacement: "y"})
	in := []*substore.ParsedNode{{Name: "xyz"}}
	out, _ := op.Apply(context.Background(), in)
	if in[0].Name != "xyz" {
		t.Fatalf("input mutated: %q", in[0].Name)
	}
	if out[0].Name != "yyz" {
		t.Fatalf("output wrong: %q", out[0].Name)
	}
}
