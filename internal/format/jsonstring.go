package format

import "unicode/utf8"

// asciiEscapes holds the escape encoding/json writes for each ASCII byte that
// needs one with HTML escaping off, and "" for every other byte.
var asciiEscapes = func() (t [utf8.RuneSelf]string) {
	const hex = "0123456789abcdef"
	for c := range 0x20 {
		t[c] = `\u00` + string(hex[c>>4]) + string(hex[c&0xf])
	}
	t['\b'], t['\f'], t['\n'], t['\r'], t['\t'] = `\b`, `\f`, `\n`, `\r`, `\t`
	t['"'], t['\\'] = `\"`, `\\`
	return t
}()

// escapeJSONString passes s to emit as a JSON string, in pieces: the quotes,
// runs of s that need no escaping, and the escapes between them. It escapes
// exactly as a json.Encoder with SetEscapeHTML(false) escapes a valid UTF-8
// string, as the other converters do: quotes, backslashes, control
// characters, and U+2028 and U+2029, leaving <, >, and & as is. Each byte of
// invalid UTF-8 becomes U+FFFD, written as is: encoding/json writes it as
// \ufffd on Go 1.26 and as is on Go 1.27, so following it would make the
// output depend on the toolchain. Passing pieces
// lets a caller count or stream a string of any length without building it,
// and avoids json.Marshal's reflection, which took most of csv2json's time.
func escapeJSONString(s string, emit func(string)) {
	emit(`"`)
	start := 0
	for i := 0; i < len(s); {
		esc, size := "", 1
		if c := s[i]; c < utf8.RuneSelf {
			esc = asciiEscapes[c]
		} else {
			var r rune
			r, size = utf8.DecodeRuneInString(s[i:])
			switch {
			case r == utf8.RuneError && size == 1:
				esc = string(utf8.RuneError)
			case r == '\u2028':
				esc = `\u2028`
			case r == '\u2029':
				esc = `\u2029`
			}
		}
		if esc == "" {
			i += size
			continue
		}
		if start < i {
			emit(s[start:i])
		}
		emit(esc)
		i += size
		start = i
	}
	if start < len(s) {
		emit(s[start:])
	}
	emit(`"`)
}

// appendJSONString appends s to dst as a JSON string, escaped as
// escapeJSONString escapes it.
func appendJSONString(dst []byte, s string) []byte {
	escapeJSONString(s, func(piece string) { dst = append(dst, piece...) })
	return dst
}
