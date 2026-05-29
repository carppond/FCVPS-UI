package firewall

import (
	"regexp"
	"strconv"
	"strings"
)

// Rule is a single firewall allow-rule surfaced to the UI. Spec is the exact
// ufw "To" token (e.g. "8081/tcp", "22") used for deletion; Port is the parsed
// numeric port (0 when the target is a range or named app-profile we don't
// manage in v1). Process / PID are filled in by enriching with the live
// listener table (ss); they are not part of the firewall rule itself.
type Rule struct {
	Spec      string `json:"spec"`
	Port      int    `json:"port"`
	Proto     string `json:"proto"` // "tcp" | "udp" | "" (both)
	Process   string `json:"process,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Protected bool   `json:"protected"`
}

// Listener is a process bound to a local port, parsed from `ss`.
type Listener struct {
	Process string
	PID     int
}

// ruleLine matches an allow-rule row in `ufw status` (plain or numbered).
// It deliberately requires the target to be a single non-space token directly
// followed by an ALLOW action, which skips IPv6 duplicate rows ("22/tcp (v6)
// ALLOW ...") and multi-word app profiles ("Nginx Full ALLOW ...") — neither
// of which v1 manages. Examples it matches:
//
//	22/tcp                     ALLOW       Anywhere
//	[ 1] 8081                  ALLOW IN    Anywhere
var ruleLine = regexp.MustCompile(`^\s*(?:\[\s*\d+\]\s*)?(\S+)\s+(ALLOW|DENY|REJECT|LIMIT)\b`)

// simplePortSpec validates a deletable rule spec: a bare port or port/proto.
// Named profiles and ranges fail this and are not deletable through the UI.
var simplePortSpec = regexp.MustCompile(`^\d{1,5}(/(tcp|udp))?$`)

// ssUsers extracts the first process name + pid from an ss "Process" column,
// e.g. users:(("nginx",pid=1234,fd=6)) → ("nginx", 1234).
var ssUsers = regexp.MustCompile(`\(\("([^"]+)",pid=(\d+)`)

// ParseUFWStatus parses the output of `ufw status` into the active flag and
// the list of ALLOW rules. Only ALLOW (incoming) rules are surfaced — the
// feature is about opening ports. IPv6 duplicate rows and app-profile rows are
// skipped. Rules are de-duplicated by (port, proto, spec).
func ParseUFWStatus(output string) (active bool, rules []Rule) {
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "Status:") {
			// Exact match — "inactive" contains "active" as a substring.
			active = strings.TrimSpace(strings.TrimPrefix(trimmed, "Status:")) == "active"
			continue
		}
		m := ruleLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		target, action := m[1], m[2]
		if action != "ALLOW" {
			continue // v1 surfaces only allow rules
		}
		port, proto := parseTarget(target)
		key := target + "|" + proto
		if seen[key] {
			continue
		}
		seen[key] = true
		rules = append(rules, Rule{Spec: target, Port: port, Proto: proto})
	}
	return active, rules
}

// parseTarget splits a ufw target token into a numeric port + proto. Returns
// port 0 for non-simple targets (ranges, named profiles) so the UI can show
// but not delete them.
func parseTarget(token string) (port int, proto string) {
	spec := token
	if i := strings.IndexByte(spec, '/'); i >= 0 {
		proto = spec[i+1:]
		spec = spec[:i]
	}
	if proto != "tcp" && proto != "udp" {
		proto = "" // unknown / both
	}
	if n, err := strconv.Atoi(spec); err == nil && n >= 1 && n <= 65535 {
		port = n
	}
	return port, proto
}

// IsSimplePortSpec reports whether spec is a bare port or port/proto that the
// UI is allowed to delete. Guards the delete path against named profiles.
func IsSimplePortSpec(spec string) bool {
	return simplePortSpec.MatchString(spec)
}

// ParseSSListeners parses `ss -ltnpH` / `ss -lunpH` output into a port→listener
// map. proto labels which transport the rows belong to (informational; the
// map key is the numeric port). Lines without a parseable port or process are
// skipped.
func ParseSSListeners(output string) map[int]Listener {
	out := make(map[int]Listener)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		// Local Address:Port is column 4 (state recv-q send-q local peer ...).
		local := fields[3]
		colon := strings.LastIndexByte(local, ':')
		if colon < 0 {
			continue
		}
		port, err := strconv.Atoi(local[colon+1:])
		if err != nil || port < 1 || port > 65535 {
			continue
		}
		if _, dup := out[port]; dup {
			continue // keep the first listener seen for a port
		}
		l := Listener{}
		if mm := ssUsers.FindStringSubmatch(line); mm != nil {
			l.Process = mm[1]
			l.PID, _ = strconv.Atoi(mm[2])
		}
		out[port] = l
	}
	return out
}
