package pdf

import (
	"math"
	"strings"
)

// Text extraction reads a page's content stream the way a viewer draws it:
// it follows the text matrix, shows each string with its font, and keeps
// the position after every glyph. Words and lines are then told apart by
// where the runs land, not by how the generator split them: Word writes a
// line as one run per kerning pair and fpdf one run per cell, and both read
// back as lines of words.
//
// What it does not do: reconstruct reading order across columns (text comes
// in the order the stream draws it), undo hyphenation at line ends, or
// decode glyphs a font gives no text for (see undecodable).

// matrix is a PDF transformation matrix [a b c d e f].
type matrix [6]float64

var identity = matrix{1, 0, 0, 1, 0, 0}

// mul returns m × n: m applied first, then n.
func (m matrix) mul(n matrix) matrix {
	return matrix{
		m[0]*n[0] + m[1]*n[2],
		m[0]*n[1] + m[1]*n[3],
		m[2]*n[0] + m[3]*n[2],
		m[2]*n[1] + m[3]*n[3],
		m[4]*n[0] + m[5]*n[2] + n[4],
		m[4]*n[1] + m[5]*n[3] + n[5],
	}
}

func translation(tx, ty float64) matrix { return matrix{1, 0, 0, 1, tx, ty} }

// textState is the part of the graphics state that text operators use.
type textState struct {
	font        *font
	size        float64
	charSpacing float64
	wordSpacing float64
	hscale      float64
	leading     float64
	rise        float64
	tm, tlm     matrix
}

type gstate struct {
	ctm matrix
	ts  textState
}

// resourceSource is what the extractor asks of a page or a form.
type resourceSource interface {
	font(name string) *font
	form(name string) *form
}

// maxFormDepth bounds forms within forms.
const maxFormDepth = 12

type extractor struct {
	out   textBuilder
	depth int
	forms []string // the forms being drawn, against a form that draws itself
}

// extractText returns the text of a content stream with its resources.
func extractText(content []byte, res resourceSource) string {
	e := &extractor{}
	e.run(content, res, identity)
	return e.out.String()
}

func (e *extractor) run(content []byte, res resourceSource, ctm matrix) {
	gs := gstate{ctm: ctm, ts: textState{hscale: 1, tm: identity, tlm: identity}}
	var stack []gstate
	p := newParser(content)
	for {
		ops, op, ok := p.next()
		if !ok {
			return
		}
		switch op {
		case "q":
			if len(stack) < 256 {
				stack = append(stack, gs)
			}
		case "Q":
			if n := len(stack); n > 0 {
				gs, stack = stack[n-1], stack[:n-1]
			}
		case "cm":
			if m, ok := matrixOf(ops); ok {
				gs.ctm = m.mul(gs.ctm)
			}
		case "BT":
			gs.ts.tm, gs.ts.tlm = identity, identity
		case "Tf":
			if len(ops) >= 2 {
				a := ops[len(ops)-2:]
				if a[0].kind == tokName && res != nil {
					gs.ts.font = res.font(a[0].text)
				}
				gs.ts.size = a[1].num
			}
		case "Tc":
			gs.ts.charSpacing = lastNumber(ops)
		case "Tw":
			gs.ts.wordSpacing = lastNumber(ops)
		case "Tz":
			gs.ts.hscale = lastNumber(ops) / 100
		case "TL":
			gs.ts.leading = lastNumber(ops)
		case "Ts":
			gs.ts.rise = lastNumber(ops)
		case "Td":
			if tx, ty, ok := lastTwo(ops); ok {
				nextLine(&gs.ts, tx, ty)
			}
		case "TD":
			if tx, ty, ok := lastTwo(ops); ok {
				gs.ts.leading = -ty
				nextLine(&gs.ts, tx, ty)
			}
		case "T*":
			nextLine(&gs.ts, 0, -gs.ts.leading)
		case "Tm":
			if m, ok := matrixOf(ops); ok {
				gs.ts.tm, gs.ts.tlm = m, m
			}
		case "Tj":
			if s, ok := lastString(ops); ok {
				e.show(&gs, s)
			}
		case "'":
			nextLine(&gs.ts, 0, -gs.ts.leading)
			if s, ok := lastString(ops); ok {
				e.show(&gs, s)
			}
		case "\"":
			if len(ops) >= 3 {
				a := ops[len(ops)-3:]
				gs.ts.wordSpacing, gs.ts.charSpacing = a[0].num, a[1].num
				nextLine(&gs.ts, 0, -gs.ts.leading)
				if a[2].isString() {
					e.show(&gs, []byte(a[2].text))
				}
			}
		case "TJ":
			if len(ops) == 0 || ops[len(ops)-1].kind != tokArrayOpen {
				continue
			}
			for _, it := range ops[len(ops)-1].items {
				switch {
				case it.isString():
					e.show(&gs, []byte(it.text))
				case it.isNumber():
					// A negative adjustment moves right: generators write
					// word gaps that way, and the position shows them.
					tx := -it.num / 1000 * gs.ts.size * gs.ts.hscale
					gs.ts.tm = translation(tx, 0).mul(gs.ts.tm)
				}
			}
		case "Do":
			if len(ops) >= 1 && ops[len(ops)-1].kind == tokName && res != nil {
				e.drawForm(res, ops[len(ops)-1].text, gs.ctm)
			}
		}
	}
}

