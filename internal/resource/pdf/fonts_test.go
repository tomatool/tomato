package pdf

import (
	"context"
	"testing"
)

// pageWith builds a one-page document from a content stream, a /Font
// resource dictionary and further objects numbered from 5.
func pageWith(content, fonts string, extra ...string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 600 800] /Resources << /Font << " + fonts + " >> /XObject << /Fm1 6 0 R /Im1 7 0 R >> >> /Contents 4 0 R >>",
		stream("", content),
	}
	return rawPDF(append(objects, extra...), "")
}

func textOf(t *testing.T, data []byte) string {
	t.Helper()
	doc := load(t, data)
	text, err := doc.text(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestLoaderCompositeFontWidths(t *testing.T) {
	// Identity-H, widths from W (both forms) and DW, text from ToUnicode.
	// "Kunden" is 6 × 500 wide at size 10 = 30: a run 34 along is a word
	// gap; the dash is DW = 1000 wide, so a run 11 along is not.
	content := `BT /F1 10 Tf 72 700 Td <004B0075006E00640065006E> Tj 34 0 Td <00560065007200730069006F006E> Tj ET
BT /F1 10 Tf 72 680 Td <2013> Tj 11 0 Td <0078> Tj ET`
	data := pageWith(content, "/F1 5 0 R",
		"<< /Type /Font /Subtype /Type0 /BaseFont /ABCDEF+OpenSans /Encoding /Identity-H /DescendantFonts [8 0 R] /ToUnicode 9 0 R >>",
		"<< >>", "<< >>",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ABCDEF+OpenSans /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /DW 1000 /W [32 [250] 65 122 500 1 2] >>",
		stream("", identityToUnicode))
	if got := textOf(t, data); got != "Kunden Version\n–x" {
		t.Errorf("got %q", got)
	}
}

func TestLoaderEmbeddedCMapEncoding(t *testing.T) {
	// A one-byte encoding CMap (as OneByteIdentityH generators write) with a
	// one-byte ToUnicode: <0102> is two codes, not one.
	encoding := "1 begincodespacerange <00> <FF> endcodespacerange 1 begincidrange <00> <FF> 0 endcidrange"
	toUnicode := "1 begincodespacerange <00> <FF> endcodespacerange 2 beginbfchar <01> <0048> <02> <0069> endbfchar"
	data := pageWith(`BT /F1 10 Tf 72 700 Td <0102> Tj ET`, "/F1 5 0 R",
		"<< /Type /Font /Subtype /Type0 /BaseFont /X /Encoding 8 0 R /DescendantFonts [9 0 R] /ToUnicode 10 0 R >>",
		"<< >>", "<< >>",
		stream("/Type /CMap /CMapName /OneByteIdentityH", encoding),
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /X /DW 500 >>",
		stream("", toUnicode))
	if got := textOf(t, data); got != "Hi" {
		t.Errorf("got %q", got)
	}
}

func TestLoaderCompositeFontWithoutDescendants(t *testing.T) {
	data := pageWith(`BT /F1 10 Tf 72 700 Td <0041> Tj ET`, "/F1 5 0 R",
		"<< /Type /Font /Subtype /Type0 /BaseFont /X /Encoding /UniGB-UCS2-H >>")
	if got := textOf(t, data); got != undecodable {
		t.Errorf("a composite font without ToUnicode reads as undecodable glyphs, got %q", got)
	}
}

func TestLoaderDifferences(t *testing.T) {
	// Letters by their own names, a ligature, a uniXXXX name, a glyph index
	// name nobody can read, and a base encoding behind them.
	data := pageWith(`BT /F1 10 Tf 72 700 Td (\001\002\003\004\005A) Tj ET`, "/F1 5 0 R",
		`<< /Type /Font /Subtype /Type1 /BaseFont /ABCDEF+Omnes /FirstChar 1 /LastChar 5 /Widths [600 400 300 700 250]
		   /Encoding << /Type /Encoding /BaseEncoding /WinAnsiEncoding /Differences [1 /O /fi /uni20AC /g5 /space] >>
		   /FontDescriptor << /Type /FontDescriptor /FontName /ABCDEF+Omnes /Flags 32 /MissingWidth 333 >> >>`)
	if got := textOf(t, data); got != "Oﬁ€"+undecodable+" A" {
		t.Errorf("got %q", got)
	}
}

func TestLoaderPredefinedEncodings(t *testing.T) {
	// 0x8A is ä in Mac OS Roman, Š in WinAnsi and unused in Standard; 0x27
	// is the straight quote everywhere but in Standard.
	cases := map[string]string{
		"/Encoding /MacRomanEncoding": "ä'",
		"/Encoding /StandardEncoding": undecodable + "’",
		"":                            "Š'",
	}
	for enc, want := range cases {
		data := pageWith(`BT /F1 10 Tf 72 700 Td (\212') Tj ET`, "/F1 5 0 R",
			"<< /Type /Font /Subtype /TrueType /BaseFont /Foo "+enc+" /FontDescriptor << /Type /FontDescriptor /FontName /Foo /Flags 32 >> >>")
		if got := textOf(t, data); got != want {
			t.Errorf("%q: got %q, want %q", enc, got, want)
		}
	}
	// A symbolic TrueType font with no encoding: the bytes are read as
	// StandardEncoding, a best effort that fits most.
	data := pageWith(`BT /F1 10 Tf 72 700 Td (Hi) Tj ET`, "/F1 5 0 R",
		"<< /Type /Font /Subtype /TrueType /BaseFont /ABCDEF+Wingdings /FontDescriptor << /Type /FontDescriptor /FontName /ABCDEF+Wingdings /Flags 4 >> >>")
	if got := textOf(t, data); got != "Hi" {
		t.Errorf("symbolic: got %q", got)
	}
}

func TestLoaderType3Font(t *testing.T) {
	data := pageWith(`BT /F1 10 Tf 72 700 Td (ab) Tj 8 0 Td (a) Tj ET`, "/F1 5 0 R",
		`<< /Type /Font /Subtype /Type3 /FontBBox [0 0 750 750] /FontMatrix [0.005 0 0 0.005 0 0] /CharProcs << /sq 8 0 R >>
		   /Encoding << /Type /Encoding /Differences [97 /a /b] >> /FirstChar 97 /LastChar 98 /Widths [100 100] /Resources << >> >>`,
		"<< >>", "<< >>",
		stream("", "100 0 d0 0 0 50 50 re f"))
	// Each glyph is 100 × 0.005 = 0.5 text space units, 5 at size 10: "ab"
	// ends 10 along, the next run 8 along moved back inside it; no gap.
	if got := textOf(t, data); got != "aba" {
		t.Errorf("got %q", got)
	}
}

func TestLoaderCoreFontMetrics(t *testing.T) {
	// "Hello" in Helvetica is 722+556+222+222+556 = 2278 wide: 22.78 at
	// size 10. A run 24.5 along is past it with the real metrics (a word
	// gap) but inside it with a 500 default (no gap).
	data := pageWith(`BT /F1 10 Tf 72 700 Td (Hello) Tj 24.5 0 Td (World) Tj ET`, "/F1 5 0 R",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	if got := textOf(t, data); got != "Hello World" {
		t.Errorf("got %q", got)
	}
	for base, want := range map[string]string{"Helvetica": "Helvetica", "ABCDEF+Arial-BoldMT": "Helvetica-Bold", "TimesNewRomanPSMT": "Times-Roman", "Calibri": "", "ABCDEF+CourierNew": "Courier"} {
		if got := coreFontName(base); got != want {
			t.Errorf("coreFontName(%q) = %q, want %q", base, got, want)
		}
	}
	if w := coreWidth("Helvetica", "fi", 123); w != 123 {
		t.Errorf("a two-rune glyph has no core width, got %v", w)
	}
	if w := coreWidth("Helvetica", "H", 0); w != 722 {
		t.Errorf("H in Helvetica is 722 wide, got %v", w)
	}
}

func TestLoaderFormXObjects(t *testing.T) {
	// A form with its own font and matrix, a form without resources that
	// uses the page's, and an image, which draws no text.
	content := `/Fm1 Do BT /F1 10 Tf 72 600 Td (Page) Tj ET /Fm2 Do /Im1 Do /Nope Do`
	data := rawPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 600 800] /Resources << /Font << /F1 5 0 R >> /XObject << /Fm1 6 0 R /Fm2 8 0 R /Im1 7 0 R >> >> /Contents 4 0 R >>",
		stream("", content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		stream("/Type /XObject /Subtype /Form /BBox [0 0 100 100] /Matrix [1 0 0 1 72 700] /Resources << /Font << /F2 9 0 R >> >>", "BT /F2 10 Tf 0 0 Td (In form) Tj ET"),
		stream("/Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8", "\x00"),
		stream("/Type /XObject /Subtype /Form /BBox [0 0 100 100]", "BT /F1 10 Tf 72 500 Td (Shared font) Tj ET"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>",
	}, "")
	if got := textOf(t, data); got != "In form\nPage\nShared font" {
		t.Errorf("got %q", got)
	}
	// The form is built once and reused.
	doc := load(t, data)
	doc.text(context.Background())
	if len(doc.loader.forms) != 2 || len(doc.loader.fonts) != 2 {
		t.Errorf("cached %d forms and %d fonts", len(doc.loader.forms), len(doc.loader.fonts))
	}
}

func TestLoaderToleratesMalformedFonts(t *testing.T) {
	// A font that is not a dictionary, a W array that stops short, widths
	// that are not numbers, a ToUnicode that is not a stream.
	content := `BT /F1 10 Tf 72 700 Td (a) Tj /F2 10 Tf <0041> Tj /F3 10 Tf (b) Tj ET`
	data := pageWith(content, "/F1 5 0 R /F2 8 0 R /F3 9 0 R",
		"(not a font)",
		"<< >>", "<< >>",
		"<< /Type /Font /Subtype /Type0 /BaseFont /X /Encoding /Identity-H /DescendantFonts [10 0 R] /ToUnicode 11 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /FirstChar 97 /Widths [(x) 500] /ToUnicode 5 0 R >>",
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /X /W [65 66] /DW (x) >>",
		"<< /Length 0 >>")
	if got := textOf(t, data); got != undecodable+"b" {
		t.Errorf("got %q", got)
	}
}

func TestEncodingTablesAndGlyphNames(t *testing.T) {
	if n := len([]rune(winAnsiHigh)); n != 128 {
		t.Errorf("winAnsiHigh has %d runes, want 128", n)
	}
	if n := len([]rune(macRomanHigh)); n != 128 {
		t.Errorf("macRomanHigh has %d runes, want 128", n)
	}
	if winAnsiEncoding[0x80] != '€' || winAnsiEncoding[0xA0] != ' ' || winAnsiEncoding[0xFF] != 'ÿ' || macRomanEncoding[0xDB] != '€' || standardEncoding[0x27] != '’' || standardEncoding[0xFB] != 'ß' {
		t.Error("encoding tables are off")
	}
	if baseEncoding("MacExpertEncoding") != &standardEncoding || baseEncoding("Nope") != nil {
		t.Error("baseEncoding")
	}
	cases := map[string]string{"uni00660069": "fi", "a.sc": "a", "fi.alt": "ﬁ", "u1F600": "😀", "Euro": "€", "O": "O", ".notdef": "", "uni20AC": "€"}
	for name, want := range cases {
		got, ok := glyphText(name)
		if !ok || got != want {
			t.Errorf("glyphText(%q) = %q %v, want %q", name, got, ok, want)
		}
	}
	for _, name := range []string{"g123", "cid45", "G7", "uniXYZW", "unknownglyph", "xyz.alt"} {
		if got, ok := glyphText(name); ok {
			t.Errorf("glyphText(%q) = %q, want unknown", name, got)
		}
	}
}
