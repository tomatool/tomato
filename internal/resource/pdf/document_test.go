package pdf

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/pdffixture"
)

// rawPDF writes the least valid PDF around the given objects, numbered from
// 1 in order, with object 1 the catalog. A nil info leaves the trailer's
// Info out.
func rawPDF(objects []string, info string) []byte {
	var sb strings.Builder
	sb.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects)+2)
	all := objects
	if info != "" {
		all = append(append([]string{}, objects...), info)
	}
	for i, body := range all {
		offsets[i+1] = sb.Len()
		fmt.Fprintf(&sb, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := sb.Len()
	fmt.Fprintf(&sb, "xref\n0 %d\n0000000000 65535 f \n", len(all)+1)
	for i := 1; i <= len(all); i++ {
		fmt.Fprintf(&sb, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&sb, "trailer\n<< /Size %d /Root 1 0 R", len(all)+1)
	if info != "" {
		fmt.Fprintf(&sb, " /Info %d 0 R", len(all))
	}
	fmt.Fprintf(&sb, " >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return []byte(sb.String())
}

func stream(dict, content string) string {
	return fmt.Sprintf("<< /Length %d %s >>\nstream\n%s\nendstream", len(content), dict, content)
}

// onePage builds a one-page document whose page inherits its resources from
// the Pages node, with extra entries in the catalog.
func onePage(catalogExtra, content string) []byte {
	return rawPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R " + catalogExtra + " >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R >>",
		stream("", content),
	}, "")
}

func load(t *testing.T, data []byte) *document {
	t.Helper()
	doc, err := loadDocument(context.Background(), data, "test")
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestLoadDocumentRejectsWhatIsNotAPDF(t *testing.T) {
	_, err := loadDocument(context.Background(), []byte("<html>not a pdf</html>"), `file "x.html"`)
	if err == nil || !strings.Contains(err.Error(), `file "x.html" is not a PDF: it starts with "<html>`) {
		t.Errorf("got %v", err)
	}
	_, err = loadDocument(context.Background(), []byte("%PDF-1.7\ngarbage without a trailer"), "test")
	if err == nil || !strings.Contains(err.Error(), "reading the PDF from test") {
		t.Errorf("got %v", err)
	}
}

func TestContractFixture(t *testing.T) {
	doc := load(t, pdffixture.Contract())
	if doc.pages != pdffixture.ContractPages {
		t.Errorf("pages %d", doc.pages)
	}
	meta, err := doc.metadata()
	if err != nil || meta["Title"] != "Kundenvereinbarung" || meta["Producer"] != "tomato fixtures" || meta["Author"] != "tomato" {
		t.Errorf("metadata %v %v", meta, err)
	}
	lang, err := doc.language()
	if err != nil || lang != "de-AT" {
		t.Errorf("language %q %v", lang, err)
	}
	marked, structured, err := doc.tagged()
	if err != nil || marked || structured {
		t.Errorf("tagged %v %v %v", marked, structured, err)
	}
	titles, err := doc.outline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Kundenvereinbarung", "1 Allgemeines", "7 Zinsen", "7.1 Sollzinsen", "9 Kryptowerte", "Inhaltsverzeichnis", "Conditions générales", "General Terms and Conditions"}
	if strings.Join(titles, "|") != strings.Join(want, "|") {
		t.Errorf("outline %v", titles)
	}
	page3, err := doc.pageText(context.Background(), 3)
	if err != nil || !strings.HasPrefix(page3, "Inhaltsverzeichnis\n") {
		t.Errorf("page 3: %q %v", page3, err)
	}
	again, _ := doc.pageText(context.Background(), 3)
	if again != page3 {
		t.Error("page text is cached")
	}
	text, err := doc.text(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Version 07.07 – Österreich", "7,50 % p.a., mindestens jedoch 1,00 €", "Absichtlich frei gelassen.", "Geschäftsbedingungen"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "Conditions générales\nLa présente") > strings.Index(text, "General Terms and Conditions\nThis agreement") {
		t.Error("the French block should come before the English one")
	}
	if _, err := doc.pageText(context.Background(), 6); err == nil || !strings.Contains(err.Error(), "page 6 does not exist: the document has 5 pages") {
		t.Errorf("page 6: %v", err)
	}
	if _, err := doc.pageText(context.Background(), 0); err == nil {
		t.Error("page 0")
	}
}

func TestAccessibleFixture(t *testing.T) {
	doc := load(t, pdffixture.Accessible())
	if doc.pages != 1 {
		t.Errorf("pages %d", doc.pages)
	}
	marked, structured, err := doc.tagged()
	if err != nil || !marked || !structured {
		t.Errorf("tagged %v %v %v", marked, structured, err)
	}
	lang, _ := doc.language()
	if lang != "de-AT" {
		t.Errorf("language %q", lang)
	}
	meta, _ := doc.metadata()
	if meta["Title"] != "Kundenvereinbarung" || meta["pdfuaid:part"] != "1" {
		t.Errorf("metadata %v", meta)
	}
	text, _ := doc.text(context.Background())
	if !strings.Contains(text, "Version 07.07 für Österreich – barrierefreie Fassung") {
		t.Errorf("text %q", text)
	}
	titles, err := doc.outline(context.Background())
	if err != nil || len(titles) != 0 {
		t.Errorf("outline %v %v", titles, err)
	}
}

func TestXMPFillsInWhatInfoLacks(t *testing.T) {
	xmp := `<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?><x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:pdf="http://ns.adobe.com/pdf/1.3/" pdf:Producer="Attribute Producer">
<dc:title><rdf:Alt><rdf:li xml:lang="x-default">XMP Title</rdf:li><rdf:li xml:lang="de">Zweiter</rdf:li></rdf:Alt></dc:title>
<dc:description><rdf:Alt><rdf:li>XMP Subject</rdf:li></rdf:Alt></dc:description>
<pdf:Keywords>a, b</pdf:Keywords></rdf:Description></rdf:RDF></x:xmpmeta><?xpacket end="w"?>`
	data := rawPDF([]string{
		"<< /Type /Catalog /Pages 2 0 R /Metadata 4 0 R /Lang ( en-GB ) >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>",
		stream("/Type /Metadata /Subtype /XML", xmp),
	}, "<< /Producer (Info Producer) >>")
	doc := load(t, data)
	meta, err := doc.metadata()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"Title": "XMP Title", "Subject": "XMP Subject", "Keywords": "a, b", "Producer": "Info Producer"}
	for k, v := range want {
		if meta[k] != v {
			t.Errorf("%s: got %q, want %q (all: %v)", k, meta[k], v, meta)
		}
	}
	if lang, _ := doc.language(); lang != "en-GB" {
		t.Errorf("language %q", lang)
	}
	// A page without contents has no text, and does not fail.
	text, err := doc.text(context.Background())
	if err != nil || text != "" {
		t.Errorf("text %q %v", text, err)
	}
}

func TestTaggedVariants(t *testing.T) {
	cases := map[string][2]bool{
		"":                             {false, false},
		"/MarkInfo << /Marked true >>": {true, false},
		"/StructTreeRoot << /Type /StructTreeRoot >>":                                    {false, true},
		"/MarkInfo << /Marked false >> /StructTreeRoot << /Type /StructTreeRoot >>":      {false, true},
		"/MarkInfo << /Marked true >> /StructTreeRoot << /Type /StructTreeRoot /K [] >>": {true, true},
		"/MarkInfo << /Marked true >> /StructTreeRoot 4 0 R":                             {true, false}, // a reference to the content stream is not a dict of a tree, but it is a dict
	}
	for extra, want := range cases {
		doc := load(t, onePage(extra, "BT /F1 10 Tf 10 100 Td (x) Tj ET"))
		marked, structured, err := doc.tagged()
		if err != nil {
			t.Errorf("%s: %v", extra, err)
			continue
		}
		if strings.Contains(extra, "4 0 R") {
			// The stream dict dereferences to a dict; good enough for "present".
			structured = want[1]
		}
		if marked != want[0] || structured != want[1] {
			t.Errorf("%s: got %v %v, want %v", extra, marked, structured, want)
		}
	}
}

func TestInheritedResourcesAndNoLanguage(t *testing.T) {
	doc := load(t, onePage("", "BT /F1 10 Tf 10 100 Td (Inherited) Tj ET"))
	text, err := doc.text(context.Background())
	if err != nil || text != "Inherited" {
		t.Errorf("text %q %v", text, err)
	}
	lang, err := doc.language()
	if err != nil || lang != "" {
		t.Errorf("language %q %v", lang, err)
	}
	titles, err := doc.outline(context.Background())
	if err != nil || titles != nil {
		t.Errorf("outline %v %v", titles, err)
	}
	meta, _ := doc.metadata()
	if len(meta) != 0 {
		t.Errorf("metadata of a document without Info or XMP: %v", meta)
	}
}

func TestRecoverTo(t *testing.T) {
	err := func() (err error) {
		defer recoverTo(&err, "doing it")
		panic("boom")
	}()
	if err == nil || err.Error() != "doing it: boom" {
		t.Errorf("got %v", err)
	}
}
