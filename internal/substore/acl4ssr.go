package substore

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ParseACL4SSR parses a subconverter-style INI document (commonly known as
// ACL4SSR rules) into a structured ACLConfig.
//
// Recognised sections:
//
//   - [general] / [General]: key=value entries kept as-is.
//   - [proxy] / [Proxy]: each non-blank line is a node URI; lines that fail
//     to parse are skipped silently.
//   - [proxy group] / [Proxy Group]: name=type,member1,member2,url,interval,tolerance
//   - [rule] / [Rule]: a Clash rule line, e.g. DOMAIN-SUFFIX,example.com,Proxy
//   - [override]: key=value entries kept as-is.
//
// Unknown sections are ignored. Comments (`#`, `;`, `//`) and blank lines are
// skipped. The parser is intentionally lenient – production subconverter
// configs in the wild frequently mix tabs / spaces / odd casing.
func ParseACL4SSR(text string) (*ACLConfig, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("acl4ssr: %w: empty input", ErrInvalidURI)
	}
	cfg := &ACLConfig{
		General:  map[string]string{},
		Override: map[string]string{},
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
	section := ""
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		switch section {
		case "general":
			parseKV(line, cfg.General)
		case "override":
			parseKV(line, cfg.Override)
		case "proxy":
			if node, err := ParseURI(line); err == nil {
				cfg.Proxy = append(cfg.Proxy, *node)
			}
		case "proxy group":
			if g, err := parseACLGroup(line); err == nil {
				cfg.Groups = append(cfg.Groups, g)
			}
		case "rule", "ruleset":
			if r, err := parseACLRule(line); err == nil {
				cfg.Rules = append(cfg.Rules, r)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("acl4ssr: scan: %w", err)
	}
	return cfg, nil
}

func stripComment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, ";") || strings.HasPrefix(s, "//") {
		return ""
	}
	return s
}

func parseKV(line string, into map[string]string) {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return
	}
	k := strings.TrimSpace(line[:eq])
	v := strings.TrimSpace(line[eq+1:])
	if k == "" {
		return
	}
	into[k] = v
}

func parseACLGroup(line string) (ACLProxyGroup, error) {
	// Layout: name=type,member1,member2,...[,url][,interval][,tolerance]
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return ACLProxyGroup{}, fmt.Errorf("acl4ssr: %w: group line missing '='", ErrInvalidURI)
	}
	name := strings.TrimSpace(line[:eq])
	rhs := strings.TrimSpace(line[eq+1:])
	parts := splitCSV(rhs)
	if len(parts) < 2 {
		return ACLProxyGroup{}, fmt.Errorf("acl4ssr: %w: group needs at least type + 1 member", ErrInvalidURI)
	}
	g := ACLProxyGroup{Name: name, Type: strings.ToLower(parts[0])}
	// Walk remaining: a trailing URL + int + int triple is the canonical
	// url-test layout. We push everything else as members.
	tail := parts[1:]
	for _, p := range tail {
		// Heuristic: a member never contains '://'; URLs do.
		switch {
		case strings.Contains(p, "://"):
			g.URL = p
		case isAllDigits(p):
			if g.Interval == 0 {
				g.Interval, _ = strconv.Atoi(p)
			} else if g.Tolerance == 0 {
				g.Tolerance, _ = strconv.Atoi(p)
			}
		default:
			g.Members = append(g.Members, p)
		}
	}
	return g, nil
}

func parseACLRule(line string) (ACLRule, error) {
	parts := splitCSV(line)
	if len(parts) < 3 {
		// Special MATCH / FINAL: TYPE,TARGET only
		if len(parts) == 2 {
			t := strings.ToUpper(strings.TrimSpace(parts[0]))
			if t == "MATCH" || t == "FINAL" {
				return ACLRule{Type: t, Target: strings.TrimSpace(parts[1])}, nil
			}
		}
		return ACLRule{}, fmt.Errorf("acl4ssr: %w: rule needs at least 3 fields", ErrInvalidURI)
	}
	r := ACLRule{
		Type:   strings.ToUpper(strings.TrimSpace(parts[0])),
		Value:  strings.TrimSpace(parts[1]),
		Target: strings.TrimSpace(parts[2]),
	}
	if len(parts) >= 4 && strings.EqualFold(parts[3], "no-resolve") {
		r.NoResolve = true
	}
	return r, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
