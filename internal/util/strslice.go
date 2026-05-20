package util

// Contains reports whether target is present in slice. Comparable element type.
func Contains[T comparable](slice []T, target T) bool {
	for _, v := range slice {
		if v == target {
			return true
		}
	}
	return false
}

// Dedupe returns slice with duplicates removed, preserving first-seen order.
// The returned slice is a new allocation; input is not mutated.
func Dedupe[T comparable](slice []T) []T {
	if len(slice) == 0 {
		return []T{}
	}
	seen := make(map[T]struct{}, len(slice))
	out := make([]T, 0, len(slice))
	for _, v := range slice {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// Filter returns the elements of slice for which keep(elem) reports true.
// Always returns a non-nil slice (possibly empty).
func Filter[T any](slice []T, keep func(T) bool) []T {
	out := make([]T, 0, len(slice))
	for _, v := range slice {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

// Map applies fn to every element of slice and returns the transformed slice.
// Output length equals input length.
func Map[T any, U any](slice []T, fn func(T) U) []U {
	out := make([]U, len(slice))
	for i, v := range slice {
		out[i] = fn(v)
	}
	return out
}
