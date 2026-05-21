package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"shiguang-vps/internal/substore"
)

func TestOutput_RequiresFormat(t *testing.T) {
	_, err := New(KindOutput, []byte(`{}`))
	if !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func TestOutput_RejectsUnknownFormat(t *testing.T) {
	raw, _ := json.Marshal(OutputArgs{Format: "surge"})
	_, err := New(KindOutput, raw)
	if !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func TestOutput_AcceptsClashClashMetaRaw(t *testing.T) {
	for _, f := range []string{"clash", "clash_meta", "raw"} {
		raw, _ := json.Marshal(OutputArgs{Format: f})
		if _, err := New(KindOutput, raw); err != nil {
			t.Fatalf("format %s: %v", f, err)
		}
	}
}

func TestOutput_PassthroughWhenNoMax(t *testing.T) {
	op := mustOp(t, KindOutput, OutputArgs{Format: "clash"})
	in := []*substore.ParsedNode{{Name: "a"}, {Name: "b"}}
	out, err := op.Apply(context.Background(), in)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want passthrough 2, got %d", len(out))
	}
}

func TestOutput_TruncatesAtMaxNodes(t *testing.T) {
	op := mustOp(t, KindOutput, OutputArgs{Format: "clash", MaxNodes: 2})
	in := []*substore.ParsedNode{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	out, _ := op.Apply(context.Background(), in)
	if len(out) != 2 {
		t.Fatalf("want 2, got %d", len(out))
	}
}

func TestOutput_RejectsNegativeMax(t *testing.T) {
	raw, _ := json.Marshal(OutputArgs{Format: "clash", MaxNodes: -1})
	if _, err := New(KindOutput, raw); !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func TestOutput_FormatAccessor(t *testing.T) {
	op := mustOp(t, KindOutput, OutputArgs{Format: "raw"})
	oo, ok := op.(*outputOp)
	if !ok {
		t.Fatalf("expected *outputOp, got %T", op)
	}
	if oo.Format() != "raw" {
		t.Fatalf("want raw, got %s", oo.Format())
	}
}
