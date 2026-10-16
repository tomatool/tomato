package pdf

import (
	"strconv"
	"strings"
)

// A content stream and a CMap share PDF's token syntax (ISO 32000-1, 7.2):
// numbers, names, strings, arrays, dictionaries, and bare keywords, which
// are the operators of a content stream and the procedures of a CMap. The
// lexer is lenient: a malformed token is skipped rather than failing the
// page, since the point is to read what text a document has, not to
// validate it.

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokName
	tokString
	tokArrayOpen
	tokArrayClose
	tokDictOpen
	tokDictClose
	tokKeyword
)

type token struct {
	kind tokenKind
	num  float64
	text string // a name without its slash, a keyword, or the bytes of a string
}

type lexer struct {
	src []byte
	pos int
}

func isWhitespace(b byte) bool {
	return b == 0 || b == '\t' || b == '\n' || b == '\f' || b == '\r' || b == ' '
}

func isDelimiter(b byte) bool {
	switch b {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func isRegular(b byte) bool { return !isWhitespace(b) && !isDelimiter(b) }

func (l *lexer) skipSpace() {
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		switch {
		case isWhitespace(b):
			l.pos++
		case b == '%':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
				l.pos++
			}
		default:
			return
		}
	}
}

func (l *lexer) next() token {
	for {
		l.skipSpace()
		if l.pos >= len(l.src) {
			return token{kind: tokEOF}
		}
		b := l.src[l.pos]
		switch {
		case b == '[':
			l.pos++
			return token{kind: tokArrayOpen}
		case b == ']':
			l.pos++
			return token{kind: tokArrayClose}
		case b == '<':
			if l.pos+1 < len(l.src) && l.src[l.pos+1] == '<' {
				l.pos += 2
				return token{kind: tokDictOpen}
			}
			l.pos++
			return token{kind: tokString, text: l.hexString()}
		case b == '>':
			if l.pos+1 < len(l.src) && l.src[l.pos+1] == '>' {
				l.pos += 2
				return token{kind: tokDictClose}
			}
			l.pos++ // a stray '>'
		case b == '(':
			l.pos++
			return token{kind: tokString, text: l.literalString()}
		case b == ')', b == '{', b == '}':
			l.pos++ // unbalanced, or a PostScript procedure brace: no meaning here
		case b == '/':
			l.pos++
			return token{kind: tokName, text: l.name()}
		default:
			start := l.pos
			for l.pos < len(l.src) && isRegular(l.src[l.pos]) {
				l.pos++
			}
			word := string(l.src[start:l.pos])
			if b == '+' || b == '-' || b == '.' || (b >= '0' && b <= '9') {
				if n, ok := parseNumber(word); ok {
					return token{kind: tokNumber, num: n}
				}
			}
			return token{kind: tokKeyword, text: word}
		}
	}
}

// parseNumber reads a PDF number, tolerating what generators emit: "6.",
// ".5", "--5" and "3.4.5" (read as 3.4).
func parseNumber(s string) (float64, bool) {
	neg := false
	for len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			neg = !neg
		}
		s = s[1:]
	}
	if s == "" || s == "." {
		return 0, false
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		if j := strings.IndexByte(s[i+1:], '.'); j >= 0 {
			s = s[:i+1+j]
		}
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}

// literalString reads a (string) after its opening parenthesis: nested
// parentheses balance, and backslash escapes are resolved.
func (l *lexer) literalString() string {
	var out []byte
	depth := 1
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		l.pos++
		switch b {
		case '\\':
			if l.pos >= len(l.src) {
				return string(out)
			}
			e := l.src[l.pos]
			l.pos++
			switch e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '\r':
				if l.pos < len(l.src) && l.src[l.pos] == '\n' {
					l.pos++
				}
			case '\n':
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for i := 0; i < 2 && l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '7'; i++ {
						v = v*8 + int(l.src[l.pos]-'0')
						l.pos++
					}
					out = append(out, byte(v))
				} else {
					out = append(out, e)
				}
			}
		case '(':
			depth++
			out = append(out, b)
		case ')':
			depth--
			if depth == 0 {
				return string(out)
			}
			out = append(out, b)
		default:
			out = append(out, b)
		}
	}
	return string(out)
}

// hexString reads a <hex string> after its opening bracket. Whitespace is
// skipped and a missing final digit is 0.
func (l *lexer) hexString() string {
	var out []byte
	var cur, n int
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		l.pos++
		if b == '>' {
			break
		}
		v, ok := hexValue(b)
		if !ok {
			continue
		}
		cur = cur<<4 | v
		n++
		if n == 2 {
			out = append(out, byte(cur))
			cur, n = 0, 0
		}
	}
	if n == 1 {
		out = append(out, byte(cur<<4))
	}
	return string(out)
}

