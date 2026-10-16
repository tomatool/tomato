package pdf

import (
	"strconv"
	"strings"
)

// The predefined simple-font encodings of ISO 32000-1 Annex D, as code to
// rune. A simple font shows one byte per glyph; without a ToUnicode CMap the
// byte means what its encoding says. Codes with no glyph are 0.

var (
	standardEncoding [256]rune
	winAnsiEncoding  [256]rune
	macRomanEncoding [256]rune
)

// Each high half is exactly 128 runes, 0x80 through 0xFF, 16 per line; a
// test checks the count. '\x00' is an unused code.
const (
	winAnsiHigh = "€•‚ƒ„…†‡ˆ‰Š‹Œ•Ž•" +
		"•‘’“”•–—˜™š›œ•žŸ" +
		" ¡¢£¤¥¦§¨©ª«¬-®¯" +
		"°±²³´µ¶·¸¹º»¼½¾¿" +
		"ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏ" +
		"ÐÑÒÓÔÕÖ×ØÙÚÛÜÝÞß" +
		"àáâãäåæçèéêëìíîï" +
		"ðñòóôõö÷øùúûüýþÿ"

	macRomanHigh = "ÄÅÇÉÑÖÜáàâäãåçéè" +
		"êëíìîïñóòôöõúùûü" +
		"†°¢£§•¶ß®©™´¨≠ÆØ" +
		"∞±≤≥¥µ∂∑∏π∫ªºΩæø" +
		"¿¡¬√ƒ≈∆«»… ÀÃÕŒœ" +
		"–—“”‘’÷◊ÿŸ⁄€‹›ﬁﬂ" +
		"‡·‚„‰ÂÊÁËÈÍÎÏÌÓÔ" +
		"ÒÚÛÙıˆ˜¯˘˙˚¸˝˛ˇ"

	// StandardEncoding's high half is sparse; it is listed by code below.
)

var standardHigh = map[byte]rune{
	0xA1: '¡', 0xA2: '¢', 0xA3: '£', 0xA4: '⁄', 0xA5: '¥', 0xA6: 'ƒ', 0xA7: '§', 0xA8: '¤',
	0xA9: '\'', 0xAA: '“', 0xAB: '«', 0xAC: '‹', 0xAD: '›', 0xAE: 'ﬁ', 0xAF: 'ﬂ',
	0xB1: '–', 0xB2: '†', 0xB3: '‡', 0xB4: '·', 0xB6: '¶', 0xB7: '•', 0xB8: '‚', 0xB9: '„',
	0xBA: '”', 0xBB: '»', 0xBC: '…', 0xBD: '‰', 0xBF: '¿',
	0xC1: '`', 0xC2: '´', 0xC3: 'ˆ', 0xC4: '˜', 0xC5: '¯', 0xC6: '˘', 0xC7: '˙', 0xC8: '¨',
	0xCA: '˚', 0xCB: '¸', 0xCD: '˝', 0xCE: '˛', 0xCF: 'ˇ', 0xD0: '—',
	0xE1: 'Æ', 0xE3: 'ª', 0xE8: 'Ł', 0xE9: 'Ø', 0xEA: 'Œ', 0xEB: 'º',
	0xF1: 'æ', 0xF5: 'ı', 0xF8: 'ł', 0xF9: 'ø', 0xFA: 'œ', 0xFB: 'ß',
}

func init() {
	for c := 32; c < 127; c++ {
		standardEncoding[c] = rune(c)
		winAnsiEncoding[c] = rune(c)
		macRomanEncoding[c] = rune(c)
	}
	// StandardEncoding's 0x27 and 0x60 are the typographic quotes.
	standardEncoding['\''] = '’'
	standardEncoding['`'] = '‘'
	for c, r := range standardHigh {
		standardEncoding[c] = r
	}
	for i, r := range []rune(winAnsiHigh) {
		winAnsiEncoding[0x80+i] = r
	}
	for i, r := range []rune(macRomanHigh) {
		macRomanEncoding[0x80+i] = r
	}
}

// baseEncoding returns the table for a predefined encoding name, or nil.
func baseEncoding(name string) *[256]rune {
	switch name {
	case "WinAnsiEncoding":
		return &winAnsiEncoding
	case "MacRomanEncoding":
		return &macRomanEncoding
	case "StandardEncoding", "MacExpertEncoding":
		return &standardEncoding
	}
	return nil
}

