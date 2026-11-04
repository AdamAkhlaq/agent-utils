package format

import (
	"bytes"
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

// FuzzAppendJSONString holds appendJSONString to the output of a json.Encoder
// with HTML escaping off on every string. For invalid UTF-8 it compares with
// the string whose invalid bytes are already U+FFFD, where encoding/json
// agrees across Go releases.
func FuzzAppendJSONString(f *testing.F) {
	for _, s := range jsonStringSeeds {
		f.Add(s)
	}
	f.Fuzz(checkJSONString)
}

func checkJSONString(t *testing.T, s string) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(replaceInvalidBytes(s)); err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(buf.String(), "\n")
	if got := appendJSONString([]byte("x"), s); string(got) != "x"+want {
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
