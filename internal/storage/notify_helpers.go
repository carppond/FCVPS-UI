package storage

import (
	"encoding/json"
)

// encodeStringArray marshals a slice into a JSON array string; empty / nil
// inputs become "[]". Helper shared by the notify repos so the on-disk
// representation of event_types stays canonical regardless of caller.
func encodeStringArray(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// decodeStringArray parses the persisted JSON-array string back into a slice.
// An empty string or "[]" yields an empty slice (never nil) so JSON responses
// are stable.
func decodeStringArray(raw string) []string {
	if raw == "" || raw == "[]" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

// hasString returns true when needle is present in haystack.
func hasString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