// glyphNames maps the glyph names a /Differences array uses to the text
// they stand for: the Adobe Glyph List's Latin part plus the ligatures and
// punctuation that appear in generated documents. Names outside it are
// resolved by their uniXXXX / uXXXX form, or not at all.
var glyphNames = map[string]string{
	"space": " ", "exclam": "!", "quotedbl": "\"", "numbersign": "#", "dollar": "$", "percent": "%",
	"ampersand": "&", "quotesingle": "'", "parenleft": "(", "parenright": ")", "asterisk": "*",
	"plus": "+", "comma": ",", "hyphen": "-", "period": ".", "slash": "/",
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4", "five": "5", "six": "6",
	"seven": "7", "eight": "8", "nine": "9", "colon": ":", "semicolon": ";", "less": "<",
	"equal": "=", "greater": ">", "question": "?", "at": "@", "bracketleft": "[", "backslash": "\\",
	"bracketright": "]", "asciicircum": "^", "underscore": "_", "grave": "`", "braceleft": "{",
	"bar": "|", "braceright": "}", "asciitilde": "~",
	"exclamdown": "¡", "cent": "¢", "sterling": "£", "currency": "¤", "yen": "¥", "brokenbar": "¦",
	"section": "§", "dieresis": "¨", "copyright": "©", "ordfeminine": "ª", "guillemotleft": "«",
	"guillemetleft": "«", "logicalnot": "¬", "registered": "®", "macron": "¯", "degree": "°",
	"plusminus": "±", "twosuperior": "²", "threesuperior": "³", "acute": "´", "mu": "µ",
	"paragraph": "¶", "periodcentered": "·", "middot": "·", "cedilla": "¸", "onesuperior": "¹",
	"ordmasculine": "º", "guillemotright": "»", "guillemetright": "»", "onequarter": "¼",
	"onehalf": "½", "threequarters": "¾", "questiondown": "¿", "multiply": "×", "divide": "÷",
	"Agrave": "À", "Aacute": "Á", "Acircumflex": "Â", "Atilde": "Ã", "Adieresis": "Ä", "Aring": "Å",
	"AE": "Æ", "Ccedilla": "Ç", "Egrave": "È", "Eacute": "É", "Ecircumflex": "Ê", "Edieresis": "Ë",
	"Igrave": "Ì", "Iacute": "Í", "Icircumflex": "Î", "Idieresis": "Ï", "Eth": "Ð", "Ntilde": "Ñ",
	"Ograve": "Ò", "Oacute": "Ó", "Ocircumflex": "Ô", "Otilde": "Õ", "Odieresis": "Ö", "Oslash": "Ø",
	"Ugrave": "Ù", "Uacute": "Ú", "Ucircumflex": "Û", "Udieresis": "Ü", "Yacute": "Ý", "Thorn": "Þ",
	"germandbls": "ß", "agrave": "à", "aacute": "á", "acircumflex": "â", "atilde": "ã", "adieresis": "ä",
	"aring": "å", "ae": "æ", "ccedilla": "ç", "egrave": "è", "eacute": "é", "ecircumflex": "ê",
	"edieresis": "ë", "igrave": "ì", "iacute": "í", "icircumflex": "î", "idieresis": "ï", "eth": "ð",
	"ntilde": "ñ", "ograve": "ò", "oacute": "ó", "ocircumflex": "ô", "otilde": "õ", "odieresis": "ö",
	"oslash": "ø", "ugrave": "ù", "uacute": "ú", "ucircumflex": "û", "udieresis": "ü", "yacute": "ý",
	"thorn": "þ", "ydieresis": "ÿ", "Ydieresis": "Ÿ",
	"Amacron": "Ā", "amacron": "ā", "Abreve": "Ă", "abreve": "ă", "Aogonek": "Ą", "aogonek": "ą",
	"Cacute": "Ć", "cacute": "ć", "Ccaron": "Č", "ccaron": "č", "Dcaron": "Ď", "dcaron": "ď",
	"Dcroat": "Đ", "dcroat": "đ", "Emacron": "Ē", "emacron": "ē", "Edotaccent": "Ė", "edotaccent": "ė",
	"Eogonek": "Ę", "eogonek": "ę", "Ecaron": "Ě", "ecaron": "ě", "Gbreve": "Ğ", "gbreve": "ğ",
	"Imacron": "Ī", "imacron": "ī", "Iogonek": "Į", "iogonek": "į", "Idotaccent": "İ", "dotlessi": "ı",
	"Lacute": "Ĺ", "lacute": "ĺ", "Lcaron": "Ľ", "lcaron": "ľ", "Lslash": "Ł", "lslash": "ł",
	"Nacute": "Ń", "nacute": "ń", "Ncaron": "Ň", "ncaron": "ň", "Omacron": "Ō", "omacron": "ō",
	"Ohungarumlaut": "Ő", "ohungarumlaut": "ő", "OE": "Œ", "oe": "œ", "Racute": "Ŕ", "racute": "ŕ",
	"Rcaron": "Ř", "rcaron": "ř", "Sacute": "Ś", "sacute": "ś", "Scedilla": "Ş", "scedilla": "ş",
	"Scaron": "Š", "scaron": "š", "Tcaron": "Ť", "tcaron": "ť", "Umacron": "Ū", "umacron": "ū",
	"Uring": "Ů", "uring": "ů", "Uhungarumlaut": "Ű", "uhungarumlaut": "ű", "Uogonek": "Ų", "uogonek": "ų",
	"Zacute": "Ź", "zacute": "ź", "Zdotaccent": "Ż", "zdotaccent": "ż", "Zcaron": "Ž", "zcaron": "ž",
	"florin": "ƒ", "circumflex": "ˆ", "caron": "ˇ", "breve": "˘", "dotaccent": "˙", "ring": "˚",
	"ogonek": "˛", "tilde": "˜", "hungarumlaut": "˝",
	"Delta": "∆", "Omega": "Ω", "pi": "π", "mu1": "µ",
	"endash": "–", "emdash": "—", "underscoredbl": "‗", "quoteleft": "‘", "quoteright": "’",
	"quotesinglbase": "‚", "quotereversed": "‛", "quotedblleft": "“", "quotedblright": "”",
	"quotedblbase": "„", "dagger": "†", "daggerdbl": "‡", "bullet": "•", "ellipsis": "…",
	"perthousand": "‰", "minute": "′", "second": "″", "guilsinglleft": "‹", "guilsinglright": "›",
	"fraction": "⁄", "Euro": "€", "trademark": "™", "arrowleft": "←", "arrowup": "↑", "arrowright": "→",
	"arrowdown": "↓", "partialdiff": "∂", "product": "∏", "summation": "∑", "minus": "−",
	"radical": "√", "infinity": "∞", "integral": "∫", "approxequal": "≈", "notequal": "≠",
	"lessequal": "≤", "greaterequal": "≥", "lozenge": "◊", "checkmark": "✓",
	"ff": "ﬀ", "fi": "ﬁ", "fl": "ﬂ", "ffi": "ﬃ", "ffl": "ﬄ", "f_f": "ﬀ", "f_i": "ﬁ", "f_l": "ﬂ",
	"f_f_i": "ﬃ", "f_f_l": "ﬄ", "nbspace": " ", "nonbreakingspace": " ", "sfthyphen": "-",
	"softhyphen": "-", "apple": "", ".notdef": "",
}

