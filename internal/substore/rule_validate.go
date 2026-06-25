package substore

import (
	"strings"

	"shiguang-vps/internal/types"
)

// twoFieldRuleTypes are the rule types whose valid form is just "TYPE,策略组"
// (no match argument). Everything else needs at least "TYPE,值,策略组".
var twoFieldRuleTypes = map[string]bool{"MATCH": true, "FINAL": true}

// knownRuleTypes is the mihomo / Clash rule-type vocabulary, used both to know
// the field-count requirement and to offer "did you mean …" typo hints.
var knownRuleTypes = map[string]bool{
	"DOMAIN": true, "DOMAIN-SUFFIX": true, "DOMAIN-KEYWORD": true,
	"DOMAIN-REGEX": true, "DOMAIN-WILDCARD": true, "GEOSITE": true,
	"IP-CIDR": true, "IP-CIDR6": true, "IP-SUFFIX": true, "IP-ASN": true,
	"GEOIP": true, "SRC-GEOIP": true, "SRC-IP-ASN": true, "SRC-IP-CIDR": true,
	"SRC-IP-SUFFIX": true, "DST-PORT": true, "SRC-PORT": true, "IN-PORT": true,
	"IN-TYPE": true, "IN-USER": true, "IN-NAME": true, "PROCESS-NAME": true,
	"PROCESS-PATH": true, "PROCESS-NAME-REGEX": true, "PROCESS-PATH-REGEX": true,
	"UID": true, "NETWORK": true, "DSCP": true, "RULE-SET": true,
	"AND": true, "OR": true, "NOT": true, "SUB-RULE": true,
	"MATCH": true, "FINAL": true,
}

// ValidateRuleContent checks each line of a type=rules body and returns the
// structurally-broken ones (empty slice = all good). Blank and comment lines
// are skipped (they are legal and simply ignored). The rule editor calls this
// to reject a save with a helpful per-line message; the render path reuses the
// same classifier to drop bad lines so a subscription never fails to load.
func ValidateRuleContent(content string) []types.RuleLineIssue {
	var issues []types.RuleLineIssue
	for i, raw := range strings.Split(content, "\n") {
		text, ok := cleanRuleLine(raw)
		if !ok {
			continue
		}
		if reason, suggestion, _ := classifyRuleLine(text); reason != "" {
			issues = append(issues, types.RuleLineIssue{
				Line: i + 1, Text: text, Reason: reason, Suggestion: suggestion,
			})
		}
	}
	return issues
}

// cleanRuleLine trims a raw line, strips a leading "- " list marker, a trailing
// inline " #…" comment and surrounding quotes. ok=false marks blank / comment
// lines, which are valid and simply skipped.
func cleanRuleLine(raw string) (string, bool) {
	t := strings.TrimSpace(raw)
	if t == "" || strings.HasPrefix(t, "#") {
		return "", false
	}
	if strings.HasPrefix(t, "- ") {
		t = strings.TrimSpace(t[2:])
	} else if t == "-" {
		return "", false
	}
	if t == "" || strings.HasPrefix(t, "#") {
		return "", false
	}
	if idx := strings.Index(t, " #"); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
	}
	if len(t) >= 2 {
		if (t[0] == '"' && t[len(t)-1] == '"') || (t[0] == '\'' && t[len(t)-1] == '\'') {
			t = strings.TrimSpace(t[1 : len(t)-1])
		}
	}
	if t == "" {
		return "", false
	}
	return t, true
}

// classifyRuleLine inspects a cleaned rule line.
//   - reason != "" → the editor flags it with that message + suggestion.
//   - hard == true → it is structurally broken for a KNOWN type (missing
//     policy), so the render path also drops it to keep the config loadable.
//
// Unknown types are only soft-flagged as possible typos (reason set, hard=false)
// and are kept at render — we can't be sure of an unknown type's arity, so we
// never silently drop a line that might be a valid but uncommon rule.
func classifyRuleLine(text string) (reason, suggestion string, hard bool) {
	fields := splitTopLevelCommas(text)
	typ := strings.ToUpper(strings.TrimSpace(fields[0]))
	switch {
	case len(fields) < 2:
		return "规则不完整,缺少策略组(应为 类型,值,策略组)", text + ",<策略组>", true
	case twoFieldRuleTypes[typ]:
		return "", "", false
	case knownRuleTypes[typ]:
		if len(fields) < 3 {
			return typ + " 缺少策略组(第 3 段)", text + ",<策略组>", true
		}
		return "", "", false
	default:
		if near := nearestRuleType(typ); near != "" {
			fixed := near + "," + strings.Join(fields[1:], ",")
			return "未知规则类型 " + typ + ",是否想写 " + near + "?", fixed, false
		}
		return "", "", false
	}
}

// renderableRuleLine reports whether a cleaned line should survive into the
// rendered rules: — false drops structurally-broken lines (known type missing
// its policy) so the subscription still loads even if a bad rule slipped past
// the editor.
func renderableRuleLine(text string) bool {
	_, _, hard := classifyRuleLine(text)
	return !hard
}

// splitTopLevelCommas splits on commas that are NOT inside parentheses, so
// logical rules like AND,((DOMAIN,a),(NETWORK,b)),策略组 keep their nested
// commas intact.
func splitTopLevelCommas(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// nearestRuleType returns the closest known rule type within edit-distance 2,
// or "" when the input is not an obvious typo of any known type.
func nearestRuleType(typ string) string {
	if len(typ) < 3 {
		return "" // too short to confidently call a typo (avoids matching "A")
	}
	best, bestDist := "", 3
	for known := range knownRuleTypes {
		if d := levenshtein(typ, known); d < bestDist {
			best, bestDist = known, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
