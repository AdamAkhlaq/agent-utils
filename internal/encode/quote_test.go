package encode

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// quoteTest is one case for a func(string) (string, error) quoting function:
// either want or a substring of the expected error.
type quoteTest struct {
	name    string
	input   string
	want    string
	wantErr string
}

func runQuoteTests(t *testing.T, fnName string, fn func(string) (string, error), tests []quoteTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fn(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("%s() = %q, error = %v, want an error containing %q", fnName, got, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s() error = %v", fnName, err)
			}
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", fnName, got, tt.want)
			}
		})
	}
}

func TestJSONQuote(t *testing.T) {
	runQuoteTests(t, "JSONQuote", JSONQuote, []quoteTest{
		{name: "plain", input: "abc", want: `"abc"`},
		{name: "empty string", input: "", want: `""`},
		{name: "quotes and backslash", input: `say "hi" \o/`, want: `"say \"hi\" \\o/"`},
		{name: "control characters", input: "\b\f\n\r\t\x00\x1f", want: `"\b\f\n\r\t\u0000\u001f"`},
		{name: "HTML characters stay literal", input: "<a href='x'>&</a>", want: `"<a href='x'>&</a>"`},
		{name: "unicode stays literal", input: "café 😀", want: `"café 😀"`},
		{name: "line and paragraph separators are escaped", input: "\u2028\u2029", want: `"\u2028\u2029"`},
		{name: "DEL stays literal", input: "\x7f", want: "\"\x7f\""},
		{name: "invalid UTF-8", input: "ab\xff", wantErr: "invalid UTF-8 at byte offset 2 (0xff)"},
	})
}

