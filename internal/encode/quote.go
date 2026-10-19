package encode

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// JSONQuote returns s as a JSON string literal, quotes included. <, > and &
// stay literal rather than becoming HTML-safe \u escapes; U+2028 and U+2029
// are escaped, as encoding/json always does. Invalid UTF-8 is an error,
// where encoding/json would silently replace it with U+FFFD.
func JSONQuote(s string) (string, error) {
	if err := validUTF8(s); err != nil {
		return "", err
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return "", fmt.Errorf("encoding JSON string: %w", err)
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// jsonSpace is the whitespace RFC 8259 allows around a JSON value.
const jsonSpace = " \t\n\r"

// JSONUnquote decodes s, which must hold exactly one JSON string literal
// (surrounding whitespace allowed), to its raw text. It parses the literal
// itself because encoding/json is lenient in ways that would lose data
// silently: it decodes null into a string as "", and replaces invalid UTF-8
// and lone surrogate escapes such as \ud800 with U+FFFD. Here each of those
// is an error.
func JSONUnquote(s string) (string, error) {
	if err := validUTF8(s); err != nil {
		return "", err
	}
	start := len(s) - len(strings.TrimLeft(s, jsonSpace))
	lit := s[start:]
	if lit == "" {
		return "", errors.New("no JSON value in input")
	}
	if lit[0] != '"' {
		if json.Valid([]byte(lit)) {
			return "", fmt.Errorf("input is a JSON %s, not a string", jsonKind(lit[0]))
		}
		r, _ := utf8.DecodeRuneInString(lit)
		return "", fmt.Errorf("not a JSON string literal: unexpected %q at byte offset %d", r, start)
	}
	var b strings.Builder
	for i := 1; i < len(lit); {
		switch c := lit[i]; {
		case c == '"':
			if rest := strings.TrimLeft(lit[i+1:], jsonSpace); rest != "" {
				return "", fmt.Errorf("unexpected data after the JSON string at byte offset %d", len(s)-len(rest))
			}
			return b.String(), nil
		case c < 0x20:
			return "", fmt.Errorf("unescaped control character U+%04X at byte offset %d in JSON string", c, start+i)
		case c == '\\':
			n, err := readJSONEscape(&b, lit, i, start+i)
			if err != nil {
				return "", err
			}
			i += n
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", fmt.Errorf("unterminated JSON string starting at byte offset %d", start)
}

var jsonEscapes = map[byte]byte{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}

// readJSONEscape appends the value of the escape sequence at s[i] to b and
// returns its length in bytes. offset is its position in the whole input,
// for errors.
func readJSONEscape(b *strings.Builder, s string, i, offset int) (int, error) {
	if i+1 == len(s) {
		return 0, fmt.Errorf("unterminated escape sequence at byte offset %d in JSON string", offset)
	}
	if c := s[i+1]; c != 'u' {
		v, ok := jsonEscapes[c]
		if !ok {
			r, _ := utf8.DecodeRuneInString(s[i+1:])
			return 0, fmt.Errorf("invalid escape sequence %q at byte offset %d in JSON string", `\`+string(r), offset)
		}
		b.WriteByte(v)
		return 2, nil
	}
	r, ok := hex4(s[i+2:])
	switch {
	case !ok:
		return 0, fmt.Errorf(`\u escape without four hex digits at byte offset %d in JSON string`, offset)
	case 0xD800 <= r && r <= 0xDBFF:
		if strings.HasPrefix(s[i+6:], `\u`) {
			if low, ok := hex4(s[i+8:]); ok && 0xDC00 <= low && low <= 0xDFFF {
				b.WriteRune(utf16.DecodeRune(r, low))
				return 12, nil
			}
		}
		return 0, fmt.Errorf(`lone surrogate \u%s at byte offset %d in JSON string: no low surrogate follows it, so it is not valid Unicode`, s[i+2:i+6], offset)
	case 0xDC00 <= r && r <= 0xDFFF:
		return 0, fmt.Errorf(`lone surrogate \u%s at byte offset %d in JSON string: no high surrogate precedes it, so it is not valid Unicode`, s[i+2:i+6], offset)
	}
	b.WriteRune(r)
	return 6, nil
}

// hex4 parses the four hex digits that start s.
func hex4(s string) (rune, bool) {
	if len(s) < 4 {
		return 0, false
	}
	var r rune
	for _, c := range []byte(s[:4]) {
		switch {
		case '0' <= c && c <= '9':
			r = r<<4 | rune(c-'0')
		case 'a' <= c && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case 'A' <= c && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return r, true
}

// jsonKind names the type of the valid JSON value whose first byte is c.
func jsonKind(c byte) string {
	switch c {
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&#34;",
	"'", "&#39;",
	"\r", "&#13;",
)

// HTMLEscape escapes s for HTML text content or a quoted attribute value:
// & < > " ' become &amp; &lt; &gt; &#34; &#39;. A carriage return becomes
// &#13; as well, because an HTML parser turns a literal CR or CRLF into LF
// but keeps a CR written as a character reference. NUL is an error, since
// parsers drop it or replace it with U+FFFD, and so is invalid UTF-8.
func HTMLEscape(s string) (string, error) {
	if err := validUTF8(s); err != nil {
		return "", err
	}
	if i := strings.IndexByte(s, 0); i >= 0 {
		return "", fmt.Errorf("NUL byte at byte offset %d cannot be represented in HTML", i)
	}
	return htmlEscaper.Replace(s), nil
}

// HTMLUnescape decodes every HTML5 named and numeric character reference in
// s, following the HTML5 rules for text content (html.UnescapeString), and
// errors on invalid UTF-8.
func HTMLUnescape(s string) (string, error) {
	if err := validUTF8(s); err != nil {
		return "", err
	}
	return html.UnescapeString(s), nil
}

// RegexEscape escapes every regular expression metacharacter in s
// (regexp.QuoteMeta), so the result matches s literally. Invalid UTF-8 is an
// error, because RE2 rejects it in a pattern.
func RegexEscape(s string) (string, error) {
	if err := validUTF8(s); err != nil {
		return "", err
	}
	return regexp.QuoteMeta(s), nil
}

// validUTF8 rejects s unless it is valid UTF-8, reporting the byte offset of
// the first invalid sequence.
func validUTF8(s string) error {
	if utf8.ValidString(s) {
		return nil
	}
	for i, r := range s {
		if r == utf8.RuneError {
			if _, size := utf8.DecodeRuneInString(s[i:]); size == 1 {
				return fmt.Errorf("invalid UTF-8 at byte offset %d (0x%02x)", i, s[i])
			}
		}
	}
	return nil
}
