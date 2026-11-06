package format

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

func TestCheckJSONSurrogates(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "no strings", input: `[1,true,null]`},
		{name: "no escapes", input: `{"a":"b"}`},
		{name: "other escapes", input: `["@u00e9@n@t@"@@@/@b@f@r"]`},
		{name: "surrogate pair", input: `"@ud83d@ude00"`},
		{name: "uppercase surrogate pair", input: `"@uD83D@uDE00"`},
		{name: "highest pair", input: `"@udbff@udfff"`},
		{name: "replacement character escape", input: `"@ufffd"`},
		{name: "raw replacement character", input: `"` + "\uFFFD" + `"`},
		{name: "escaped backslash before u", input: `"@@udc00"`},
		{name: "two escaped backslashes before u", input: `"@@@@ud800"`},
		{name: "empty input", input: ``},

		{name: "lone high surrogate", input: `{"a":"@ud800"}`, wantErr: `string at line 1, column 6 escapes a lone UTF-16 surrogate (@ud800), which has no UTF-8 form`},
		{name: "lone low surrogate", input: `{"a":"x@uDC00y"}`, wantErr: `string at line 1, column 6 escapes a lone UTF-16 surrogate (@uDC00)`},
		{name: "escaped backslash then a lone surrogate", input: `"@@@ud800"`, wantErr: `column 1 escapes a lone UTF-16 surrogate (@ud800)`},
		{name: "high surrogate before a letter", input: `"@ud83d@u0041"`, wantErr: `(@ud83d)`},
		{name: "high surrogate before another high", input: `"@ud83d@ud83d@ude00"`, wantErr: `(@ud83d)`},
		{name: "high surrogate before a raw U+FFFD", input: `"@ud83d` + "\uFFFD" + `"`, wantErr: `(@ud83d)`},
		{name: "high surrogate at the end", input: `"ab@ud83d"`, wantErr: `(@ud83d)`},
		{name: "low then high", input: `"@ude00@ud83d"`, wantErr: `(@ude00)`},
		{name: "high surrogate after a pair", input: `{"a":"@ud83d@ude00@udbff"}`, wantErr: `(@udbff)`},
		{name: "lone surrogate in a key", input: `{"@ud83dz":1}`, wantErr: `string at line 1, column 2`},
		{name: "position after quotes and escapes", input: "[\"a@\"b\",\n  \"@@\", \"@\"@udfff\"]", wantErr: `string at line 2, column 9 escapes a lone UTF-16 surrogate (@udfff)`},
		{name: "first of two", input: `["@udc01","@ud801"]`, wantErr: `column 2 escapes a lone UTF-16 surrogate (@udc01)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.ReplaceAll(tt.input, "@", `\`)
			err := CheckJSONSurrogates([]byte(input))
			want := strings.ReplaceAll(tt.wantErr, "@", `\`)
			if want == "" {
				if err != nil {
					t.Fatalf("CheckJSONSurrogates(%s) error = %v", input, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("CheckJSONSurrogates(%s) error = %v, want it to contain %q", input, err, want)
			}
		})
	}
}

// FuzzCheckJSONSurrogates writes arbitrary UTF-16 code units as \uXXXX
// escapes and compares the check with unicode/utf16, which replaces exactly
// the lone surrogates with U+FFFD.
func FuzzCheckJSONSurrogates(f *testing.F) {
	f.Add([]byte{0xd8, 0x3d, 0xde, 0x00}, "a", false)
	f.Add([]byte{0xd8, 0x00}, "", true)
	f.Add([]byte{0xdc, 0x00, 0x00, 0x41}, `\\`, false)
	f.Add([]byte{0xff, 0xfd, 0xd8, 0x00, 0xd8, 0x00, 0xdc, 0x00}, "\n", true)
	f.Fuzz(func(t *testing.T, raw []byte, plain string, upper bool) {
		if !utf8.ValidString(plain) || strings.ContainsAny(plain, `"\`) || strings.ContainsFunc(plain, func(r rune) bool { return r < 0x20 }) {
			return
		}
		escape := `\u%04x`
		if upper {
			escape = `\u%04X`
		}
		units := make([]uint16, len(raw)/2)
		var b strings.Builder
		b.WriteString(`{"k":["` + plain + `\\`)
		for i := range units {
			units[i] = uint16(raw[2*i])<<8 | uint16(raw[2*i+1])
			fmt.Fprintf(&b, escape, units[i])
		}
		b.WriteString(`"]}`)
		replaced := 0
		for _, r := range utf16.Decode(units) {
			if r == utf8.RuneError {
				replaced++
			}
		}
		for _, u := range units {
			if u == 0xfffd {
				replaced--
			}
		}
		err := CheckJSONSurrogates([]byte(b.String()))
		if (err != nil) != (replaced > 0) {
			t.Fatalf("CheckJSONSurrogates(%s) error = %v, but utf16 replaced %d lone surrogates", b.String(), err, replaced)
		}
		if err != nil && !strings.Contains(err.Error(), "string at line 1, column 7 ") {
			t.Fatalf("CheckJSONSurrogates(%s) error = %v, want the string at column 7", b.String(), err)
		}
	})
}

// FuzzCheckJSONSurrogatesAnyInput checks that input that is not valid JSON
// cannot make the check panic or read out of range.
func FuzzCheckJSONSurrogatesAnyInput(f *testing.F) {
	for _, s := range []string{`"\ud800`, `\`, `\u`, `\ud8`, `"\ud800\u`, `"\ud800\udc`, `\\\`, `"\uzzzz"`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_ = CheckJSONSurrogates([]byte(input))
	})
}