func TestJSONUnquote(t *testing.T) {
	runQuoteTests(t, "JSONUnquote", JSONUnquote, []quoteTest{
		{name: "plain", input: `"abc"`, want: "abc"},
		{name: "empty string", input: `""`, want: ""},
		{name: "every short escape", input: `"\"\\\/\b\f\n\r\t"`, want: "\"\\/\b\f\n\r\t"},
		{name: "unicode escapes", input: `"caf\u00e9 \u00E9"`, want: "café é"},
		{name: "NUL escape", input: `"a\u0000b"`, want: "a\x00b"},
		{name: "surrogate pair", input: `"\ud83d\ude00"`, want: "😀"},
		{name: "literal unicode", input: `"☕"`, want: "☕"},
		{name: "escaped replacement character is kept", input: `"\ufffd"`, want: "\ufffd"},
		{name: "surrounding whitespace", input: " \t\r\n\"a\"\n ", want: "a"},
		{name: "empty input", input: "", wantErr: "no JSON value in input"},
		{name: "only whitespace", input: " \n", wantErr: "no JSON value in input"},
		{name: "null", input: "null", wantErr: "input is a JSON null, not a string"},
		{name: "number", input: "-1.5", wantErr: "input is a JSON number, not a string"},
		{name: "boolean", input: "true", wantErr: "input is a JSON boolean, not a string"},
		{name: "object", input: `{"a":"b"}`, wantErr: "input is a JSON object, not a string"},
		{name: "array", input: ` ["a"]`, wantErr: "input is a JSON array, not a string"},
		{name: "unquoted text", input: " abc", wantErr: `not a JSON string literal: unexpected 'a' at byte offset 1`},
		{name: "single quotes", input: "'a'", wantErr: "not a JSON string literal"},
		{name: "two strings", input: `"a" "b"`, wantErr: "unexpected data after the JSON string at byte offset 4"},
		{name: "unterminated", input: `"abc`, wantErr: "unterminated JSON string starting at byte offset 0"},
		{name: "unterminated after escaped quote", input: `"abc\"`, wantErr: "unterminated JSON string"},
		{name: "unterminated escape", input: `"abc\`, wantErr: "unterminated escape sequence at byte offset 4"},
		{name: "escaped space before the end", input: `"a\ `, wantErr: `invalid escape sequence "\\ " at byte offset 2`},
		{name: "unterminated with trailing spaces", input: "\"a  ", wantErr: "unterminated JSON string starting at byte offset 0"},
		{name: "raw newline", input: "\"a\nb\"", wantErr: "unescaped control character U+000A at byte offset 2"},
		{name: "invalid escape", input: `"a\x41"`, wantErr: `invalid escape sequence "\\x" at byte offset 2`},
		{name: "short unicode escape", input: `"\u12"`, wantErr: `\u escape without four hex digits at byte offset 1`},
		{name: "non-hex unicode escape", input: `"\u12g4"`, wantErr: `\u escape without four hex digits`},
		{name: "lone high surrogate", input: `"\ud800"`, wantErr: `lone surrogate \ud800 at byte offset 1 in JSON string: no low surrogate follows it`},
		{name: "high surrogate before a non-surrogate", input: `"\ud800\u0041"`, wantErr: "lone surrogate"},
		{name: "two high surrogates", input: `"\ud800\ud800"`, wantErr: "lone surrogate"},
		{name: "lone low surrogate", input: `"\uDC00"`, wantErr: `lone surrogate \uDC00 at byte offset 1 in JSON string: no high surrogate precedes it`},
		{name: "invalid UTF-8", input: "\"a\xff\"", wantErr: "invalid UTF-8 at byte offset 2"},
	})
}

func TestHTMLEscape(t *testing.T) {
	runQuoteTests(t, "HTMLEscape", HTMLEscape, []quoteTest{
		{name: "the five special characters", input: `<a href="x" title='y'>&</a>`, want: "&lt;a href=&#34;x&#34; title=&#39;y&#39;&gt;&amp;&lt;/a&gt;"},
		{name: "existing entity is escaped again", input: "&amp;", want: "&amp;amp;"},
		{name: "empty string", input: "", want: ""},
		{name: "unicode stays literal", input: "café ☕", want: "café ☕"},
		{name: "carriage return becomes a reference", input: "a\r\nb", want: "a&#13;\nb"},
		{name: "NUL", input: "a\x00", wantErr: "NUL byte at byte offset 1 cannot be represented in HTML"},
		{name: "invalid UTF-8", input: "\xc3", wantErr: "invalid UTF-8 at byte offset 0 (0xc3)"},
	})
}

func TestHTMLUnescape(t *testing.T) {
	runQuoteTests(t, "HTMLUnescape", HTMLUnescape, []quoteTest{
		{name: "named references", input: "&lt;&gt;&amp;&quot;&apos;&eacute;", want: `<>&"'é`},
		{name: "numeric references", input: "&#34;&#x27;&#X1F600;&#13;", want: "\"'😀\r"},
		{name: "empty string", input: "", want: ""},
		{name: "unknown entity is kept", input: "&nosuch;", want: "&nosuch;"},
		{name: "legacy entity without semicolon", input: "&copy 2026", want: "© 2026"},
		{name: "windows-1252 numeric range", input: "&#128;", want: "€"},
		{name: "NUL reference becomes the replacement character", input: "&#0;", want: "\ufffd"},
		{name: "invalid UTF-8", input: "a\xff", wantErr: "invalid UTF-8 at byte offset 1"},
	})
}

func TestRegexEscape(t *testing.T) {
	runQuoteTests(t, "RegexEscape", RegexEscape, []quoteTest{
		{name: "every metacharacter", input: `\.+*?()|[]{}^$`, want: `\\\.\+\*\?\(\)\|\[\]\{\}\^\$`},
		{name: "other punctuation stays bare", input: "a-b/c#d e'f\"g", want: "a-b/c#d e'f\"g"},
		{name: "empty string", input: "", want: ""},
		{name: "unicode stays literal", input: "café", want: "café"},
		{name: "invalid UTF-8", input: "\xff", wantErr: "invalid UTF-8 at byte offset 0"},
	})
}

func FuzzJSONRoundTrip(f *testing.F) {
	for _, s := range []string{"", "abc", "\"\\", "\x00\n\u2028", "<&>", "😀", "\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		quoted, err := JSONQuote(s)
		if !utf8.ValidString(s) {
			if err == nil {
				t.Fatalf("JSONQuote(%q) accepted invalid UTF-8", s)
			}
			return
		}
		if err != nil {
			t.Fatalf("JSONQuote(%q) error = %v", s, err)
		}
		var std string
		if err := json.Unmarshal([]byte(quoted), &std); err != nil || std != s {
			t.Fatalf("encoding/json decodes %q to %q, %v, want %q", quoted, std, err, s)
		}
		if got, err := JSONUnquote(quoted); err != nil || got != s {
			t.Fatalf("JSONUnquote(%q) = %q, %v, want %q", quoted, got, err, s)
		}
	})
}

