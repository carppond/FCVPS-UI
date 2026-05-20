package substore

import (
	"bufio"
	"fmt"
	"strings"
)

// ParseURI dispatches the input URI to the protocol-specific parser based on
// its scheme. Returns ErrUnsupportedScheme for unrecognised schemes. Empty /
// whitespace-only input returns ErrInvalidURI.
func ParseURI(uri string) (*ParsedNode, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil, fmt.Errorf("%w: empty", ErrInvalidURI)
	}
	idx := strings.Index(uri, "://")
	if idx < 0 {
		return nil, fmt.Errorf("%w: missing scheme", ErrInvalidURI)
	}
	scheme := strings.ToLower(uri[:idx])
	switch scheme {
	case "vmess":
		return ParseVmess(uri)
	case "vless":
		return ParseVless(uri)
	case "ss":
		return ParseSS(uri)
	case "ssr":
		return ParseSSR(uri)
	case "trojan":
		return ParseTrojan(uri)
	case "hysteria":
		return ParseHysteria(uri)
	case "hysteria2", "hy2":
		return ParseHysteria2(uri)
	case "tuic":
		return ParseTUIC(uri)
	case "wireguard":
		return ParseWireguard(uri)
	case "anytls":
		return ParseAnyTLS(uri)
	case "socks5":
		return ParseSocks5(uri)
	}
	if strings.HasPrefix(uri, "naive+") {
		return ParseNaive(uri)
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupportedScheme, scheme)
}

// BulkError pairs a parse error with the line number on which it occurred.
// Line numbers are 1-based to match editor conventions.
type BulkError struct {
	Line int
	URI  string
	Err  error
}

// Error implements the error interface.
func (b *BulkError) Error() string {
	return fmt.Sprintf("line %d (%s): %v", b.Line, truncate(b.URI, 40), b.Err)
}

// Unwrap exposes the underlying parse error.
func (b *BulkError) Unwrap() error { return b.Err }

// ParseBulk parses a multi-line subscription text. Empty lines and shell-
// style comments (`#`, `//`) are skipped. Errors on individual lines do not
// abort processing; the function returns every parsed node alongside the
// list of per-line errors.
func ParseBulk(text string) ([]*ParsedNode, []*BulkError) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	// Subscription URIs can be longer than the default 64 KiB token; bump.
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	var (
		nodes []*ParsedNode
		errs  []*BulkError
	)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") {
			continue
		}
		n, err := ParseURI(raw)
		if err != nil {
			errs = append(errs, &BulkError{Line: line, URI: raw, Err: err})
			continue
		}
		nodes = append(nodes, n)
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, &BulkError{Line: line, Err: fmt.Errorf("scan: %w", err)})
	}
	return nodes, errs
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
