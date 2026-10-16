package pdf

import (
	"fmt"
	"strings"
	"testing"
)

const identityToUnicode = `/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapName /Adobe-Identity-UCS def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0000> <FFFF> <0000>
endbfrange
endcmap
end end`

func TestCMapIdentityRangeCountsThroughTheHighByte(t *testing.T) {
	m := parseCMap([]byte(identityToUnicode))
	for code, want := range map[uint32]string{0x0041: "A", 0x00E4: "ä", 0x2013: "–", 0x20AC: "€"} {
		got, ok := m.text(code, 2)
		if !ok || got != want {
			t.Errorf("code %04X: got %q %v, want %q", code, got, ok, want)
		}
	}
}

func TestCMapCharsRangesAndArrays(t *testing.T) {
	src := `1 begincodespacerange <00> <FF> endcodespacerange
3 beginbfchar
<01> <0044>
<02> <00660069>
<03> /eacute
endbfchar
2 beginbfrange
<10> <12> <0061>
<20> <21> [<0058> <D83DDE00>]
endbfrange`
	m := parseCMap([]byte(src))
	cases := map[uint32]string{0x01: "D", 0x02: "fi", 0x03: "é", 0x10: "a", 0x12: "c", 0x20: "X", 0x21: "😀"}
	for code, want := range cases {
		got, ok := m.text(code, 1)
		if !ok || got != want {
			t.Errorf("code %02X: got %q %v, want %q", code, got, ok, want)
		}
	}
	if _, ok := m.text(0x13, 1); ok {
		t.Error("0x13 is outside every range")
	}
	if _, ok := m.text(0x22, 1); ok {
		t.Error("0x22 is past the array")
	}
	if n := m.defaultLength(); n != 1 {
		t.Errorf("defaultLength %d, want 1", n)
	}
}

func TestCMapLookupIgnoresCodeWidthMismatch(t *testing.T) {
	// A two-byte codespace on a CMap used by a simple font: the codes are
	// one byte wide when shown.
	m := parseCMap([]byte("1 begincodespacerange <0000> <FFFF> endcodespacerange 1 beginbfchar <0041> <0061> endbfchar"))
	if got, ok := m.text(0x41, 1); !ok || got != "a" {
		t.Errorf("got %q %v", got, ok)
	}
}

func TestCMapLongSectionsAreKeptWhole(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("1 begincodespacerange <0000> <FFFF> endcodespacerange\n300 beginbfchar\n")
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&sb, "<%04X> <%04X>\n", i+1, 0x41+i%26)
	}
	sb.WriteString("endbfchar")
	m := parseCMap([]byte(sb.String()))
	if len(m.chars) != 300 {
		t.Fatalf("kept %d of 300 bfchar entries", len(m.chars))
	}
	if got, _ := m.text(1, 2); got != "A" {
		t.Errorf("first entry: %q", got)
	}
}

func TestCMapCIDMappings(t *testing.T) {
	src := `1 begincodespacerange <00> <80> <8140> <FFFF> endcodespacerange
1 begincidchar <20> 1 endcidchar
1 begincidrange <8140> <817E> 633 endcidrange`
	m := parseCMap([]byte(src))
	if cid, ok := m.cid(0x20, 1); !ok || cid != 1 {
		t.Errorf("cidchar: %d %v", cid, ok)
	}
	if cid, ok := m.cid(0x8142, 2); !ok || cid != 635 {
		t.Errorf("cidrange: %d %v", cid, ok)
	}
	if _, ok := m.cid(0x8200, 2); ok {
		t.Error("0x8200 is outside the range")
	}
	codes := m.split([]byte{0x20, 0x81, 0x42, 0x41}, 2)
	want := []code{{0x20, 1}, {0x8142, 2}, {0x41, 1}}
	if fmt.Sprint(codes) != fmt.Sprint(want) {
		t.Errorf("split %v, want %v", codes, want)
	}
	// A byte in no codespace falls back to the given width, clipped at the end.
	codes = m.split([]byte{0x90, 0x01, 0x90}, 2)
	if fmt.Sprint(codes) != fmt.Sprint([]code{{0x9001, 2}, {0x90, 1}}) {
		t.Errorf("fallback split %v", codes)
	}
	if !m.hasMappings() {
		t.Error("hasMappings")
	}
	if parseCMap(nil).hasMappings() {
		t.Error("an empty CMap has no mappings")
	}
}

func TestCMapIgnoresMalformedEntries(t *testing.T) {
	src := `1 begincodespacerange <00> <FFFF> endcodespacerange
2 beginbfchar <01> endbfchar
1 beginbfrange <01> <02> 5 endbfrange
1 beginbfrange <0001> <02> <0041> endbfrange
1 begincidrange <01> <02> (x) endcidrange`
	m := parseCMap([]byte(src))
	if m.hasMappings() || len(m.codespaces) != 0 {
		t.Errorf("malformed entries were kept: %+v", m)
	}
	if n := m.defaultLength(); n != 0 {
		t.Errorf("defaultLength of nothing: %d", n)
	}
	mixed := parseCMap([]byte("1 beginbfchar <01> <0041> endbfchar 1 beginbfchar <0002> <0042> endbfchar"))
	if n := mixed.defaultLength(); n != 0 {
		t.Errorf("two widths give no default, got %d", n)
	}
}

func TestUTF16BEAndAddToLast(t *testing.T) {
	if got := utf16BE([]byte{0xFE, 0xFF, 0x00, 0x41, 0xD8, 0x3D, 0xDE, 0x00}); got != "A😀" {
		t.Errorf("utf16BE: %q", got)
	}
	if got := utf16BE([]byte{0xE9}); got != "é" {
		t.Errorf("single byte: %q", got)
	}
	if got := utf16BE(nil); got != "" {
		t.Errorf("empty: %q", got)
	}
	if got := string(addToLast([]byte{0x00, 0xFF}, 1)); got != "\x01\x00" {
		t.Errorf("addToLast carries into the high byte: %q", got)
	}
	if got := string(addToLast([]byte{0x41}, 2)); got != "C" {
		t.Errorf("addToLast one byte: %q", got)
	}
	if got := addToLast(nil, 2); len(got) != 0 {
		t.Errorf("addToLast nil: %v", got)
	}
}

func TestSplitFixed(t *testing.T) {
	got := splitFixed([]byte{1, 2, 3}, 2)
	if fmt.Sprint(got) != fmt.Sprint([]code{{0x0102, 2}, {0x03, 1}}) {
		t.Errorf("splitFixed: %v", got)
	}
}
