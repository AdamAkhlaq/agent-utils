package format

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

var jsonStringSeeds = []string{
	"", "plain", `"quoted" \back\slash`, "\b\f\n\r\t", "\x00\x01\x1f\x7f", "<a href=\"x\">&amp;</a>",
	"\u2027\u2028\u2029\u202a", "é 日本 😀", "bad \xff byte", "\xed\xa0\x80", "trailing \xe2\x80",
}

func TestAppendJSONString(t *testing.T) {
	for _, s := range jsonStringSeeds {
		t.Run(s, func(t *testing.T) { checkJSONString(t, s) })
	}
}

// FuzzAppendJSONString holds appendJSONString to json.Marshal's output on
// every string, so csv2json's output does not change with the encoder. For
// invalid UTF-8 it compares with the string whose invalid bytes are already
// U+FFFD, where json.Marshal agrees across Go releases.
func FuzzAppendJSONString(f *testing.F) {
	for _, s := range jsonStringSeeds {
		f.Add(s)
	}
	f.Fuzz(checkJSONString)
}

func checkJSONString(t *testing.T, s string) {
	want, err := json.Marshal(replaceInvalidBytes(s))
	if err != nil {
		t.Fatal(err)
	}
	if got := appendJSONString([]byte("x"), s); string(got) != "x"+string(want) {
		t.Errorf("appendJSONString(%q) = %s, want %s", s, got[1:], want)
	}
}

// replaceInvalidBytes replaces each byte of invalid UTF-8 in s with U+FFFD.
func replaceInvalidBytes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
