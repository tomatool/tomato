package pdffixture

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/encoding/charmap"
)

// Accessible is a one-page tagged PDF written by hand to conform to PDF/UA-1
// (ISO 14289-1): every piece of content is marked and sits in a structure
// tree under a Document element, the catalog carries the language and
// MarkInfo, the viewer is told to display the title, the XMP metadata
// declares pdfuaid:part 1 and the title, and the one font, Go Regular
// (BSD-3, from golang.org/x/image), is embedded with widths taken from its
// own tables. No PDF writer in Go produces a tagged document; this is the
// least one that veraPDF accepts.
func Accessible() []byte {
	face, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		panic("pdffixture: " + err.Error())
	}
	var buf sfnt.Buffer
	upem := float64(face.UnitsPerEm())
	ppem := fixed.Int26_6(int(face.UnitsPerEm()) << 6)
	scaled := func(v fixed.Int26_6) int { return int(float64(v>>6)*1000/upem + 0.5) }
	scaledNeg := func(v fixed.Int26_6) int { return -scaled(-v) }

	// Widths of the WinAnsi codes 32..255, from the font's advances.
	notdef, _ := face.GlyphAdvance(&buf, 0, ppem, font.HintingNone)
	widths := make([]string, 0, 224)
	var toUnicode strings.Builder
	for code := 32; code < 256; code++ {
		r := winAnsiRune(byte(code))
		w := notdef
		if r != 0 {
			if gi, err := face.GlyphIndex(&buf, r); err == nil && gi != 0 {
				w, _ = face.GlyphAdvance(&buf, gi, ppem, font.HintingNone)
			}
			fmt.Fprintf(&toUnicode, "<%02X> <%04X>\n", code, r)
		}
		widths = append(widths, fmt.Sprint(scaled(w)))
	}
	metrics, _ := face.Metrics(&buf, ppem, font.HintingNone)
	bounds, _ := face.Bounds(&buf, ppem, font.HintingNone)

	w := &writer{}
	w.header()

	const (
		catalog = iota + 1
		pages
		page
		contents
		structTreeRoot
		fontDict
		fontDescriptor
		fontFile
		metadata
		docElem
		parentTree
		info
		toUnicodeCMap
		firstTextElem // one structure element per block follows
	)

	blocks := []struct {
		tag  string
		size int
		y    int
		text string
	}{
		{"H1", 18, 760, "Kundenvereinbarung"},
		{"P", 11, 730, "Version 07.07 für Österreich – barrierefreie Fassung"},
		{"P", 11, 712, "9 Kryptowerte: Absichtlich frei gelassen."},
		{"P", 11, 694, "Diese Vereinbarung regelt die Geschäftsbeziehung zwischen der Bank und dem Kunden."},
	}

	var content bytes.Buffer
	var kids, nums []string
	for i, b := range blocks {
		fmt.Fprintf(&content, "/%s << /MCID %d >> BDC\nBT /F1 %d Tf 72 %d Td (%s) Tj ET\nEMC\n", b.tag, i, b.size, b.y, pdfString(winAnsiBytes(b.text)))
		kids = append(kids, fmt.Sprintf("%d 0 R", firstTextElem+i))
		nums = append(nums, fmt.Sprintf("%d 0 R", firstTextElem+i))
	}

	w.obj(catalog, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R /Lang (de-AT) /MarkInfo << /Marked true >> /StructTreeRoot %d 0 R /ViewerPreferences << /DisplayDocTitle true >> /Metadata %d 0 R >>", pages, structTreeRoot, metadata))
	w.obj(pages, fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R] /Count 1 >>", page))
	w.obj(page, fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 595.276 841.89] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R /StructParents 0 /Tabs /S >>", pages, fontDict, contents))
	w.stream(contents, "", content.Bytes())
	w.obj(structTreeRoot, fmt.Sprintf("<< /Type /StructTreeRoot /K [%d 0 R] /ParentTree %d 0 R /ParentTreeNextKey 1 >>", docElem, parentTree))
	w.obj(fontDict, fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /GoRegular /FirstChar 32 /LastChar 255 /Widths [%s] /Encoding /WinAnsiEncoding /FontDescriptor %d 0 R /ToUnicode %d 0 R >>", strings.Join(widths, " "), fontDescriptor, toUnicodeCMap))
	w.obj(fontDescriptor, fmt.Sprintf("<< /Type /FontDescriptor /FontName /GoRegular /Flags 32 /FontBBox [%d %d %d %d] /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV 80 /FontFile2 %d 0 R >>",
		scaledNeg(-bounds.Min.X), scaledNeg(bounds.Max.Y), scaled(bounds.Max.X), scaled(-bounds.Min.Y),
		scaled(metrics.Ascent), scaledNeg(metrics.Descent), scaled(metrics.CapHeight), fontFile))
	w.stream(fontFile, fmt.Sprintf("/Length1 %d /Filter /FlateDecode", len(goregular.TTF)), deflate(goregular.TTF))
	w.stream(metadata, "/Type /Metadata /Subtype /XML", []byte(xmp))
	w.obj(docElem, fmt.Sprintf("<< /Type /StructElem /S /Document /P %d 0 R /K [%s] >>", structTreeRoot, strings.Join(kids, " ")))
	w.obj(parentTree, fmt.Sprintf("<< /Nums [0 [%s]] >>", strings.Join(nums, " ")))
	w.obj(info, "<< /Title (Kundenvereinbarung) /Author (tomato) /Producer (tomato fixtures) /Creator (tomato) /CreationDate (D:20260102030405Z) /ModDate (D:20260102030405Z) >>")
	w.stream(toUnicodeCMap, "", []byte(cmapHeader+fmt.Sprintf("%d beginbfchar\n%sendbfchar\n", strings.Count(toUnicode.String(), "\n"), toUnicode.String())+cmapFooter))
	for i, b := range blocks {
		w.obj(firstTextElem+i, fmt.Sprintf("<< /Type /StructElem /S /%s /P %d 0 R /Pg %d 0 R /K [%d] >>", b.tag, docElem, page, i))
	}
	return w.finish(catalog, info)
}

const xmp = `<?xpacket begin="` + "\ufeff" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/" x:xmptk="tomato fixtures">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:pdfuaid="http://www.aiim.org/pdfua/ns/id/">
   <pdfuaid:part>1</pdfuaid:part>
  </rdf:Description>
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
   <dc:title><rdf:Alt><rdf:li xml:lang="x-default">Kundenvereinbarung</rdf:li></rdf:Alt></dc:title>
   <dc:creator><rdf:Seq><rdf:li>tomato</rdf:li></rdf:Seq></dc:creator>
  </rdf:Description>
  <rdf:Description rdf:about="" xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
   <pdf:Producer>tomato fixtures</pdf:Producer>
  </rdf:Description>
  <rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/">
   <xmp:CreatorTool>tomato</xmp:CreatorTool>
   <xmp:CreateDate>2026-01-02T03:04:05Z</xmp:CreateDate>
   <xmp:ModifyDate>2026-01-02T03:04:05Z</xmp:ModifyDate>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

const cmapHeader = `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def
1 begincodespacerange
<00> <FF>
endcodespacerange
`

const cmapFooter = `endcmap
CMapName currentdict /CMap defineresource pop
end
end
`

// winAnsiRune is the character a WinAnsiEncoding code stands for, 0 for an
// unused code. It is Windows-1252, with the two differences ISO 32000-1
// Annex D notes.
func winAnsiRune(code byte) rune {
	switch code {
	case 0xA0:
		return ' '
	case 0xAD:
		return '-'
	}
	r := charmap.Windows1252.DecodeByte(code)
	if r == '�' || r < 32 {
		return 0
	}
	return r
}

// winAnsiBytes encodes text in WinAnsiEncoding; the fixture uses only
// characters it has.
func winAnsiBytes(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		b, ok := charmap.Windows1252.EncodeRune(r)
		if !ok {
			panic(fmt.Sprintf("pdffixture: %q is not in WinAnsiEncoding", r))
		}
		out = append(out, b)
	}
	return out
}

