package pipeline

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"shiguang-vps/internal/substore"
)

// Filter expression mini-language.
//
// Grammar (parsed by the recursive-descent evaluator below):
//
//	expr      := or
//	or        := and ( "||" and )*
//	and       := unary ( "&&" unary )*
//	unary     := "!" unary | primary
//	primary   := "(" expr ")" | comparison
//	comparison:= ident op rhs
//	op        := "==" | "!=" | "in" | "not in" | "~=" | "<" | ">" | "<=" | ">="
//	rhs       := string | number | bool | listLit
//	listLit   := "[" ( string|number|bool ( "," ... )* )? "]"
//
// Fields accessible on ParsedNode: name, server, port, protocol, tag,
// network, tls (bool), sni, password, uuid, method, host, path, region.
// `region` is derived: regex match of common country-code / city tokens in
// the node name + tag. The compare() / resolveField() helpers live in
// expr_eval.go.

// ErrInvalidExpression marks a parse / type error in a filter expression.
// Wraps to types.ErrPipelineOperatorParams at the API boundary.
var ErrInvalidExpression = errors.New("pipeline: invalid filter expression")

// FilterExpr is a compiled filter expression ready to evaluate against any
// ParsedNode. Construct via CompileFilter.
type FilterExpr struct {
	root exprNode
	src  string
}

// CompileFilter parses src into a FilterExpr.
func CompileFilter(src string) (*FilterExpr, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		// An empty expression keeps every node (no-op filter).
		return &FilterExpr{src: src, root: &literalNode{value: true}}, nil
	}
	p := newParser(src)
	root, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidExpression, err)
	}
	if !p.eof() {
		return nil, fmt.Errorf("%w: unexpected token %q", ErrInvalidExpression, p.peek())
	}
	return &FilterExpr{src: src, root: root}, nil
}

// Eval returns whether node satisfies the expression. Type mismatches surface
// as evaluation errors so callers can short-circuit (rather than silently
// falling through to "false", which makes debugging painful).
func (e *FilterExpr) Eval(node *substore.ParsedNode) (bool, error) {
	if e == nil || e.root == nil {
		return true, nil
	}
	v, err := e.root.eval(node)
	if err != nil {
		return false, err
	}
	return truthy(v), nil
}

// String returns the original expression source.
func (e *FilterExpr) String() string {
	if e == nil {
		return ""
	}
	return e.src
}

// -- AST node types ----------------------------------------------------------

type exprNode interface {
	eval(node *substore.ParsedNode) (any, error)
}

type literalNode struct{ value any }

func (n *literalNode) eval(*substore.ParsedNode) (any, error) { return n.value, nil }

type identNode struct{ name string }

func (n *identNode) eval(node *substore.ParsedNode) (any, error) {
	return resolveField(node, n.name)
}

type listNode struct{ items []any }

func (n *listNode) eval(*substore.ParsedNode) (any, error) { return n.items, nil }

type unaryNode struct {
	op    string
	inner exprNode
}

func (n *unaryNode) eval(node *substore.ParsedNode) (any, error) {
	v, err := n.inner.eval(node)
	if err != nil {
		return nil, err
	}
	if n.op == "!" {
		return !truthy(v), nil
	}
	return nil, fmt.Errorf("unknown unary op %q", n.op)
}

type binaryNode struct {
	op    string
	left  exprNode
	right exprNode
}

func (n *binaryNode) eval(node *substore.ParsedNode) (any, error) {
	if n.op == "&&" || n.op == "||" {
		return n.evalShortCircuit(node)
	}
	lv, err := n.left.eval(node)
	if err != nil {
		return nil, err
	}
	rv, err := n.right.eval(node)
	if err != nil {
		return nil, err
	}
	return compare(n.op, lv, rv)
}

func (n *binaryNode) evalShortCircuit(node *substore.ParsedNode) (any, error) {
	lv, err := n.left.eval(node)
	if err != nil {
		return nil, err
	}
	lb := truthy(lv)
	if n.op == "&&" && !lb {
		return false, nil
	}
	if n.op == "||" && lb {
		return true, nil
	}
	rv, err := n.right.eval(node)
	if err != nil {
		return nil, err
	}
	return truthy(rv), nil
}

// -- Parser ------------------------------------------------------------------

type parser struct {
	src string
	pos int
}

func newParser(src string) *parser { return &parser{src: src} }

func (p *parser) eof() bool {
	p.skipSpace()
	return p.pos >= len(p.src)
}

func (p *parser) peek() string {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return ""
	}
	end := p.pos + 1
	if end > len(p.src) {
		end = len(p.src)
	}
	return p.src[p.pos:end]
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}

func (p *parser) parseExpr() (exprNode, error) { return p.parseOr() }

