package detect

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// Evaluation engine: matches a single event (a decoded log record) against a
// Sigma rule's detection block. This is a deliberately pragmatic subset of the
// Sigma specification — enough to make detections testable with embedded sample
// events (see runtest.go). Unsupported constructs return an explicit error
// rather than silently passing or failing.

// EvaluateRule reports whether event satisfies the rule's detection/condition.
// rule is a decoded Sigma document (as produced by normalize). Returns an error
// for unsupported constructs (e.g. aggregation conditions) so callers never
// mistake "couldn't evaluate" for "didn't match".
func EvaluateRule(rule map[string]any, event map[string]any) (bool, error) {
	det, ok := rule["detection"].(map[string]any)
	if !ok {
		return false, fmt.Errorf("rule has no detection block")
	}
	condRaw, ok := det["condition"]
	if !ok {
		return false, fmt.Errorf("detection has no condition")
	}
	cond, ok := condRaw.(string)
	if !ok {
		return false, fmt.Errorf("condition must be a string")
	}

	// Pre-evaluate every named search identifier against the event.
	names := make([]string, 0, len(det))
	results := make(map[string]bool, len(det))
	for name, ident := range det {
		if name == "condition" {
			continue
		}
		m, err := matchSearch(event, ident)
		if err != nil {
			return false, fmt.Errorf("search %q: %w", name, err)
		}
		names = append(names, name)
		results[name] = m
	}

	return evalCondition(cond, names, results)
}

// matchSearch evaluates one search identifier against the event. An identifier
// is a map (all field entries must match — AND), a list of maps (any map
// matches — OR), or a list of keyword strings (any keyword found anywhere — OR).
func matchSearch(event map[string]any, ident any) (bool, error) {
	switch v := ident.(type) {
	case map[string]any:
		return matchMap(event, v)
	case []any:
		if len(v) == 0 {
			return false, nil
		}
		// Keyword list (free-text) vs list-of-maps (OR of selections).
		if _, isMap := v[0].(map[string]any); !isMap {
			return matchKeywords(event, v), nil
		}
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return false, fmt.Errorf("mixed list in search identifier")
			}
			matched, err := matchMap(event, m)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported search identifier type %T", ident)
	}
}

