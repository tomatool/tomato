package pdf

import (
	"strings"
	"testing"
)

type fakeResources struct {
	fonts map[string]*font
	forms map[string]*form
}

func (r fakeResources) font(name string) *font { return r.fonts[name] }
func (r fakeResources) form(name string) *form { return r.forms[name] }

// helvetica is a simple WinAnsi font with uniform widths: letters 500,
// space 250.
func helvetica() *font {
	var table [256]string
	for c, r := range winAnsiEncoding {
		if r != 0 {
			table[c] = string(r)
		}
	}
	return &font{name: "Helvetica", simple: &table, defaultWidth: 500, widths: map[uint32]float64{32: 250}}
}

func extract(t *testing.T, content string, res resourceSource) string {
	t.Helper()
	return extractText([]byte(content), res)
}

func TestExtractLinesAndWords(t *testing.T) {
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}}
	content := `BT /F1 10 Tf 72 700 Td (Kunden) Tj (vereinbarung) Tj ET
BT /F1 10 Tf 72 686 Td (Version) Tj 40 0 Td (07.07) Tj ET
BT /F1 10 Tf 72 672 Td [(A) -300 (B) -20 (C)] TJ ET
BT /F1 10 Tf 72 672 Td ET`
	got := extract(t, content, res)
	want := "Kundenvereinbarung\nVersion 07.07\nA BC"
	// "Version" is 7 glyphs × 5 = 35 wide; the next run starts 40 along: a gap.
	// -300 in a TJ moves 3 units, a word gap; -20 moves 0.2, kerning.
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractKerningIsNotASpace(t *testing.T) {
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}}
	// Word writes one run per kerning pair with tiny adjustments.
	got := extract(t, `BT /F1 12 Tf 1 0 0 1 72 700 Tm [(DOORD) 12 (ASH,) -5 ( INC.)] TJ ET`, res)
	if got != "DOORDASH, INC." {
		t.Errorf("got %q", got)
	}
}

func TestExtractNewLineOperators(t *testing.T) {
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}}
	content := `BT /F1 10 Tf 14 TL 72 700 Td (one) Tj T* (two) Tj (three) ' 2 1 (four) " 0 -14 TD (five) Tj ET`
	got := extract(t, content, res)
	if got != "one\ntwo\nthree\nfour\nfive" {
		t.Errorf("got %q", got)
	}
}

func TestExtractSuperscriptStaysOnTheLine(t *testing.T) {
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}}
	got := extract(t, `BT /F1 10 Tf 72 700 Td (m) Tj 4 Ts /F1 6 Tf (2) Tj 0 Ts /F1 10 Tf ( of space) Tj ET`, res)
	if got != "m2 of space" {
		t.Errorf("got %q", got)
	}
}

func TestExtractRotatedTextReadsAlongItsBaseline(t *testing.T) {
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}}
	// 90° rotation: the baseline runs up the page.
	content := `BT /F1 10 Tf 0 1 -1 0 300 100 Tm (Seite) Tj 30 0 Td (1) Tj ET
BT /F1 10 Tf 0 1 -1 0 280 100 Tm (von 2) Tj ET`
	got := extract(t, content, res)
	if got != "Seite 1\nvon 2" {
		t.Errorf("got %q", got)
	}
}

func TestExtractFormXObjects(t *testing.T) {
	letterhead := &form{id: "9.0", content: []byte(`BT /F1 8 Tf 0 0 Td (Beispielbank AG) Tj ET`), matrix: identity}
	letterhead.res = nil // draws with the page's resources
	self := &form{id: "10.0", content: []byte(`BT /F1 8 Tf 0 0 Td (loop) Tj ET /Self Do`), matrix: identity}
	res := fakeResources{
		fonts: map[string]*font{"F1": helvetica()},
		forms: map[string]*form{"Head": letterhead, "Self": self},
	}
	got := extract(t, `q 1 0 0 1 72 800 cm /Head Do Q BT /F1 10 Tf 72 700 Td (Body) Tj ET /Self Do /Missing Do`, res)
	if got != "Beispielbank AG\nBody\nloop" {
		t.Errorf("got %q", got)
	}
}

