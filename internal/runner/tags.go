package runner

import (
	"fmt"
	"strings"
	"unicode"
)

// toGodogTags translates a Cucumber tag expression ("@smoke and not @slow",
// "(@a or @b) and not @wip") into godog's legacy filter syntax, where
// "&&" is AND, "," is OR and "~" is NOT, with no parentheses. godog
// evaluates that syntax as an AND of OR-groups, so the expression is
// converted to conjunctive normal form first.
//
// Expressions without and/or/not/parentheses are returned unchanged, so
// the legacy syntax ("@smoke && ~@slow") keeps working.
func toGodogTags(expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" || !isTagExpression(expr) {
		return expr, nil
	}

	p := &tagParser{tokens: tokenizeTags(expr)}
	node, err := p.parseOr()
	if err != nil {
		return "", fmt.Errorf("invalid tag expression %q: %w", expr, err)
	}
	if p.pos < len(p.tokens) {
		return "", fmt.Errorf("invalid tag expression %q: unexpected %q", expr, p.tokens[p.pos])
	}

	clauses := toCNF(node)
	groups := make([]string, 0, len(clauses))
	for _, c := range clauses {
		groups = append(groups, strings.Join(c, ","))
	}
	return strings.Join(groups, " && "), nil
}

func isTagExpression(expr string) bool {
	for _, t := range tokenizeTags(expr) {
		switch t {
		case "and", "or", "not", "(", ")":
			return true
		}
	}
	return false
}

func tokenizeTags(expr string) []string {
	var tokens []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	for _, r := range expr {
		switch {
		case r == '(' || r == ')':
			flush()
			tokens = append(tokens, string(r))
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return tokens
}

// tagNode is a parsed tag expression: a literal tag, or not/and/or.
type tagNode struct {
	op          string // "tag", "not", "and", "or"
	tag         string
	left, right *tagNode
}

// tagParser implements Cucumber precedence: not > and > or.
type tagParser struct {
	tokens []string
	pos    int
}

func (p *tagParser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *tagParser) parseOr() (*tagNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() == "or" {
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &tagNode{op: "or", left: left, right: right}
	}
	return left, nil
}

func (p *tagParser) parseAnd() (*tagNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek() == "and" {
		p.pos++
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &tagNode{op: "and", left: left, right: right}
	}
	return left, nil
}

func (p *tagParser) parseNot() (*tagNode, error) {
	switch tok := p.peek(); {
	case tok == "not":
		p.pos++
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &tagNode{op: "not", left: inner}, nil
	case tok == "(":
		p.pos++
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return inner, nil
	case strings.HasPrefix(tok, "@"):
		p.pos++
		return &tagNode{op: "tag", tag: tok}, nil
	case tok == "":
		return nil, fmt.Errorf("unexpected end of expression")
	default:
		return nil, fmt.Errorf("unexpected %q (tags must start with @)", tok)
	}
}

// toCNF returns the expression as an AND of OR-clauses of literals
// ("@tag" or "~@tag"). Negations are pushed down with De Morgan's laws.
func toCNF(n *tagNode) [][]string {
	return cnf(n, false)
}

func cnf(n *tagNode, negated bool) [][]string {
	switch n.op {
	case "tag":
		if negated {
			return [][]string{{"~" + n.tag}}
		}
		return [][]string{{n.tag}}
	case "not":
		return cnf(n.left, !negated)
	}

	isAnd := n.op == "and"
	if negated {
		isAnd = !isAnd
	}
	left, right := cnf(n.left, negated), cnf(n.right, negated)
	if isAnd {
		return append(left, right...)
	}
	// (A1 ∧ A2) ∨ (B1 ∧ B2) = ∧ of every (Ai ∨ Bj)
	out := make([][]string, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			clause := append(append([]string{}, l...), r...)
			out = append(out, clause)
		}
	}
	return out
}
