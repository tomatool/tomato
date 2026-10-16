package pdf

import (
	"strings"
	"testing"
)

func tokens(src string) []token {
	l := &lexer{src: []byte(src)}
	var out []token
	for {
		t := l.next()
		if t.kind == tokEOF {
			return out
		}
		out = append(out, t)
	}
}

func TestLexerNumbersNamesAndKeywords(t *testing.T) {
	got := tokens("1 -2.5 .5 6. --5 /Name /A#20B /F1 Tj true BT %comment\n ET")
	want := []token{
		{kind: tokNumber, num: 1}, {kind: tokNumber, num: -2.5}, {kind: tokNumber, num: 0.5}, {kind: tokNumber, num: 6},
		{kind: tokNumber, num: 5}, {kind: tokName, text: "Name"}, {kind: tokName, text: "A B"}, {kind: tokName, text: "F1"},
		{kind: tokKeyword, text: "Tj"}, {kind: tokKeyword, text: "true"}, {kind: tokKeyword, text: "BT"}, {kind: tokKeyword, text: "ET"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d tokens %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLexerStrings(t *testing.T) {
	cases := map[string]string{
		`(plain)`:                   "plain",
		`(nested (paren) ok)`:       "nested (paren) ok",
		`(esc \( \) \\ \n\t\r\b\f)`: "esc ( ) \\ \n\t\r\b\f",
		`(oct \101\12\7x)`:          "oct A\n\ax",
		"(cont\\\nnext)":            "contnext",
		"(crlf\\\r\nnext)":          "crlfnext",
		`<48656C6C6F>`:              "Hello",
		`<48 65 6C\n6C 6F>`:         "Hello",
		`<4>`:                       "@",
		`(unterminated`:             "unterminated",
	}
	for src, want := range cases {
		got := tokens(src)
		if len(got) != 1 || got[0].kind != tokString || got[0].text != want {
			t.Errorf("%s: got %+v, want string %q", src, got, want)
		}
	}
}

func TestParseNumber(t *testing.T) {
	for src, want := range map[string]float64{"3.4.5": 3.4, "+7": 7, "-.5": -0.5, "---1": -1} {
		got, ok := parseNumber(src)
		if !ok || got != want {
			t.Errorf("%s: got %v %v, want %v", src, got, ok, want)
		}
	}
	for _, src := range []string{".", "-", "", "abc", "1e"} {
		if _, ok := parseNumber(src); ok {
			t.Errorf("%q should not parse as a number", src)
		}
	}
}

func TestParserOperandsAndOperators(t *testing.T) {
	p := newParser([]byte("BT /F1 12 Tf 1 0 0 1 72 700 Tm [(A) -300 (B)] TJ << /MCID 0 /K [1 2] >> BDC null true ET ] >> (x) Tj"))
	type step struct {
		op   string
		nops int
	}
	var got []step
	var tj, bdc []operand
	for {
		ops, op, ok := p.next()
		if !ok {
			break
		}
		got = append(got, step{op, len(ops)})
		switch op {
		case "TJ":
			tj = ops
		case "BDC":
			bdc = ops
		}
	}
	want := []step{{"BT", 0}, {"Tf", 2}, {"Tm", 6}, {"TJ", 1}, {"BDC", 1}, {"ET", 2}, {"Tj", 1}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d: got %v, want %v", i, got[i], want[i])
		}
	}
	if tj[0].kind != tokArrayOpen || len(tj[0].items) != 3 || tj[0].items[1].num != -300 || tj[0].items[2].text != "B" {
		t.Errorf("TJ array: %+v", tj[0])
	}
	d := bdc[0]
	if d.kind != tokDictOpen || d.dict["MCID"].num != 0 || len(d.dict["K"].items) != 2 {
		t.Errorf("BDC dict: %+v", d)
	}
}

func TestParserSkipsInlineImages(t *testing.T) {
	src := "(before) Tj BI /W 2 /H 2 /BPC 8 /CS /G ID \x00\xffEI\x01 EI\n(after) Tj q BI /W 1 ID "
	p := newParser([]byte(src))
	var shown []string
	for {
		ops, op, ok := p.next()
		if !ok {
			break
		}
		if op == "Tj" {
			shown = append(shown, ops[0].text)
		}
	}
	if strings.Join(shown, ",") != "before,after" {
		t.Errorf("shown %v, want before,after (the image data skipped, an unterminated image ends the stream)", shown)
	}
}

func TestParserBoundsOperandsAndNesting(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("1 ")
	}
	sb.WriteString("op")
	p := newParser([]byte(sb.String()))
	ops, op, _ := p.next()
	if op != "op" || len(ops) != maxOperands {
		t.Errorf("got %d operands for %s, want the last %d", len(ops), op, maxOperands)
	}

	deep := strings.Repeat("[", 40) + "1" + strings.Repeat("]", 40) + " op"
	p = newParser([]byte(deep))
	ops, op, ok := p.next()
	if !ok || op != "op" || len(ops) != 1 {
		t.Errorf("deeply nested arrays: %v %q %d", ok, op, len(ops))
	}
}