func TestExtractFormDepthIsBounded(t *testing.T) {
	// Forms nested deeper than maxFormDepth, each with its own id.
	forms := map[string]*form{}
	for i := 0; i < maxFormDepth+5; i++ {
		forms["F"+string(rune('a'+i))] = &form{id: string(rune('a' + i)), matrix: identity,
			content: []byte(`BT /F1 8 Tf 0 -10 Td (x) Tj ET /F` + string(rune('a'+i+1)) + ` Do`)}
	}
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}, forms: forms}
	got := extract(t, `/Fa Do`, res)
	if n := strings.Count(got, "x"); n != maxFormDepth {
		t.Errorf("drew %d levels, want %d", n, maxFormDepth)
	}
}

func TestExtractGraphicsStateAndSpacing(t *testing.T) {
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}}
	// Tz scales advances; Tw widens spaces; q/Q restores the matrix so the
	// second run lands on the first baseline again.
	content := `q 2 0 0 2 0 0 cm BT /F1 10 Tf 50 Tz 5 Tw 1 Tc 36 350 Td (a b) Tj ET Q BT /F1 10 Tf 72 700 Td (c) Tj ET`
	got := extract(t, content, res)
	if got != "a b c" {
		t.Errorf("got %q", got)
	}
	// Operands beyond the operator's arity: the last ones count.
	got = extract(t, `BT 1 2 /F1 10 Tf 9 9 72 700 Td (ok) Tj ET`, res)
	if got != "ok" {
		t.Errorf("extra operands: got %q", got)
	}
}

func TestExtractWithoutFontOrResources(t *testing.T) {
	if got := extract(t, `BT /F1 10 Tf (lost) Tj ET`, fakeResources{}); got != "" {
		t.Errorf("an unknown font shows nothing, got %q", got)
	}
	if got := extract(t, `BT (lost) Tj ET`, nil); got != "" {
		t.Errorf("no resources at all, got %q", got)
	}
	if got := extract(t, `BT /F1 10 Tf () Tj ET /X Do`, nil); got != "" {
		t.Errorf("nil resources with Do, got %q", got)
	}
}

func TestExtractCompositeFont(t *testing.T) {
	f := &font{name: "Open Sans", composite: true, identity: true, defaultWidth: 1000,
		toUnicode: parseCMap([]byte(identityToUnicode)), widths: map[uint32]float64{0x20: 250}}
	res := fakeResources{fonts: map[string]*font{"F1": f}}
	got := extract(t, `BT /F1 10 Tf 72 700 Td <004B0075006E00640065006E00202013> Tj ET`, res)
	if got != "Kunden –" {
		t.Errorf("got %q", got)
	}
}

func TestExtractUndecodableGlyphs(t *testing.T) {
	f := &font{name: "Icons", composite: true, identity: true, defaultWidth: 1000}
	res := fakeResources{fonts: map[string]*font{"F1": f}}
	got := extract(t, `BT /F1 10 Tf 72 700 Td <00010002> Tj ET`, res)
	if got != undecodable+undecodable {
		t.Errorf("got %q", got)
	}
}

func TestExtractType3Widths(t *testing.T) {
	var table [256]string
	table['a'] = "a"
	// Glyph space is 1/100 here (FontMatrix 0.01), glyphs 60 units wide.
	f := &font{name: "T3", type3: true, scale: 10, simple: &table, widths: map[uint32]float64{'a': 60}}
	res := fakeResources{fonts: map[string]*font{"F1": f}}
	got := extract(t, `BT /F1 10 Tf 72 700 Td (a) Tj 8 0 Td (a) Tj ET`, res)
	if got != "a a" {
		t.Errorf("got %q (the glyph is 6 wide, the next starts at 8: a gap)", got)
	}
	got = extract(t, `BT /F1 10 Tf 72 700 Td (a) Tj 6.5 0 Td (a) Tj ET`, res)
	if got != "aa" {
		t.Errorf("got %q (half a point after a 6 wide glyph is kerning)", got)
	}
}

func TestExtractSameLineMovedBack(t *testing.T) {
	res := fakeResources{fonts: map[string]*font{"F1": helvetica()}}
	// A second column drawn after the first, on the same baseline but far left.
	got := extract(t, `BT /F1 10 Tf 300 700 Td (right) Tj -228 0 Td (left) Tj ET`, res)
	if got != "right left" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeText(t *testing.T) {
	got := normalizeText("  a   b  \n\n\t\n c\t d \n")
	if got != "a b\nc d" {
		t.Errorf("got %q", got)
	}
}

func TestMatrixMul(t *testing.T) {
	m := translation(10, 20).mul(matrix{2, 0, 0, 2, 5, 5})
	if m != (matrix{2, 0, 0, 2, 25, 45}) {
		t.Errorf("got %v", m)
	}
}
