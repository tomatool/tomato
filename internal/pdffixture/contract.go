// Package pdffixture generates the PDF documents tomato's own tests assert
// on: the unit tests of the pdf resource, the test app that serves them,
// and the committed copies under tests/testdata/pdf. Every document is
// deterministic, so a test can compare bytes and the committed files are
// checked against the generator.
package pdffixture

import (
	"bytes"
	"time"

	"github.com/go-pdf/fpdf"
)

// fixedTime is the only timestamp the documents carry.
var fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// ContractPages is the page count of Contract.
const ContractPages = 5

// Contract is a bilingual customer agreement of the kind a contract
// generator produces: five pages, a title, an outline with a nested entry,
// metadata, the German block before the French and the English one, and a
// clause kept with its number and printed as intentionally left blank. It
// uses the standard Helvetica fonts with WinAnsi encoding, as fpdf and TCPDF
// documents do, with an en dash and a euro sign from the high half.
func Contract() []byte {
	p := fpdf.New("P", "mm", "A4", "")
	// fpdf writes its font and resource catalogs in map order unless told
	// to sort them; the committed copy is compared byte for byte.
	p.SetCatalogSort(true)
	p.SetCreationDate(fixedTime)
	p.SetModificationDate(fixedTime)
	p.SetTitle("Kundenvereinbarung", true)
	p.SetAuthor("tomato", true)
	p.SetSubject("Kundenvereinbarung Version 07.07", true)
	p.SetProducer("tomato fixtures", true)
	p.SetCreator("tomato", true)
	p.SetLang("de-AT")
	tr := p.UnicodeTranslatorFromDescriptor("")

	heading := func(text string, level int) {
		p.SetFont("Helvetica", "B", 14)
		p.Bookmark(tr(text), level, -1)
		p.CellFormat(0, 9, tr(text), "", 1, "L", false, 0, "")
		p.SetFont("Helvetica", "", 11)
	}
	para := func(text string) {
		p.MultiCell(0, 6, tr(text), "", "L", false)
		p.Ln(3)
	}

	// Page 1: the title page.
	p.AddPage()
	p.SetFont("Helvetica", "B", 20)
	p.Bookmark("Kundenvereinbarung", 0, -1)
	p.CellFormat(0, 12, "Kundenvereinbarung", "", 1, "L", false, 0, "")
	// Two runs on one baseline, in two styles: the text reads as one line.
	p.SetFont("Helvetica", "", 11)
	p.CellFormat(p.GetStringWidth("Version ")+1, 6, "Version ", "", 0, "L", false, 0, "")
	p.SetFont("Helvetica", "B", 11)
	p.CellFormat(0, 6, tr("07.07 – Österreich"), "", 1, "L", false, 0, "")
	p.SetFont("Helvetica", "", 11)
	p.Ln(4)
	para("Allgemeine Geschäftsbedingungen der Beispielbank AG für Kundinnen und Kunden in Österreich.")

	// Page 2: numbered clauses, one of them deselected but kept.
	p.AddPage()
	heading("1 Allgemeines", 0)
	para("Diese Vereinbarung regelt die Geschäftsbeziehung zwischen der Bank und dem Kunden.")
	heading("7 Zinsen", 0)
	heading("7.1 Sollzinsen", 1)
	para("Der Sollzinssatz beträgt 7,50 % p.a., mindestens jedoch 1,00 € pro Abrechnungsperiode.")
	heading("9 Kryptowerte", 0)
	para("Absichtlich frei gelassen.")

	// Page 3: the table of contents.
	p.AddPage()
	heading("Inhaltsverzeichnis", 0)
	for _, line := range []string{"1 Allgemeines", "7 Zinsen", "7.1 Sollzinsen", "9 Kryptowerte", "Conditions générales", "General Terms and Conditions"} {
		p.CellFormat(0, 6, tr(line), "", 1, "L", false, 0, "")
	}

	// Pages 4 and 5: the other languages, each a full block after the last.
	p.AddPage()
	heading("Conditions générales", 0)
	para("La présente convention régit la relation d'affaires entre la banque et le client.")
	p.AddPage()
	heading("General Terms and Conditions", 0)
	para("This agreement governs the business relationship between the bank and the customer.")

	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		panic("pdffixture: " + err.Error())
	}
	return buf.Bytes()
}
