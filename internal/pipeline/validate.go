package pipeline

import (
	"fmt"
)

// Validate runs structural checks on an AST without instantiating operators.
// Returns nil when the AST is well-formed.
//
// Auto-fix: if the user did not add an output step, Validate appends a
// default "output: clash" rather than rejecting the pipeline. Most users
// only care about filter/sort/rename — forcing them to manually drag an
// output step is UX friction for zero benefit.
func Validate(ast *AST) error {
	if ast == nil {
		return fmt.Errorf("%w: nil AST", ErrSchemaMismatch)
	}
	if ast.APIVersion == "" {
		ast.APIVersion = APIVersion
	}
	if ast.APIVersion != APIVersion {
		return fmt.Errorf("%w: got %q, want %q",
			ErrSchemaMismatch, ast.APIVersion, APIVersion)
	}
	if len(ast.Operators) > MaxOperators {
		return fmt.Errorf("%w: %d > %d", ErrTooManyOperators, len(ast.Operators), MaxOperators)
	}

	// Separate non-output steps from output steps.
	var nonOutput []OperatorSpec
	var outputStep *OperatorSpec
	for _, op := range ast.Operators {
		if op.Kind == "" {
			return fmt.Errorf("%w: step kind required", ErrUnknownOperator)
		}
		if _, ok := factoryFor(op.Kind); !ok {
			return fmt.Errorf("%w: kind %q", ErrUnknownOperator, op.Kind)
		}
		if op.Kind == KindOutput {
			cp := op
			outputStep = &cp // keep the last one if multiple
		} else {
			nonOutput = append(nonOutput, op)
		}
	}

	// Auto-append default output if user didn't add one.
	if outputStep == nil {
		outputStep = &OperatorSpec{
			Kind: KindOutput,
			Args: []byte(`{"format":"clash"}`),
		}
	}

	// Reassemble: non-output steps in original order + output at end.
	nonOutput = append(nonOutput, *outputStep)
	ast.Operators = nonOutput
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
