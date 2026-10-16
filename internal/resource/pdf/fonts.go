package pdf

import (
	"context"
	"fmt"
	"strings"

	pdffont "github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// font is what the extractor needs of one font resource: how to cut a
// string into codes, what text each code stands for, and how wide each
// glyph is, so a gap between two runs can be told from kerning.
type font struct {
	name      string
	composite bool
	identity  bool  // a composite font with Identity-H or -V: two bytes per code
	encoding  *cmap // a composite font's embedded encoding CMap, code to CID
	toUnicode *cmap
	simple    *[256]string // a simple font's encoding with its differences applied
	type3     bool
	scale     float64 // Type3: FontMatrix[0] × 1000, the glyph space to text space factor

	widths       map[uint32]float64 // glyph space, by code (simple) or CID (composite)
	defaultWidth float64
	coreName     string // a standard 14 font without Widths: ask pdfcpu's metrics
}

// glyph is one shown character: its text, and its advance in glyph space
// (1/1000 of the text space unit).
type glyph struct {
	text  string
	width float64
	space bool // a single-byte code 32, which word spacing applies to
}

// undecodable stands for a glyph the font gives no text for: a composite
// font without a ToUnicode CMap, or a glyph named by its index only.
const undecodable = "�"

// decode cuts a string into glyphs.
func (f *font) decode(s []byte) []glyph {
	var codes []code
	switch {
	case !f.composite:
		codes = splitFixed(s, 1)
	case f.identity:
		codes = splitFixed(s, 2)
	case f.encoding != nil && len(f.encoding.codespaces) > 0:
		codes = f.encoding.split(s, 2)
	case f.toUnicode != nil && len(f.toUnicode.codespaces) > 0:
		codes = f.toUnicode.split(s, 2)
	default:
		codes = splitFixed(s, 2)
	}
	out := make([]glyph, 0, len(codes))
	for _, c := range codes {
		g := glyph{space: c.n == 1 && c.code == 32}
		cid := c.code
		if f.encoding != nil {
			if v, ok := f.encoding.cid(c.code, c.n); ok {
				cid = v
			}
		}
		found := false
		if f.toUnicode != nil {
			g.text, found = f.toUnicode.text(c.code, c.n)
		}
		if !found && f.simple != nil && c.n == 1 {
			g.text = f.simple[byte(c.code)]
			found = g.text != ""
		}
		if !found {
			g.text = undecodable
		}
		w, ok := f.widths[cid]
		switch {
		case ok:
		case f.coreName != "":
			w = coreWidth(f.coreName, g.text, f.defaultWidth)
		default:
			w = f.defaultWidth
		}
		if f.type3 {
			w *= f.scale
		}
		g.width = w
		out = append(out, g)
	}
	return out
}

// coreWidth is the advance of a standard 14 font's glyph from pdfcpu's
// metrics, for fonts that leave Widths out as the standard 14 may.
func coreWidth(coreName, text string, fallback float64) float64 {
	rs := []rune(text)
	if len(rs) != 1 {
		return fallback
	}
	w, err := pdffont.CharWidth(context.Background(), coreName, rs[0])
	if err != nil || w <= 0 {
		return fallback
	}
	return float64(w)
}

// coreFontAliases maps the names generators give the standard 14 fonts to
// the names pdfcpu's metrics know.
var coreFontAliases = map[string]string{
	"Arial": "Helvetica", "ArialMT": "Helvetica", "Arial-Bold": "Helvetica-Bold", "Arial-BoldMT": "Helvetica-Bold",
	"Arial-Italic": "Helvetica-Oblique", "Arial-ItalicMT": "Helvetica-Oblique", "Arial-BoldItalic": "Helvetica-BoldOblique",
	"Arial-BoldItalicMT": "Helvetica-BoldOblique", "Helvetica-Italic": "Helvetica-Oblique", "Helvetica-BoldItalic": "Helvetica-BoldOblique",
	"TimesNewRoman": "Times-Roman", "TimesNewRomanPSMT": "Times-Roman", "TimesNewRoman-Bold": "Times-Bold",
	"TimesNewRomanPS-BoldMT": "Times-Bold", "TimesNewRoman-Italic": "Times-Italic", "TimesNewRomanPS-ItalicMT": "Times-Italic",
	"TimesNewRoman-BoldItalic": "Times-BoldItalic", "TimesNewRomanPS-BoldItalicMT": "Times-BoldItalic", "Times": "Times-Roman",
	"CourierNew": "Courier", "CourierNewPSMT": "Courier", "CourierNew-Bold": "Courier-Bold", "CourierNewPS-BoldMT": "Courier-Bold",
	"CourierNew-Italic": "Courier-Oblique", "CourierNewPS-ItalicMT": "Courier-Oblique", "CourierNew-BoldItalic": "Courier-BoldOblique",
	"CourierNewPS-BoldItalicMT": "Courier-BoldOblique",
}

// coreFontName returns the standard 14 name a BaseFont stands for, or "".
func coreFontName(base string) string {
	if i := strings.IndexByte(base, '+'); i == 6 {
		base = base[7:] // a subset tag: ABCDEF+Helvetica
	}
	if alias, ok := coreFontAliases[base]; ok {
		base = alias
	}
	if pdffont.IsCoreFont(base) {
		return base
	}
	return ""
}

// fontLoader builds fonts from their dictionaries, once each.
type fontLoader struct {
	x     *model.XRefTable
	fonts map[string]*font
	forms map[string]*form
}

func newFontLoader(x *model.XRefTable) *fontLoader {
	return &fontLoader{x: x, fonts: map[string]*font{}, forms: map[string]*form{}}
}

func (l *fontLoader) deref(o types.Object) types.Object {
	if o == nil {
		return nil
	}
	v, err := l.x.Dereference(o)
	if err != nil {
		return nil
	}
	return v
}

func (l *fontLoader) dict(o types.Object) types.Dict {
	switch v := l.deref(o).(type) {
	case types.Dict:
		return v
	case types.StreamDict:
		return v.Dict
	case *types.StreamDict:
		return v.Dict
	}
	return nil
}

func (l *fontLoader) array(o types.Object) types.Array {
	a, _ := l.deref(o).(types.Array)
	return a
}

func (l *fontLoader) name(o types.Object) string {
	n, _ := l.deref(o).(types.Name)
	return n.Value()
}

func (l *fontLoader) number(o types.Object) (float64, bool) {
	switch v := l.deref(o).(type) {
	case types.Integer:
		return float64(v.Value()), true
	case types.Float:
		return v.Value(), true
	}
	return 0, false
}

// stream returns a stream's decoded content, or nil.
func (l *fontLoader) stream(o types.Object) []byte {
	if o == nil {
		return nil
	}
	sd, _, err := l.x.DereferenceStreamDict(o)
	if err != nil || sd == nil {
		return nil
	}
	if len(sd.Content) == 0 {
		if err := sd.Decode(); err != nil {
			return nil
		}
	}
	return sd.Content
}

// key identifies an object for caching: its reference, or "" for a direct
// object, which is built every time.
func key(o types.Object) string {
	if ref, ok := o.(types.IndirectRef); ok {
		return fmt.Sprintf("%d.%d", ref.ObjectNumber.Value(), ref.GenerationNumber.Value())
	}
	return ""
}

// load returns the font an entry of a /Font resource dictionary refers to.
func (l *fontLoader) load(o types.Object) *font {
	k := key(o)
	if k != "" {
		if f, ok := l.fonts[k]; ok {
			return f
		}
	}
	d := l.dict(o)
	if d == nil {
		return nil
	}
	f := l.build(d)
	if k != "" {
		l.fonts[k] = f
	}
	return f
}

func (l *fontLoader) build(d types.Dict) *font {
	f := &font{name: l.name(d["BaseFont"]), defaultWidth: 500}
	if data := l.stream(d["ToUnicode"]); data != nil {
		if m := parseCMap(data); m.hasMappings() {
			f.toUnicode = m
		}
	}
	subtype := l.name(d["Subtype"])
	switch subtype {
	case "Type0":
		l.buildComposite(f, d)
	case "Type3":
		f.type3 = true
		f.scale = 0.001 * 1000
		if m := l.array(d["FontMatrix"]); len(m) == 6 {
			if a, ok := l.number(m[0]); ok {
				f.scale = a * 1000
			}
		}
		f.simple = l.simpleEncoding(d, subtype, 0)
		l.simpleWidths(f, d)
	default: // Type1, MMType1, TrueType
		flags := 0
		if fd := l.dict(d["FontDescriptor"]); fd != nil {
			if v, ok := l.number(fd["Flags"]); ok {
				flags = int(v)
			}
		}
		f.simple = l.simpleEncoding(d, subtype, flags)
		l.simpleWidths(f, d)
		if f.widths == nil {
			f.coreName = coreFontName(f.name)
		}
	}
	return f
}

func (l *fontLoader) buildComposite(f *font, d types.Dict) {
	f.composite = true
	switch enc := l.deref(d["Encoding"]).(type) {
	case types.Name:
		f.identity = enc.Value() == "Identity-H" || enc.Value() == "Identity-V"
	case types.StreamDict, *types.StreamDict:
		if data := l.stream(d["Encoding"]); data != nil {
			if m := parseCMap(data); m.hasMappings() || len(m.codespaces) > 0 {
				f.encoding = m
			}
		}
	}
	f.defaultWidth = 1000
	desc := l.array(d["DescendantFonts"])
	if len(desc) == 0 {
		return
	}
	cid := l.dict(desc[0])
	if cid == nil {
		return
	}
	if dw, ok := l.number(cid["DW"]); ok {
		f.defaultWidth = dw
	}
	w := l.array(cid["W"])
	f.widths = map[uint32]float64{}
	for i := 0; i < len(w); {
		first, ok := l.number(w[i])
		if !ok || i+1 >= len(w) {
			break
		}
		if arr, isArr := l.deref(w[i+1]).(types.Array); isArr {
			for j, item := range arr {
				if v, ok := l.number(item); ok {
					f.widths[uint32(first)+uint32(j)] = v
				}
			}
			i += 2
			continue
		}
		if i+2 >= len(w) {
			break
		}
		last, ok1 := l.number(w[i+1])
		width, ok2 := l.number(w[i+2])
		if ok1 && ok2 && last >= first && last-first < 65536 {
			for c := uint32(first); c <= uint32(last); c++ {
				f.widths[c] = width
			}
		}
		i += 3
	}
}

// simpleEncoding builds a simple font's code to text table: the base
// encoding the font names, or the one its kind implies, with /Differences
// applied over it.
func (l *fontLoader) simpleEncoding(d types.Dict, subtype string, flags int) *[256]string {
	var baseName string
	var diffs types.Array
	switch enc := l.deref(d["Encoding"]).(type) {
	case types.Name:
		baseName = enc.Value()
	case types.Dict:
		baseName = l.name(enc["BaseEncoding"])
		diffs = l.array(enc["Differences"])
	}
	base := baseEncoding(baseName)
	symbolic := flags&4 != 0 && flags&32 == 0
	if base == nil {
		switch {
		case subtype == "Type3":
			// Type3 fonts carry their whole encoding in Differences.
		case subtype == "TrueType" && !symbolic:
			base = &winAnsiEncoding
		default:
			base = &standardEncoding
		}
	}
	var table [256]string
	if base != nil {
		for c, r := range base {
			if r != 0 {
				table[c] = string(r)
			}
		}
	}
	code := 0
	for _, item := range diffs {
		switch v := l.deref(item).(type) {
		case types.Integer:
			code = v.Value()
		case types.Float:
			code = int(v.Value())
		case types.Name:
			if code >= 0 && code < 256 {
				if t, ok := glyphText(v.Value()); ok {
					table[code] = t
				} else {
					table[code] = undecodable
				}
			}
			code++
		}
	}
	return &table
}

func (l *fontLoader) simpleWidths(f *font, d types.Dict) {
	if fd := l.dict(d["FontDescriptor"]); fd != nil {
		if mw, ok := l.number(fd["MissingWidth"]); ok {
			f.defaultWidth = mw
		}
	}
	widths := l.array(d["Widths"])
	if len(widths) == 0 {
		return
	}
	first := 0
	if v, ok := l.number(d["FirstChar"]); ok {
		first = int(v)
	}
	f.widths = map[uint32]float64{}
	for i, item := range widths {
		if v, ok := l.number(item); ok && first+i >= 0 && first+i < 256 {
			f.widths[uint32(first+i)] = v
		}
	}
}

// form is a Form XObject: content drawn with its own resources and matrix.
type form struct {
	id      string
	content []byte
	matrix  matrix
	res     *pageResources
}

// pageResources answers the extractor's lookups from a /Resources dictionary,
// falling back to the parent's for a form that has none of its own.
type pageResources struct {
	l      *fontLoader
	dict   types.Dict
	parent *pageResources
	fonts  map[string]*font
}

func newPageResources(l *fontLoader, d types.Dict, parent *pageResources) *pageResources {
	return &pageResources{l: l, dict: d, parent: parent, fonts: map[string]*font{}}
}

func (r *pageResources) font(name string) *font {
	if f, ok := r.fonts[name]; ok {
		return f
	}
	var f *font
	if fonts := r.l.dict(r.dict["Font"]); fonts != nil {
		if o, ok := fonts[name]; ok {
			f = r.l.load(o)
		}
	}
	if f == nil && r.parent != nil {
		f = r.parent.font(name)
	}
	r.fonts[name] = f
	return f
}

func (r *pageResources) form(name string) *form {
	xobjects := r.l.dict(r.dict["XObject"])
	o, ok := xobjects[name]
	if !ok {
		if r.parent != nil {
			return r.parent.form(name)
		}
		return nil
	}
	k := key(o)
	if k != "" {
		if f, ok := r.l.forms[k]; ok {
			return f
		}
	}
	sd, _, err := r.l.x.DereferenceStreamDict(o)
	if err != nil || sd == nil || r.l.name(sd.Dict["Subtype"]) != "Form" {
		return nil
	}
	if len(sd.Content) == 0 {
		if err := sd.Decode(); err != nil {
			return nil
		}
	}
	f := &form{id: k, content: sd.Content, matrix: identity}
	if m := r.l.array(sd.Dict["Matrix"]); len(m) == 6 {
		var mm matrix
		valid := true
		for i := range m {
			v, ok := r.l.number(m[i])
			if !ok {
				valid = false
				break
			}
			mm[i] = v
		}
		if valid {
			f.matrix = mm
		}
	}
	if res := r.l.dict(sd.Dict["Resources"]); res != nil {
		f.res = newPageResources(r.l, res, r)
	} else {
		f.res = r
	}
	if k != "" {
		r.l.forms[k] = f
	}
	return f
}