// glyphText returns the text a glyph name stands for, and whether it is known.
func glyphText(name string) (string, bool) {
	if s, ok := glyphNames[name]; ok {
		return s, true
	}
	// A letter's glyph name is the letter: /A, /b.
	if len(name) == 1 && (name[0] >= 'A' && name[0] <= 'Z' || name[0] >= 'a' && name[0] <= 'z') {
		return name, true
	}
	// Stylistic variants: a.sc, fi.alt, one.oldstyle.
	if i := strings.IndexByte(name, '.'); i > 0 {
		return glyphText(name[:i])
	}
	// uni20AC, uni00660069 (several code points), u1F600.
	if strings.HasPrefix(name, "uni") && len(name) >= 7 && (len(name)-3)%4 == 0 {
		var sb strings.Builder
		for i := 3; i+4 <= len(name); i += 4 {
			v, err := strconv.ParseUint(name[i:i+4], 16, 32)
			if err != nil {
				return "", false
			}
			sb.WriteRune(rune(v))
		}
		return sb.String(), true
	}
	if strings.HasPrefix(name, "u") && len(name) >= 5 && len(name) <= 7 {
		if v, err := strconv.ParseUint(name[1:], 16, 32); err == nil {
			return string(rune(v)), true
		}
	}
	// Names like g123, cid45 or G7 are glyph indexes with no text behind them.
	return "", false
}
