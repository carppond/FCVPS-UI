package substore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Sentinel errors surfaced by the URI parsers. Wrap them with fmt.Errorf when
// adding context so callers can keep using errors.Is for branching.
var (
	// ErrInvalidURI is returned when the input string is empty or does not
	// match the expected URI shape for a given parser.
	ErrInvalidURI = errors.New("substore: invalid URI")
	// ErrUnsupportedScheme is returned by the route dispatcher when the URI
	// scheme is not one of the 12 supported protocols.
	ErrUnsupportedScheme = errors.New("substore: unsupported scheme")
	// ErrInvalidBase64 is returned when a base64 segment fails to decode.
	ErrInvalidBase64 = errors.New("substore: invalid base64 payload")
	// ErrMissingField is returned when a required field is absent from the URI.
	ErrMissingField = errors.New("substore: missing required field")
	// ErrInvalidPort is returned when port is missing or out of range.
	ErrInvalidPort = errors.New("substore: invalid port")
)

// decodeBase64Loose decodes a base64 / base64url string, tolerating missing
// padding (a common defect in real-world subscription strings). Callers may
// feed either flavour interchangeably.
func decodeBase64Loose(s string) ([]byte, error) {
	if s == "" {
		return nil, fmt.Errorf("%w: empty", ErrInvalidBase64)
	}
	// Some sources URL-escape the payload (e.g. %3D for =).
	if unescaped, err := url.QueryUnescape(s); err == nil {
		s = unescaped
	}
	s = strings.TrimSpace(s)
	// Normalise base64url to std alphabet for transparent decoding.
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	out, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// As a last resort try raw / url variants.
		if out2, err2 := base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "=")); err2 == nil {
			return out2, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidBase64, err)
	}
	return out, nil
}

// parsePort parses a port string and ensures it sits in the 1..65535 range.
// Empty input returns ErrInvalidPort so callers do not propagate a zero port.
func parsePort(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("%w: empty", ErrInvalidPort)
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidPort, err)
	}
	if v <= 0 || v > 65535 {
		return 0, fmt.Errorf("%w: out of range %d", ErrInvalidPort, v)
	}
	return v, nil
}

// decodeFragment returns the URL fragment (#name) decoded into a human
// readable string. Returns empty when fragment is absent.
func decodeFragment(u *url.URL) string {
	if u == nil || u.Fragment == "" {
		return u.Fragment
	}
	if decoded, err := url.QueryUnescape(u.Fragment); err == nil {
		return decoded
	}
	return u.Fragment
}

// rawCopy returns a shallow copy of a query string into the Raw map; this is
// the canonical way parsers preserve unsupported fields per PRD M-SUB.3.
func rawCopy(values url.Values, exclude ...string) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}
	skip := make(map[string]struct{}, len(exclude))
	for _, k := range exclude {
		skip[k] = struct{}{}
	}
	out := make(map[string]interface{}, len(values))
	for k, v := range values {
		if _, ok := skip[k]; ok {
			continue
		}
		if len(v) == 1 {
			out[k] = v[0]
		} else {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// splitALPN splits a comma-separated alpn list. Trailing / leading whitespace
// is trimmed and empty entries are dropped.
func splitALPN(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// stripScheme removes the leading "scheme://" prefix from a URI and returns
// the remainder. Returns an error when the prefix is absent.
func stripScheme(uri, scheme string) (string, error) {
	uri = strings.TrimSpace(uri)
	prefix := scheme + "://"
	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("%w: expected scheme %q", ErrInvalidURI, scheme)
	}
	return strings.TrimPrefix(uri, prefix), nil
}
