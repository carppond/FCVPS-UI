package pipeline

import (
	"fmt"
)

// Validate runs structural checks on an AST without instantiating operators.
// Returns nil when the AST is well-formed; otherwise returns one of the
// sentinel errors (ErrSchemaMismatch / ErrUnknownOperator /
// ErrTooManyOperators / ErrOutputRequired) wrapped with diagnostic context.
//
// Validate is cheap (no regex compile, no clone) so the engine calls it as
// part of every Run / Preview to keep the AST honest even when the caller
// supplied a freshly parsed JSON.
func Validate(ast *AST) error {
	if ast == nil {
		return fmt.Errorf("%w: nil AST", ErrSchemaMismatch)
	}
	if ast.APIVersion != APIVersion {
		return fmt.Errorf("%w: got %q, want %q",
			ErrSchemaMismatch, ast.APIVersion, APIVersion)
	}
	if len(ast.Operators) == 0 {
		return fmt.Errorf("%w: pipeline must have at least one operator (output)", ErrOutputRequired)
	}
	if len(ast.Operators) > MaxOperators {
		return fmt.Errorf("%w: %d > %d", ErrTooManyOperators, len(ast.Operators), MaxOperators)
	}

	outputs := 0
	for i, op := range ast.Operators {
		if op.Kind == "" {
			return fmt.Errorf("%w: step %d kind required", ErrUnknownOperator, i)
		}
		if _, ok := factoryFor(op.Kind); !ok {
			return fmt.Errorf("%w: step %d kind %q", ErrUnknownOperator, i, op.Kind)
		}
		if op.Kind == KindOutput {
			outputs++
			if i != len(ast.Operators)-1 {
				return fmt.Errorf("%w: output step must be last (found at %d)", ErrOutputRequired, i)
			}
		}
	}
	if outputs == 0 {
		return fmt.Errorf("%w: missing terminal output step", ErrOutputRequired)
	}
	if outputs > 1 {
		return fmt.Errorf("%w: exactly one output step allowed", ErrOutputRequired)
	}
	return nil
}

// factoryFor is a registry probe that does not allocate. Returns the factory
// + ok.
func factoryFor(kind string) (OperatorFactory, bool) {
	registryMu.RLock()
	f, ok := registry[kind]
	registryMu.RUnlock()
	return f, ok
}
