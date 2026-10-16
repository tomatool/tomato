package pdf

import (
	"strings"
	"unicode/utf16"
)

// A CMap (ISO 32000-1, 9.7.5 and 9.10.3) maps the byte codes a font's
// strings are made of. A ToUnicode CMap maps them to text with bfchar and
// bfrange; an encoding CMap of a composite font maps them to CIDs with
// cidchar and cidrange. Both begin with the codespace ranges that say how
// many bytes a code has.

type codespace struct {
	n      int
	lo, hi []byte
}

type codeKey struct {
	n    int
	code uint32
}

type bfRange struct {
	n      int
	lo, hi uint32
	dst    []byte   // the text of lo, UTF-16BE; later codes count up from it
	dsts   [][]byte // or one text per code
}

type cidRange struct {
	n      int
	lo, hi uint32
	cid    uint32
}

type cmap struct {
	codespaces []codespace
	chars      map[codeKey]string
	ranges     []bfRange
	cidChars   map[codeKey]uint32
	cidRanges  []cidRange
	// lengths records the code lengths bfchar and bfrange used, for CMaps
	// that leave the codespace out.
	lengths [5]bool
}

// parseCMap reads a CMap. It never fails: what it cannot read is skipped.
func parseCMap(src []byte) *cmap {
	m := &cmap{chars: map[codeKey]string{}, cidChars: map[codeKey]uint32{}}
	p := newParser(src)
	for {
		_, op, ok := p.next()
		if !ok {
			return m
		}
		switch op {
		case "begincodespacerange":
			items := p.section("endcodespacerange")
			for i := 0; i+1 < len(items); i += 2 {
				lo, hi := items[i], items[i+1]
				if !lo.isString() || !hi.isString() || len(lo.text) == 0 || len(lo.text) != len(hi.text) || len(lo.text) > 4 {
					continue
				}
				m.codespaces = append(m.codespaces, codespace{n: len(lo.text), lo: []byte(lo.text), hi: []byte(hi.text)})
			}
		case "beginbfchar":
			items := p.section("endbfchar")
			for i := 0; i+1 < len(items); i += 2 {
				src, dst := items[i], items[i+1]
				if !src.isString() || len(src.text) == 0 || len(src.text) > 4 {
					continue
				}
				var text string
				switch {
				case dst.isString():
					text = utf16BE([]byte(dst.text))
				case dst.kind == tokName:
					text, _ = glyphText(dst.text)
				default:
					continue
				}
				m.chars[codeKey{len(src.text), codeOf([]byte(src.text))}] = text
				m.lengths[len(src.text)] = true
			}
		case "beginbfrange":
			items := p.section("endbfrange")
			for i := 0; i+2 < len(items); i += 3 {
				lo, hi, dst := items[i], items[i+1], items[i+2]
				if !lo.isString() || !hi.isString() || len(lo.text) == 0 || len(lo.text) > 4 || len(hi.text) != len(lo.text) {
					continue
				}
				r := bfRange{n: len(lo.text), lo: codeOf([]byte(lo.text)), hi: codeOf([]byte(hi.text))}
				switch {
				case dst.isString():
					r.dst = []byte(dst.text)
				case dst.kind == tokArrayOpen:
					for _, it := range dst.items {
						if it.isString() {
							r.dsts = append(r.dsts, []byte(it.text))
						}
					}
				default:
					continue
				}
				m.ranges = append(m.ranges, r)
				m.lengths[r.n] = true
			}
		case "begincidchar":
			items := p.section("endcidchar")
			for i := 0; i+1 < len(items); i += 2 {
				src, cid := items[i], items[i+1]
				if !src.isString() || len(src.text) == 0 || len(src.text) > 4 || !cid.isNumber() {
					continue
				}
				m.cidChars[codeKey{len(src.text), codeOf([]byte(src.text))}] = uint32(cid.num)
				m.lengths[len(src.text)] = true
			}
		case "begincidrange":
			items := p.section("endcidrange")
			for i := 0; i+2 < len(items); i += 3 {
				lo, hi, cid := items[i], items[i+1], items[i+2]
				if !lo.isString() || !hi.isString() || len(lo.text) == 0 || len(lo.text) > 4 || len(hi.text) != len(lo.text) || !cid.isNumber() {
					continue
				}
				m.cidRanges = append(m.cidRanges, cidRange{n: len(lo.text), lo: codeOf([]byte(lo.text)), hi: codeOf([]byte(hi.text)), cid: uint32(cid.num)})
				m.lengths[len(lo.text)] = true
			}
		}
	}
}

// maxSectionItems bounds one CMap section; a font maps at most 65536
// codes, three items each.
const maxSectionItems = 3 * 65536

