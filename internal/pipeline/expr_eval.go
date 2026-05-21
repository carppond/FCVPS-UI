package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"shiguang-vps/internal/substore"
)

// truthy is the Boolean coercion used by &&, ||, !, and bare-expression tests
// inside filter expressions. Matches the loose JavaScript / Lua convention:
// empty string / zero / nil / empty list are false; anything else is true.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case int64:
		return x != 0
	case int:
		return x != 0
	case float64:
		return x != 0
	case []any:
		return len(x) > 0
	}
	return true
}

// compare returns the boolean result of "lv op rv" with light type coercion:
// numeric comparisons promote int → float; equality across numeric / string
// follows Go-like rules.
func compare(op string, lv, rv any) (any, error) {
	switch op {
	case "in", "not in":
		return compareIn(op, lv, rv)
	case "==":
		ok, err := equal(lv, rv)
		return ok, err
	case "!=":
		ok, err := equal(lv, rv)
		return !ok, err
	case "~=":
		return compareRegex(lv, rv)
	case "<", ">", "<=", ">=":
		return compareOrder(op, lv, rv)
	}
	return nil, fmt.Errorf("unknown op %q", op)
}

func compareIn(op string, lv, rv any) (any, error) {
	list, ok := rv.([]any)
	if !ok {
		return nil, fmt.Errorf("op %q: rhs must be a list, got %T", op, rv)
	}
	found := false
	for _, item := range list {
		if ok, _ := equal(lv, item); ok {
			found = true
			break
		}
	}
	if op == "not in" {
		return !found, nil
	}
	return found, nil
}

func compareRegex(lv, rv any) (any, error) {
	ls, ok := lv.(string)
	if !ok {
		ls = fmt.Sprintf("%v", lv)
	}
	rs, ok := rv.(string)
	if !ok {
		return nil, fmt.Errorf("~= rhs must be string, got %T", rv)
	}
	re, err := regexp.Compile(rs)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRegex, err)
	}
	return re.MatchString(ls), nil
}

func compareOrder(op string, lv, rv any) (any, error) {
	if lf, lok := toNumber(lv); lok {
		if rf, rok := toNumber(rv); rok {
			switch op {
			case "<":
				return lf < rf, nil
			case ">":
				return lf > rf, nil
			case "<=":
				return lf <= rf, nil
			case ">=":
				return lf >= rf, nil
			}
		}
	}
	ls, lsOK := lv.(string)
	rs, rsOK := rv.(string)
	if lsOK && rsOK {
		switch op {
		case "<":
			return ls < rs, nil
		case ">":
			return ls > rs, nil
		case "<=":
			return ls <= rs, nil
		case ">=":
			return ls >= rs, nil
		}
	}
	return nil, fmt.Errorf("op %q: incompatible operands %T %T", op, lv, rv)
}

func equal(a, b any) (bool, error) {
	if af, aok := toNumber(a); aok {
		if bf, bok := toNumber(b); bok {
			return af == bf, nil
		}
	}
	if as, ok := a.(string); ok {
		if bs, ok := b.(string); ok {
			return as == bs, nil
		}
	}
	if ab, ok := a.(bool); ok {
		if bb, ok := b.(bool); ok {
			return ab == bb, nil
		}
	}
	if a == nil && b == nil {
		return true, nil
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b), nil
}

func toNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// regionPattern detects common 2-letter region codes / English country names
// inside node names. Case-insensitive.
var regionPattern = regexp.MustCompile(`(?i)\b(hk|tw|jp|sg|us|uk|de|ru|kr|in|au|ca|tr|nl|fr|br|hongkong|taiwan|japan|singapore|usa|united\s*states|korea|germany|russia|india|australia|canada|france|brazil)\b`)

// resolveField extracts a logical field value from a ParsedNode. It always
// returns a Go value compatible with compare(): string for textual, int64 for
// integers, bool for booleans, nil when not present.
func resolveField(node *substore.ParsedNode, name string) (any, error) {
	if node == nil {
		return nil, nil
	}
	switch name {
	case "name":
		return node.Name, nil
	case "server":
		return node.Server, nil
	case "port":
		return int64(node.Port), nil
	case "protocol":
		return node.Protocol, nil
	case "tag":
		return node.Tag, nil
	case "network":
		return node.Network, nil
	case "tls":
		return node.TLS, nil
	case "sni":
		return node.SNI, nil
	case "password":
		return node.Password, nil
	case "uuid":
		return node.UUID, nil
	case "method":
		return node.Method, nil
	case "host":
		return node.Host, nil
	case "path":
		return node.Path, nil
	case "region":
		return deriveRegion(node), nil
	}
	if strings.HasPrefix(name, "raw.") {
		if node.Raw == nil {
			return nil, nil
		}
		key := strings.TrimPrefix(name, "raw.")
		if v, ok := node.Raw[key]; ok {
			return v, nil
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unknown field %q", name)
}

// deriveRegion returns a lowercase region code inferred from the node name +
// tag. Empty string when nothing matches. Used by the `region` field in
// filter expressions.
func deriveRegion(node *substore.ParsedNode) string {
	hay := strings.ToLower(node.Name + " " + node.Tag)
	m := regionPattern.FindString(hay)
	if m == "" {
		return ""
	}
	switch strings.ReplaceAll(strings.ToLower(m), " ", "") {
	case "hongkong":
		return "hk"
	case "taiwan":
		return "tw"
	case "japan":
		return "jp"
	case "singapore":
		return "sg"
	case "usa", "unitedstates":
		return "us"
	case "korea":
		return "kr"
	case "germany":
		return "de"
	case "russia":
		return "ru"
	case "india":
		return "in"
	case "australia":
		return "au"
	case "canada":
		return "ca"
	case "france":
		return "fr"
	case "brazil":
		return "br"
	}
	return strings.ToLower(m)
}
