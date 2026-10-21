package pdf

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// document is a loaded PDF: parsed once by pdfcpu, its text extracted page
// by page as steps ask for it.
type document struct {
	source string // where it came from, for messages
	data   []byte
	ctx    *model.Context
	loader *fontLoader
	pages  int
	texts  map[int]string
}

// pdfcpu keeps a configuration directory by default; tomato has nothing to
// put there.
var disableConfigDir sync.Once

// loadDocument parses a PDF. source says where the bytes came from, for
// messages: `file "out/ca.pdf"`, `"uploads-storage" object "cas/x.pdf"`.
func loadDocument(c context.Context, data []byte, source string) (doc *document, err error) {
	disableConfigDir.Do(api.DisableConfigDir)
	defer recoverTo(&err, "reading the PDF from "+source)

	if !bytes.Contains(head(data, 1024), []byte("%PDF-")) {
		return nil, fmt.Errorf("%s is not a PDF: it starts with %q", source, string(head(data, 24)))
	}
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	ctx, err := api.ReadContext(c, bytes.NewReader(data), conf)
	if err != nil {
		return nil, fmt.Errorf("reading the PDF from %s: %w", source, err)
	}
	if err := ctx.EnsurePageCount(); err != nil {
		return nil, fmt.Errorf("counting the pages of %s: %w", source, err)
	}
	return &document{
		source: source,
		data:   data,
		ctx:    ctx,
		loader: newFontLoader(ctx.XRefTable),
		pages:  ctx.PageCount,
		texts:  map[int]string{},
	}, nil
}

func head(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}

// recoverTo turns a panic in pdfcpu or the extractor, which a damaged file
// can cause, into an error for the step.
func recoverTo(err *error, what string) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("%s: %v", what, r)
	}
}

// metadata returns the Info dictionary's entries, with the XMP metadata
// filling in what it lacks: PDF 2.0 and PDF/UA documents carry their title
// there.
func (d *document) metadata() (m map[string]string, err error) {
	defer recoverTo(&err, "reading the metadata")
	x := d.ctx.XRefTable
	m = map[string]string{}
	if x.Info != nil {
		if info, err := x.DereferenceDict(*x.Info); err == nil {
			for k, v := range info {
				if s, err := x.DereferenceText(v); err == nil {
					m[k] = s
				}
			}
		}
	}
	for k, v := range d.xmp() {
		if m[k] == "" {
			m[k] = v
		}
	}
	return m, nil
}

// xmpKeys maps XMP properties to the Info dictionary keys that mean the
// same, so `metadata "Title"` reads either.
var xmpKeys = map[string]string{
	"title": "Title", "creator": "Author", "description": "Subject", "Producer": "Producer",
	"Keywords": "Keywords", "CreatorTool": "Creator", "part": "pdfuaid:part",
}

// xmp reads the catalog's XMP metadata stream, element and attribute forms
// alike: dc:title's first rdf:li, pdf:Producer as an element or an attribute
// of rdf:Description.
func (d *document) xmp() map[string]string {
	out := map[string]string{}
	data := d.loader.stream(d.catalog()["Metadata"])
	if data == nil {
		return out
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var stack []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
			if t.Name.Local == "Description" {
				for _, a := range t.Attr {
					if k, ok := xmpKeys[a.Name.Local]; ok && out[k] == "" {
						out[k] = strings.TrimSpace(a.Value)
					}
				}
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text == "" || len(stack) == 0 {
				continue
			}
			name := stack[len(stack)-1]
			if name == "li" && len(stack) >= 3 {
				name = stack[len(stack)-3] // dc:title > rdf:Alt > rdf:li
			}
			if k, ok := xmpKeys[name]; ok && out[k] == "" {
				out[k] = text
			}
		}
	}
}

func (d *document) catalog() types.Dict {
	cat, err := d.ctx.XRefTable.Catalog()
	if err != nil {
		return types.Dict{}
	}
	return cat
}

// language returns the catalog's Lang entry, the document's natural
// language as a BCP 47 tag.
func (d *document) language() (lang string, err error) {
	defer recoverTo(&err, "reading the language")
	o := d.catalog()["Lang"]
	if o == nil {
		return "", nil
	}
	s, err := d.ctx.XRefTable.DereferenceText(o)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

// tagged reports whether the document declares itself tagged (MarkInfo
// Marked) and has a structure tree to show for it.
func (d *document) tagged() (marked, structured bool, err error) {
	defer recoverTo(&err, "reading the structure tree")
	cat := d.catalog()
	if mi := d.loader.dict(cat["MarkInfo"]); mi != nil {
		if b, ok := d.loader.deref(mi["Marked"]).(types.Boolean); ok {
			marked = b.Value()
		}
	}
	structured = d.loader.dict(cat["StructTreeRoot"]) != nil
	return marked, structured, nil
}

// outline returns the titles of the document outline (its bookmarks), depth
// first, children after their parent.
func (d *document) outline(c context.Context) (titles []string, err error) {
	defer recoverTo(&err, "reading the outline")
	root := d.loader.dict(d.catalog()["Outlines"])
	if root == nil {
		return nil, nil
	}
	seen := map[string]bool{}
	var walk func(item types.Object, depth int)
	walk = func(item types.Object, depth int) {
		for i := 0; item != nil && i < 10000; i++ {
			if k := key(item); k != "" {
				if seen[k] {
					return
				}
				seen[k] = true
			}
			node := d.loader.dict(item)
			if node == nil {
				return
			}
			if t, err := d.ctx.XRefTable.DereferenceText(node["Title"]); err == nil {
				titles = append(titles, strings.TrimSpace(t))
			}
			if depth < 32 {
				walk(node["First"], depth+1)
			}
			item = node["Next"]
		}
	}
	walk(root["First"], 0)
	return titles, nil
}

// pageText returns the text of one page, 1-based.
func (d *document) pageText(c context.Context, n int) (text string, err error) {
	if n < 1 || n > d.pages {
		return "", fmt.Errorf("page %d does not exist: the document has %d pages", n, d.pages)
	}
	if t, ok := d.texts[n]; ok {
		return t, nil
	}
	defer recoverTo(&err, fmt.Sprintf("extracting the text of page %d", n))
	x := d.ctx.XRefTable
	pd, _, _, err := x.PageDict(c, n, false)
	if err != nil {
		return "", fmt.Errorf("page %d: %w", n, err)
	}
	content, err := x.PageContent(pd, n)
	if err != nil && !errors.Is(err, model.ErrNoContent) {
		return "", fmt.Errorf("page %d content: %w", n, err)
	}
	res := newPageResources(d.loader, d.resources(pd), nil)
	text = extractText(content, res)
	d.texts[n] = text
	return text, nil
}

// resources finds a page's resources, inherited from its ancestors when
// the page has none of its own.
func (d *document) resources(page types.Dict) types.Dict {
	node := page
	for i := 0; node != nil && i < 64; i++ {
		if r := d.loader.dict(node["Resources"]); r != nil {
			return r
		}
		node = d.loader.dict(node["Parent"])
	}
	return types.Dict{}
}

// text returns the text of the whole document, pages in order.
func (d *document) text(c context.Context) (string, error) {
	var sb strings.Builder
	for n := 1; n <= d.pages; n++ {
		t, err := d.pageText(c, n)
		if err != nil {
			return "", err
		}
		if t == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(t)
	}
	return sb.String(), nil
}

// reader returns the document's bytes, for the validator.
func (d *document) reader() io.Reader { return bytes.NewReader(d.data) }