// section collects the items up to the given end keyword. It reads tokens
// itself: next() keeps the last operands of an operator only, and a
// section is thousands of items long.
func (p *parser) section(end string) []operand {
	var items []operand
	for len(items) < maxSectionItems {
		t := p.lex.next()
		switch t.kind {
		case tokEOF:
			return items
		case tokKeyword:
			if t.text == end || strings.HasPrefix(t.text, "begin") || strings.HasPrefix(t.text, "end") {
				return items
			}
		case tokArrayClose, tokDictClose:
		default:
			items = append(items, p.value(t, 0))
		}
	}
	return items
}

func codeOf(b []byte) uint32 {
	var v uint32
	for _, c := range b {
		v = v<<8 | uint32(c)
	}
	return v
}

// utf16BE decodes the destination of a bfchar or bfrange. A single byte,
// which some generators write, is read as the character it is.
func utf16BE(b []byte) string {
	if len(b) == 1 {
		return string(rune(b[0]))
	}
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
	}
	rs := utf16.Decode(units)
	var out []rune
	for _, r := range rs {
		if r == 0xFEFF {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

// addToLast counts a bfrange destination up: the last UTF-16 code unit is
// the counter, so <0041> + 0xC0 is U+0101, not a byte overflow. That is
// what the identity ranges of fpdf and TCPDF (<0000> <FFFF> <0000>) need.
func addToLast(dst []byte, delta uint32) []byte {
	out := append([]byte(nil), dst...)
	switch {
	case len(out) >= 2:
		i := len(out) - 2
		v := uint32(out[i])<<8 | uint32(out[i+1]) + delta
		out[i], out[i+1] = byte(v>>8), byte(v)
	case len(out) == 1:
		out[0] += byte(delta)
	}
	return out
}

// text returns what a code of n bytes maps to. A CMap that writes its
// mappings in another width than the font's codes (a two-byte codespace on
// a simple font happens) is read by value as well.
func (m *cmap) text(code uint32, n int) (string, bool) {
	if s, ok := m.textOfLength(code, n); ok {
		return s, true
	}
	for other := 1; other <= 4; other++ {
		if other == n {
			continue
		}
		if s, ok := m.textOfLength(code, other); ok {
			return s, true
		}
	}
	return "", false
}

func (m *cmap) textOfLength(code uint32, n int) (string, bool) {
	if s, ok := m.chars[codeKey{n, code}]; ok {
		return s, true
	}
	for _, r := range m.ranges {
		if r.n != n || code < r.lo || code > r.hi {
			continue
		}
		if r.dsts != nil {
			if i := int(code - r.lo); i < len(r.dsts) {
				return utf16BE(r.dsts[i]), true
			}
			return "", false
		}
		return utf16BE(addToLast(r.dst, code-r.lo)), true
	}
	return "", false
}

// cid returns the CID a code maps to in an encoding CMap.
func (m *cmap) cid(code uint32, n int) (uint32, bool) {
	if c, ok := m.cidChars[codeKey{n, code}]; ok {
		return c, true
	}
	for _, r := range m.cidRanges {
		if r.n == n && code >= r.lo && code <= r.hi {
			return r.cid + (code - r.lo), true
		}
	}
	return 0, false
}

// hasMappings reports whether the CMap maps anything at all.
func (m *cmap) hasMappings() bool {
	return len(m.chars) > 0 || len(m.ranges) > 0 || len(m.cidChars) > 0 || len(m.cidRanges) > 0
}

// defaultLength is the code length to use when the codespace does not
// decide: the one length the mappings use, or 0 for no opinion.
func (m *cmap) defaultLength() int {
	n := 0
	for i := 1; i <= 4; i++ {
		if m.lengths[i] {
			if n != 0 {
				return 0
			}
			n = i
		}
	}
	return n
}

type code struct {
	code uint32
	n    int
}

// split cuts a string into codes by the codespace ranges; bytes outside
// every range are read as codes of fallback bytes (1 when 0).
func (m *cmap) split(s []byte, fallback int) []code {
	if fallback <= 0 {
		fallback = 1
	}
	var out []code
	for i := 0; i < len(s); {
		n := 0
		for try := 1; try <= 4 && i+try <= len(s) && n == 0; try++ {
			for _, cs := range m.codespaces {
				if cs.n == try && inCodespace(s[i:i+try], cs) {
					n = try
					break
				}
			}
		}
		if n == 0 {
			n = fallback
			if i+n > len(s) {
				n = len(s) - i
			}
		}
		out = append(out, code{code: codeOf(s[i : i+n]), n: n})
		i += n
	}
	return out
}

func inCodespace(b []byte, cs codespace) bool {
	for i := range b {
		if b[i] < cs.lo[i] || b[i] > cs.hi[i] {
			return false
		}
	}
	return true
}

// splitFixed cuts a string into codes of n bytes each.
func splitFixed(s []byte, n int) []code {
	out := make([]code, 0, len(s)/n+1)
	for i := 0; i < len(s); i += n {
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		out = append(out, code{code: codeOf(s[i:end]), n: end - i})
	}
	return out
}
