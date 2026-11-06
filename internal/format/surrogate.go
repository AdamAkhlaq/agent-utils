package format

import (
	"bytes"
	"fmt"
	"unicode/utf16"
)

// CheckJSONSurrogates rejects a JSON document that escapes a lone UTF-16
// surrogate: a high surrogate escape not followed by a low one, or a low one
// on its own. Such a string has no UTF-8 form, and encoding/json decodes it
// as U+FFFD without a word, so a command that converts the decoded strings
// would change the data silently. The error names the line and column of
// the string's opening quote. data must be valid JSON. Callers check it
// after decoding the document, so every JSON command reports syntax errors
// first, then errors in the document's structure, then lone surrogates.
func CheckJSONSurrogates(data []byte) error {
	// Every surrogate escape starts with one of these, and most documents
	// have neither, so a fast search usually settles it.
	if !bytes.Contains(data, []byte(`\ud`)) && !bytes.Contains(data, []byte(`\uD`)) {
		return nil
	}
	for i := 0; ; {
		j := bytes.IndexByte(data[i:], '\\')
		if j < 0 {
			return nil
		}
		i += j
		r, ok := hexEscape(data, i)
		if !ok {
			// Every other escape is two bytes, and skipping both keeps an
			// escaped backslash from being read as the start of an escape.
			i += 2
			if i >= len(data) {
				return nil
			}
			continue
		}
		if !utf16.IsSurrogate(r) {
			i += 6
			continue
		}
		if r < 0xdc00 {
			if low, ok := hexEscape(data, i+6); ok && low >= 0xdc00 && low < 0xe000 {
				i += 12
				continue
			}
		}
		line, col := lineCol(data, int64(stringStart(data, i)+1))
		return fmt.Errorf("string at line %d, column %d escapes a lone UTF-16 surrogate (%s), which has no UTF-8 form", line, col, data[i:i+6])
	}
}

// hexEscape decodes the \uXXXX escape at data[i:], if there is one.
func hexEscape(data []byte, i int) (rune, bool) {
	if i+6 > len(data) || data[i] != '\\' || data[i+1] != 'u' {
		return 0, false
	}
	var r rune
	for _, c := range data[i+2 : i+6] {
		switch {
		case c >= '0' && c <= '9':
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return r, true
}

// stringStart returns the offset of the opening quote of the JSON string
// that holds offset i.
func stringStart(data []byte, i int) int {
	start, inString := 0, false
	for k := 0; k < i; k++ {
		switch {
		case inString && data[k] == '\\':
			k++
		case data[k] == '"':
			if !inString {
				start = k
			}
			inString = !inString
		}
	}
	return start
}