// JSONUnquote must agree with encoding/json wherever both succeed, and may
// only refuse what encoding/json accepts when that is a lone surrogate,
// which encoding/json replaces with U+FFFD.
func FuzzJSONUnquote(f *testing.F) {
	for _, s := range []string{`"a"`, `"\u00e9\ud83d\ude00"`, `"\ud800"`, `null`, ` "a" `, `"a`, `"\x"`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, err := JSONUnquote(s)
		var v any
		stdErr := json.Unmarshal([]byte(s), &v)
		std, isString := v.(string)
		stdOK := stdErr == nil && isString && utf8.ValidString(s)
		switch {
		case err == nil && !stdOK:
			t.Fatalf("JSONUnquote(%q) = %q, but encoding/json gives %#v, %v", s, got, v, stdErr)
		case err == nil && got != std:
			t.Fatalf("JSONUnquote(%q) = %q, encoding/json gives %q", s, got, std)
		case err != nil && stdOK && !strings.Contains(err.Error(), "lone surrogate"):
			t.Fatalf("JSONUnquote(%q) error = %v, but encoding/json decodes it to %q", s, err, std)
		}
	})
}

func FuzzHTMLRoundTrip(f *testing.F) {
	for _, s := range []string{"", "<a>&amp;</a>", "'\"", "\r\n", "&#0;", "\x00", "\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		escaped, err := HTMLEscape(s)
		if !utf8.ValidString(s) || strings.IndexByte(s, 0) >= 0 {
			if err == nil {
				t.Fatalf("HTMLEscape(%q) accepted invalid UTF-8 or NUL", s)
			}
			return
		}
		if err != nil {
			t.Fatalf("HTMLEscape(%q) error = %v", s, err)
		}
		if strings.ContainsAny(escaped, "<>\"'\r") {
			t.Fatalf("HTMLEscape(%q) = %q still holds a special character", s, escaped)
		}
		if got := html.UnescapeString(escaped); got != s {
			t.Fatalf("html.UnescapeString(%q) = %q, want %q", escaped, got, s)
		}
		if got, err := HTMLUnescape(escaped); err != nil || got != s {
			t.Fatalf("HTMLUnescape(%q) = %q, %v, want %q", escaped, got, err, s)
		}
	})
}

func FuzzRegexEscape(f *testing.F) {
	for _, s := range []string{"", "a.b", `\.+*?()|[]{}^$`, "\n", "\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		pattern, err := RegexEscape(s)
		if !utf8.ValidString(s) {
			if err == nil {
				t.Fatalf("RegexEscape(%q) accepted invalid UTF-8", s)
			}
			return
		}
		if err != nil {
			t.Fatalf("RegexEscape(%q) error = %v", s, err)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Fatalf("RegexEscape(%q) = %q, which does not compile: %v", s, pattern, err)
		}
		if prefix, complete := re.LiteralPrefix(); !complete || prefix != s {
			t.Fatalf("RegexEscape(%q) = %q, which is not the literal %q", s, pattern, s)
		}
	})
}
