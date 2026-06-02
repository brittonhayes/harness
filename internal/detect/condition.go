package detect

import (
	"fmt"
	"strconv"
	"strings"
)

// Condition parsing & evaluation. Supports the boolean grammar Sigma uses for
// the `detection.condition` field:
//
//	expr   := or
//	or     := and ('or' and)*
//	and    := not ('and' not)*
//	not    := 'not' not | primary
//	primary:= '(' expr ')' | quantifier | NAME
//	quantifier := (INT | 'all') 'of' (PATTERN | 'them')
//
// PATTERN is a search-identifier name with an optional trailing '*' wildcard;
// 'them' means all identifiers. Aggregation expressions ("| count() by ...")
// are not supported and produce an explicit error.

// evalCondition evaluates the condition string given the per-identifier match
// results. names is the full set of search-identifier names.
func evalCondition(condition string, names []string, results map[string]bool) (bool, error) {
	if strings.Contains(condition, "|") {
		return false, fmt.Errorf("aggregation conditions (|) are not supported by the test engine")
	}
	toks, err := tokenize(condition)
	if err != nil {
		return false, err
	}
	p := &condParser{toks: toks, names: names, results: results}
	node, err := p.parseExpr()
	if err != nil {
		return false, err
	}
	if p.pos != len(p.toks) {
		return false, fmt.Errorf("unexpected token %q in condition", p.toks[p.pos])
	}
	return node, nil
}

// tokenize splits a condition into words and parentheses.
func tokenize(s string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '(' || r == ')':
			flush()
			toks = append(toks, string(r))
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	if len(toks) == 0 {
		return nil, fmt.Errorf("empty condition")
	}
	return toks, nil
}

type condParser struct {
	toks    []string
	pos     int
	names   []string
	results map[string]bool
}

func (p *condParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func (p *condParser) next() string {
	t := p.peek()
	p.pos++
	return t
}

func (p *condParser) parseExpr() (bool, error) { return p.parseOr() }

func (p *condParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for strings.EqualFold(p.peek(), "or") {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

func (p *condParser) parseAnd() (bool, error) {
	left, err := p.parseNot()
	if err != nil {
		return false, err
	}
	for strings.EqualFold(p.peek(), "and") {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

func (p *condParser) parseNot() (bool, error) {
	if strings.EqualFold(p.peek(), "not") {
		p.next()
		v, err := p.parseNot()
		if err != nil {
			return false, err
		}
		return !v, nil
	}
	return p.parsePrimary()
}

func (p *condParser) parsePrimary() (bool, error) {
	tok := p.peek()
	if tok == "" {
		return false, fmt.Errorf("unexpected end of condition")
	}
	if tok == "(" {
		p.next()
		v, err := p.parseExpr()
		if err != nil {
			return false, err
		}
		if p.next() != ")" {
			return false, fmt.Errorf("missing closing parenthesis")
		}
		return v, nil
	}
	// Quantifier: "<N|all> of <pattern|them>".
	if strings.EqualFold(tok, "all") || isInt(tok) {
		if strings.EqualFold(p.peekAt(1), "of") {
			return p.parseQuantifier()
		}
	}
	// Bare identifier.
	name := p.next()
	v, ok := p.results[name]
	if !ok {
		return false, fmt.Errorf("unknown search identifier %q in condition", name)
	}
	return v, nil
}

func (p *condParser) peekAt(n int) string {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n]
	}
	return ""
}

func (p *condParser) parseQuantifier() (bool, error) {
	quant := p.next() // "all" or an integer
	p.next()          // "of"
	pattern := p.next()
	if pattern == "" {
		return false, fmt.Errorf("expected pattern after 'of'")
	}

	matched := 0
	total := 0
	for _, name := range p.names {
		if pattern == "them" || patternMatch(pattern, name) {
			total++
			if p.results[name] {
				matched++
			}
		}
	}
	if total == 0 {
		return false, fmt.Errorf("quantifier pattern %q matched no search identifiers", pattern)
	}
	if strings.EqualFold(quant, "all") {
		return matched == total, nil
	}
	n, _ := strconv.Atoi(quant)
	return matched >= n, nil
}

// patternMatch matches a quantifier pattern (optionally ending in '*') against
// an identifier name.
func patternMatch(pattern, name string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == name
}

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