func hexValue(b byte) (int, bool) {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0'), true
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10, true
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10, true
	}
	return 0, false
}

// name reads a /Name after its slash, resolving #xx escapes.
func (l *lexer) name() string {
	var out []byte
	for l.pos < len(l.src) && isRegular(l.src[l.pos]) {
		b := l.src[l.pos]
		l.pos++
		if b == '#' && l.pos+1 < len(l.src) {
			hi, ok1 := hexValue(l.src[l.pos])
			lo, ok2 := hexValue(l.src[l.pos+1])
			if ok1 && ok2 {
				out = append(out, byte(hi<<4|lo))
				l.pos += 2
				continue
			}
		}
		out = append(out, b)
	}
	return string(out)
}

// operand is a parsed value: the operands of an operator, or the items of a
// CMap section.
type operand struct {
	kind  tokenKind // tokNumber, tokName, tokString, tokArrayOpen (an array), tokDictOpen (a dict), tokKeyword (true/false/null)
	num   float64
	text  string
	items []operand
	dict  map[string]operand
}

func (o operand) isNumber() bool { return o.kind == tokNumber }
func (o operand) isString() bool { return o.kind == tokString }

// parser turns tokens into operands and operators.
type parser struct {
	lex lexer
}

func newParser(src []byte) *parser { return &parser{lex: lexer{src: src}} }

// maxOperands bounds the operands kept for one operator; a damaged stream
// could otherwise accumulate without end.
const maxOperands = 64

// next returns the next operator with its operands. Inline images are
// skipped here, since their binary data is not token syntax.
func (p *parser) next() (ops []operand, operator string, ok bool) {
	for {
		t := p.lex.next()
		switch t.kind {
		case tokEOF:
			return nil, "", false
		case tokKeyword:
			switch t.text {
			case "true", "false", "null":
				ops = append(ops, operand{kind: tokKeyword, text: t.text})
				continue
			case "BI":
				p.skipInlineImage()
				ops = ops[:0]
				continue
			}
			return ops, t.text, true
		case tokArrayClose, tokDictClose:
			continue // unbalanced; ignore
		default:
			ops = append(ops, p.value(t, 0))
		}
		if len(ops) > maxOperands {
			ops = ops[len(ops)-maxOperands:]
		}
	}
}

// maxNesting bounds arrays within arrays.
const maxNesting = 32

func (p *parser) value(t token, depth int) operand {
	switch t.kind {
	case tokNumber:
		return operand{kind: tokNumber, num: t.num}
	case tokName:
		return operand{kind: tokName, text: t.text}
	case tokString:
		return operand{kind: tokString, text: t.text}
	case tokArrayOpen:
		arr := operand{kind: tokArrayOpen}
		for {
			n := p.lex.next()
			if n.kind == tokEOF || n.kind == tokArrayClose {
				return arr
			}
			if n.kind == tokDictClose {
				continue
			}
			if depth >= maxNesting && (n.kind == tokArrayOpen || n.kind == tokDictOpen) {
				continue
			}
			arr.items = append(arr.items, p.value(n, depth+1))
		}
	case tokDictOpen:
		d := operand{kind: tokDictOpen, dict: map[string]operand{}}
		for {
			k := p.lex.next()
			if k.kind == tokEOF || k.kind == tokDictClose {
				return d
			}
			if k.kind != tokName {
				continue
			}
			v := p.lex.next()
			if v.kind == tokEOF || v.kind == tokDictClose {
				return d
			}
			if depth >= maxNesting && (v.kind == tokArrayOpen || v.kind == tokDictOpen) {
				continue
			}
			d.dict[k.text] = p.value(v, depth+1)
		}
	default:
		return operand{kind: tokKeyword, text: t.text}
	}
}

// skipInlineImage skips from after BI to after EI: the image dictionary up
// to ID, one whitespace byte, then binary data until an EI that stands on
// its own between whitespace.
func (p *parser) skipInlineImage() {
	for {
		t := p.lex.next()
		if t.kind == tokEOF {
			return
		}
		if t.kind == tokKeyword && t.text == "ID" {
			break
		}
	}
	src := p.lex.src
	i := p.lex.pos + 1
	for i+1 < len(src) {
		if src[i] == 'E' && src[i+1] == 'I' && isWhitespace(src[i-1]) && (i+2 >= len(src) || isWhitespace(src[i+2]) || isDelimiter(src[i+2])) {
			p.lex.pos = i + 2
			return
		}
		i++
	}
	p.lex.pos = len(src)
}