// matchMap requires every "field|modifiers: value" entry to match (AND).
func matchMap(event map[string]any, m map[string]any) (bool, error) {
	for key, expected := range m {
		ok, err := matchField(event, key, expected)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// matchKeywords reports whether any keyword appears (case-insensitively) as a
// substring of any stringified value in the event.
func matchKeywords(event map[string]any, keywords []any) bool {
	hay := strings.ToLower(strings.Join(allStrings(event), "\n"))
	for _, kw := range keywords {
		if strings.Contains(hay, strings.ToLower(fmt.Sprint(kw))) {
			return true
		}
	}
	return false
}

// matchField evaluates a single "field|mod1|mod2: expected" entry.
func matchField(event map[string]any, keyExpr string, expected any) (bool, error) {
	parts := strings.Split(keyExpr, "|")
	field := parts[0]
	mods := parts[1:]

	actuals, found := lookup(event, field)

	// A null expected value matches when the field is absent or null.
	if expected == nil {
		if !found || len(actuals) == 0 {
			return true, nil
		}
		for _, a := range actuals {
			if a != nil {
				return false, nil
			}
		}
		return true, nil
	}
	if !found {
		return false, nil
	}

	expectedList := toList(expected)
	combineAll := hasMod(mods, "all") // list values combined with AND instead of OR

	for _, exp := range expectedList {
		matched := false
		for _, act := range actuals {
			ok, err := matchValue(act, exp, mods)
			if err != nil {
				return false, err
			}
			if ok {
				matched = true
				break
			}
		}
		if combineAll && !matched {
			return false, nil
		}
		if !combineAll && matched {
			return true, nil
		}
	}
	return combineAll, nil
}

// matchValue compares one actual event value against one expected value under
// the given modifiers.
func matchValue(actual, expected any, mods []string) (bool, error) {
	switch {
	case hasMod(mods, "re"):
		re, err := regexp.Compile(fmt.Sprint(expected))
		if err != nil {
			return false, fmt.Errorf("invalid regex %q: %w", expected, err)
		}
		return re.MatchString(fmt.Sprint(actual)), nil
	case hasMod(mods, "cidr"):
		return matchCIDR(fmt.Sprint(actual), fmt.Sprint(expected))
	case hasNumericMod(mods):
		return matchNumeric(actual, expected, mods)
	}

	for _, m := range mods {
		switch m {
		case "all", "cased":
			// handled elsewhere / affects comparison below
		case "contains", "startswith", "endswith", "windash":
		case "base64", "base64offset", "utf16", "utf16le", "utf16be", "wide":
			return false, fmt.Errorf("unsupported modifier %q", m)
		default:
			return false, fmt.Errorf("unsupported modifier %q", m)
		}
	}

	a := fmt.Sprint(actual)
	e := fmt.Sprint(expected)
	if !hasMod(mods, "cased") {
		a = strings.ToLower(a)
		e = strings.ToLower(e)
	}
	switch {
	case hasMod(mods, "contains"):
		return strings.Contains(a, e), nil
	case hasMod(mods, "startswith"):
		return strings.HasPrefix(a, e), nil
	case hasMod(mods, "endswith"):
		return strings.HasSuffix(a, e), nil
	default:
		return wildcardMatch(e, a), nil
	}
}

func matchCIDR(actual, cidr string) (bool, error) {
	pre, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false, fmt.Errorf("invalid cidr %q: %w", cidr, err)
	}
	ip, err := netip.ParseAddr(actual)
	if err != nil {
		return false, nil // a non-IP value simply doesn't match the range
	}
	return pre.Contains(ip), nil
}

func matchNumeric(actual, expected any, mods []string) (bool, error) {
	a, err := toFloat(actual)
	if err != nil {
		return false, nil
	}
	e, err := toFloat(expected)
	if err != nil {
		return false, fmt.Errorf("non-numeric comparison value %v", expected)
	}
	switch {
	case hasMod(mods, "lt"):
		return a < e, nil
	case hasMod(mods, "lte"):
		return a <= e, nil
	case hasMod(mods, "gt"):
		return a > e, nil
	case hasMod(mods, "gte"):
		return a >= e, nil
	}
	return false, nil
}

// lookup resolves a (possibly dotted) field name to its value(s). It supports
// both flat dotted keys ({"userIdentity.type": "Root"}) and nested objects
// ({"userIdentity": {"type": "Root"}}). A multi-valued field yields all values.
func lookup(m map[string]any, field string) ([]any, bool) {
	if v, ok := m[field]; ok {
		return toList(v), true
	}
	head, rest, found := strings.Cut(field, ".")
	if !found {
		return nil, false
	}
	child, ok := m[head]
	if !ok {
		return nil, false
	}
	switch c := child.(type) {
	case map[string]any:
		return lookup(c, rest)
	case []any:
		var out []any
		ok := false
		for _, item := range c {
			if cm, isMap := item.(map[string]any); isMap {
				if vs, found := lookup(cm, rest); found {
					out = append(out, vs...)
					ok = true
				}
			}
		}
		return out, ok
	default:
		return nil, false
	}
}

// wildcardMatch matches a Sigma glob (supporting * and ?, with backslash
// escapes) against s. Both arguments are expected to already be case-folded by
// the caller when case-insensitive matching is desired.
func wildcardMatch(pattern, s string) bool {
	if !strings.ContainsAny(pattern, "*?\\") {
		return pattern == s
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '\\':
			if i+1 < len(pattern) {
				b.WriteString(regexp.QuoteMeta(string(pattern[i+1])))
				i++
			} else {
				b.WriteString(`\\`)
			}
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

// allStrings flattens every scalar value in a nested structure to strings.
func allStrings(v any) []string {
	switch t := v.(type) {
	case map[string]any:
		var out []string
		for _, val := range t {
			out = append(out, allStrings(val)...)
		}
		return out
	case []any:
		var out []string
		for _, val := range t {
			out = append(out, allStrings(val)...)
		}
		return out
	case nil:
		return nil
	default:
		return []string{fmt.Sprint(t)}
	}
}

func toList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return []any{v}
}

func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case float64:
		return n, nil
	case string:
		return strconv.ParseFloat(n, 64)
	default:
		return strconv.ParseFloat(fmt.Sprint(v), 64)
	}
}

func hasMod(mods []string, name string) bool {
	for _, m := range mods {
		if m == name {
			return true
		}
	}
	return false
}

func hasNumericMod(mods []string) bool {
	return hasMod(mods, "lt") || hasMod(mods, "lte") || hasMod(mods, "gt") || hasMod(mods, "gte")
}
