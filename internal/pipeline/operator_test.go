package pipeline

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestRegisteredKindsHasSixOperators(t *testing.T) {
	kinds := RegisteredKinds()
	want := map[string]bool{
		KindFilter: true, KindMap: true, KindSort: true,
		KindDedupe: true, KindRegexRename: true, KindOutput: true,
	}
	for _, k := range kinds {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Fatalf("missing kinds in registry: %v", want)
	}
}

func TestNewUnknownKindReturnsSentinel(t *testing.T) {
	_, err := New("nope", nil)
	if !errors.Is(err, ErrUnknownOperator) {
		t.Fatalf("want ErrUnknownOperator, got %v", err)
	}
}

func TestNewSurfacesFactoryErrors(t *testing.T) {
	// output requires format; nil args triggers the factory validation.
	_, err := New(KindOutput, []byte(`{}`))
	if !errors.Is(err, ErrInvalidArgs) {
		t.Fatalf("want ErrInvalidArgs, got %v", err)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on duplicate Register")
		}
	}()
	Register(KindFilter, func(_ json.RawMessage) (Operator, error) { return nil, nil })
}