func (p *parser) parseOr() (exprNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if !p.acceptLit("||") {
			break
		}
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: "||", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (exprNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if !p.acceptLit("&&") {
			break
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{op: "&&", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (exprNode, error) {
	p.skipSpace()
	if p.acceptLit("!") {
		// Disambiguate against "!=" — we already consumed '!'; peek for '=' to
		// roll back.
		if p.pos < len(p.src) && p.src[p.pos] == '=' {
			p.pos--
		} else {
			inner, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			return &unaryNode{op: "!", inner: inner}, nil
		}
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (exprNode, error) {
	p.skipSpace()
	if p.acceptLit("(") {
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.acceptLit(")") {
			return nil, fmt.Errorf("expected ')'")
		}
		return inner, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (exprNode, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	op := p.consumeOperator()
	if op == "" {
		return left, nil
	}
	right, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	return &binaryNode{op: op, left: left, right: right}, nil
}

func (p *parser) consumeOperator() string {
	p.skipSpace()
	candidates := []string{"==", "!=", "<=", ">=", "~=", "<", ">"}
	for _, c := range candidates {
		if p.acceptLit(c) {
			return c
		}
	}
	if p.acceptKeyword("not") && p.acceptKeyword("in") {
		return "not in"
	}
	if p.acceptKeyword("in") {
		return "in"
	}
	return ""
}

func (p *parser) parseAtom() (exprNode, error) {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	c := p.src[p.pos]
	switch {
	case c == '"' || c == '\'':
		s, err := p.parseString()
		if err != nil {
			return nil, err
		}
		return &literalNode{value: s}, nil
	case c == '[':
		return p.parseListLiteral()
	case c == '-' || (c >= '0' && c <= '9'):
		return p.parseNumber()
	case isIdentStart(rune(c)):
		ident := p.readIdent()
		switch ident {
		case "true":
			return &literalNode{value: true}, nil
		case "false":
			return &literalNode{value: false}, nil
		case "null", "nil":
			return &literalNode{value: nil}, nil
		}
		return &identNode{name: ident}, nil
	}
	return nil, fmt.Errorf("unexpected char %q at %d", c, p.pos)
}

func (p *parser) parseString() (string, error) {
	quote := p.src[p.pos]
	p.pos++
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] != quote {
		if p.src[p.pos] == '\\' && p.pos+1 < len(p.src) {
			p.pos += 2
			continue
		}
		p.pos++
	}
	if p.pos >= len(p.src) {
		return "", fmt.Errorf("unterminated string starting at %d", start-1)
	}
	raw := p.src[start:p.pos]
	p.pos++ // closing quote
	unescaped := strings.NewReplacer(`\"`, `"`, `\'`, `'`, `\\`, `\`).Replace(raw)
	return unescaped, nil
}

func (p *parser) parseNumber() (exprNode, error) {
	start := p.pos
	if p.src[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.src) && (p.src[p.pos] == '.' || (p.src[p.pos] >= '0' && p.src[p.pos] <= '9')) {
		p.pos++
	}
	tok := p.src[start:p.pos]
	if i, err := strconv.ParseInt(tok, 10, 64); err == nil {
		return &literalNode{value: i}, nil
	}
	f, err := strconv.ParseFloat(tok, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", tok)
	}
	return &literalNode{value: f}, nil
}

func (p *parser) parseListLiteral() (exprNode, error) {
	p.pos++ // '['
	out := &listNode{}
	for {
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == ']' {
			p.pos++
			return out, nil
		}
		atom, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		v, err := atom.eval(nil) // list literals contain only constants
		if err != nil {
			return nil, fmt.Errorf("non-constant in list literal: %w", err)
		}
		out.items = append(out.items, v)
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.pos < len(p.src) && p.src[p.pos] == ']' {
			p.pos++
			return out, nil
		}
		return nil, fmt.Errorf("expected ',' or ']' in list literal")
	}
}

func (p *parser) acceptLit(lit string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.src[p.pos:], lit) {
		p.pos += len(lit)
		return true
	}
	return false
}

func (p *parser) acceptKeyword(kw string) bool {
	p.skipSpace()
	if !strings.HasPrefix(p.src[p.pos:], kw) {
		return false
	}
	end := p.pos + len(kw)
	if end < len(p.src) {
		next := rune(p.src[end])
		if isIdentPart(next) {
			return false
		}
	}
	p.pos = end
	return true
}

func (p *parser) readIdent() string {
	start := p.pos
	for p.pos < len(p.src) && isIdentPart(rune(p.src[p.pos])) {
		p.pos++
	}
	return p.src[start:p.pos]
}

func isIdentStart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9') || r == '.'
}