// pdfString escapes bytes for a (literal string).
func pdfString(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		switch c {
		case '(', ')', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case '\r':
			sb.WriteString(`\r`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func deflate(b []byte) []byte {
	var out bytes.Buffer
	zw := zlib.NewWriter(&out)
	zw.Write(b)
	zw.Close()
	return out.Bytes()
}

// writer is the least PDF file writer: objects in order, a classic cross
// reference table, a trailer.
type writer struct {
	buf     bytes.Buffer
	offsets map[int]int
}

func (w *writer) header() {
	w.offsets = map[int]int{}
	w.buf.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
}

func (w *writer) obj(n int, body string) {
	w.offsets[n] = w.buf.Len()
	fmt.Fprintf(&w.buf, "%d 0 obj\n%s\nendobj\n", n, body)
}

func (w *writer) stream(n int, extra string, data []byte) {
	w.offsets[n] = w.buf.Len()
	fmt.Fprintf(&w.buf, "%d 0 obj\n<< /Length %d %s >>\nstream\n", n, len(data), extra)
	w.buf.Write(data)
	w.buf.WriteString("\nendstream\nendobj\n")
}

func (w *writer) finish(root, info int) []byte {
	size := 0
	for n := range w.offsets {
		if n+1 > size {
			size = n + 1
		}
	}
	xref := w.buf.Len()
	fmt.Fprintf(&w.buf, "xref\n0 %d\n0000000000 65535 f \n", size)
	for n := 1; n < size; n++ {
		fmt.Fprintf(&w.buf, "%010d 00000 n \n", w.offsets[n])
	}
	fmt.Fprintf(&w.buf, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R /ID [<746f6d61746f206669787475726520> <746f6d61746f206669787475726520>] >>\nstartxref\n%d\n%%%%EOF\n", size, root, info, xref)
	return w.buf.Bytes()
}