func nextLine(ts *textState, tx, ty float64) {
	ts.tlm = translation(tx, ty).mul(ts.tlm)
	ts.tm = ts.tlm
}

func matrixOf(ops []operand) (matrix, bool) {
	if len(ops) < 6 {
		return matrix{}, false
	}
	var m matrix
	for i, o := range ops[len(ops)-6:] {
		if !o.isNumber() {
			return matrix{}, false
		}
		m[i] = o.num
	}
	return m, true
}

func lastNumber(ops []operand) float64 {
	if n := len(ops); n > 0 && ops[n-1].isNumber() {
		return ops[n-1].num
	}
	return 0
}

func lastTwo(ops []operand) (float64, float64, bool) {
	n := len(ops)
	if n < 2 || !ops[n-2].isNumber() || !ops[n-1].isNumber() {
		return 0, 0, false
	}
	return ops[n-2].num, ops[n-1].num, true
}

func lastString(ops []operand) ([]byte, bool) {
	if n := len(ops); n > 0 && ops[n-1].isString() {
		return []byte(ops[n-1].text), true
	}
	return nil, false
}

// show draws a string: it decodes the glyphs, advances the text matrix by
// each, and hands the run with its start and end positions to the builder.
func (e *extractor) show(gs *gstate, s []byte) {
	ts := &gs.ts
	if ts.font == nil || len(s) == 0 {
		return
	}
	params := matrix{ts.size * ts.hscale, 0, 0, ts.size, 0, ts.rise}
	start := params.mul(ts.tm).mul(gs.ctm)

	var sb strings.Builder
	for _, g := range ts.font.decode(s) {
		sb.WriteString(g.text)
		adv := g.width/1000*ts.size + ts.charSpacing
		if g.space {
			adv += ts.wordSpacing
		}
		ts.tm = translation(adv*ts.hscale, 0).mul(ts.tm)
	}
	end := params.mul(ts.tm).mul(gs.ctm)

	// Positions along the baseline and across it, so rotated text reads
	// the same as upright text.
	ux, uy := start[0], start[1]
	if norm := math.Hypot(ux, uy); norm > 0 {
		ux, uy = ux/norm, uy/norm
	} else {
		ux, uy = 1, 0
	}
	along := func(m matrix) float64 { return m[4]*ux + m[5]*uy }
	across := func(m matrix) float64 { return -m[4]*uy + m[5]*ux }
	size := math.Hypot(start[2], start[3])
	e.out.add(sb.String(), along(start), across(start), along(end), size)
}

func (e *extractor) drawForm(res resourceSource, name string, ctm matrix) {
	f := res.form(name)
	if f == nil || e.depth >= maxFormDepth {
		return
	}
	for _, id := range e.forms {
		if id != "" && id == f.id {
			return
		}
	}
	e.depth++
	e.forms = append(e.forms, f.id)
	var inner resourceSource = f.res
	if f.res == nil {
		inner = res
	}
	e.run(f.content, inner, f.matrix.mul(ctm))
	e.forms = e.forms[:len(e.forms)-1]
	e.depth--
}

// textBuilder assembles runs into lines. A run that lands on another
// baseline starts a line; one that leaves a gap after the previous run
// starts a word.
type textBuilder struct {
	sb       strings.Builder
	have     bool
	lastLine float64 // across the baseline
	lastEnd  float64 // along it
	lastSize float64
}

func (b *textBuilder) add(s string, start, line, end, size float64) {
	if s == "" {
		return
	}
	if size <= 0 {
		size = b.lastSize
	}
	if size <= 0 {
		size = 10
	}
	if b.have {
		ref := math.Max(size, b.lastSize)
		switch gap := start - b.lastEnd; {
		case math.Abs(line-b.lastLine) > 0.5*ref:
			b.newline()
		case gap > 0.12*ref || gap < -ref:
			// Wider than kerning, it is a word gap; far back on the same
			// line, it is another column or cell.
			b.space()
		}
	}
	b.sb.WriteString(s)
	b.have, b.lastLine, b.lastEnd, b.lastSize = true, line, end, size
}

func (b *textBuilder) space() {
	if s := b.sb.String(); s != "" && !strings.HasSuffix(s, " ") && !strings.HasSuffix(s, "\n") {
		b.sb.WriteByte(' ')
	}
}

func (b *textBuilder) newline() {
	if s := b.sb.String(); s != "" && !strings.HasSuffix(s, "\n") {
		b.sb.WriteByte('\n')
	}
}

// String returns the text with runs of whitespace as one space, lines
// trimmed, and blank lines dropped.
func (b *textBuilder) String() string {
	return normalizeText(b.sb.String())
}

func normalizeText(s string) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.Join(strings.Fields(strings.ReplaceAll(line, " ", " ")), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
