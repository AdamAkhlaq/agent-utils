package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// JSONToYAML converts the JSON document in r to block-style YAML in w, the
// inverse of YAMLToJSON. It writes YAML straight from the JSON tokens, so key
// order and the digits of every number survive, and a string is written
// plain only when no YAML 1.1 or 1.2 reader could take it for anything but
// that string. Like every converter here it leaves the final line break to
// the CLI, and a literal block that ends the document needs it to keep its
// own final line break. Nothing is written to w unless the whole conversion
// succeeds.
func JSONToYAML(w io.Writer, r io.Reader) error {
	data, err := readInput(r)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("empty input")
	}
	if err := checkSyntax(data); err != nil {
		return err
	}
	e := yamlEmitter{data: data, dec: json.NewDecoder(bytes.NewReader(data))}
	e.dec.UseNumber()
	if err := e.value(0, atRoot); err != nil {
		return err
	}
	if _, err := w.Write(e.buf.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// Where a value is written: at the start of the document, after "key:", or
// after a "- " or ": " indicator. The indentation passed alongside is that of
// the key or indicator.
type yamlPlace int

const (
	atRoot yamlPlace = iota
	afterKey
	afterIndicator
)

// maxImplicitKey is the longest key YAML allows without the explicit "? "
// form.
const maxImplicitKey = 1024

type yamlEmitter struct {
	data []byte
	dec  *json.Decoder
	buf  bytes.Buffer
	// quoteNext is set for the value of a tagged or explicit key, where
	// go-yaml cannot read a literal block.
	quoteNext bool
}

func (e *yamlEmitter) value(indent int, at yamlPlace) error {
	quote := e.quoteNext
	e.quoteNext = false
	start := nextTokenOffset(e.data, e.dec)
	tok, err := e.dec.Token()
	if err != nil {
		return syntaxError(e.data, err)
	}
	switch tok := tok.(type) {
	case json.Delim:
		if tok == '{' {
			return e.mapping(indent, at)
		}
		return e.sequence(indent, at)
	case string:
		if err := e.checkString(tok, start); err != nil {
			return err
		}
		e.string(tok, indent, at, quote)
	case json.Number:
		e.scalar(yamlNumber(tok.String()), at)
	case bool:
		e.scalar(fmt.Sprint(tok), at)
	case nil:
		e.scalar("null", at)
	}
	return nil
}

func (e *yamlEmitter) mapping(indent int, at yamlPlace) error {
	if !e.dec.More() {
		e.scalar("{}", at)
		return e.closing()
	}
	if at != atRoot {
		indent += 2
	}
	seen := map[string]bool{}
	for first := true; e.dec.More(); first = false {
		start := nextTokenOffset(e.data, e.dec)
		tok, err := e.dec.Token()
		if err != nil {
			return syntaxError(e.data, err)
		}
		key, _ := tok.(string)
		if err := e.checkString(key, start); err != nil {
			return err
		}
		if seen[key] {
			line, col := lineCol(e.data, start+1)
			return fmt.Errorf("duplicate key %q at line %d, column %d", key, line, col)
		}
		seen[key] = true
		if !first || at == afterKey {
			e.newline(indent)
		}
		if err := e.entry(key, indent); err != nil {
			return err
		}
	}
	return e.closing()
}

func (e *yamlEmitter) entry(key string, indent int) error {
	k := yamlKey(key)
	e.quoteNext = key == "<<"
	if utf8.RuneCountInString(k) > maxImplicitKey {
		e.quoteNext = true
		e.buf.WriteString("? " + k)
		e.newline(indent)
		e.buf.WriteString(": ")
		return e.value(indent, afterIndicator)
	}
	e.buf.WriteString(k + ":")
	return e.value(indent, afterKey)
}

// sequence writes its items at the indentation of the key that holds it, the
// compact style go-yaml and most hand-written YAML use.
func (e *yamlEmitter) sequence(indent int, at yamlPlace) error {
	if !e.dec.More() {
		e.scalar("[]", at)
		return e.closing()
	}
	if at == afterIndicator {
		indent += 2
	}
	for first := true; e.dec.More(); first = false {
		if !first || at == afterKey {
			e.newline(indent)
		}
		e.buf.WriteString("- ")
		if err := e.value(indent, afterIndicator); err != nil {
			return err
		}
	}
	return e.closing()
}

// checkString rejects a string whose source escapes a lone UTF-16 surrogate,
// which encoding/json decodes as U+FFFD without a word. start is the offset
// of the string's opening quote.
func (e *yamlEmitter) checkString(s string, start int64) error {
	if !strings.ContainsRune(s, utf8.RuneError) {
		return nil
	}
	if esc, ok := loneSurrogate(e.data[start:e.dec.InputOffset()]); ok {
		line, col := lineCol(e.data, start+1)
		return fmt.Errorf("string at line %d, column %d escapes a lone UTF-16 surrogate (%s), which has no UTF-8 form", line, col, esc)
	}
	return nil
}

// loneSurrogate returns the first \uXXXX escape in the valid JSON string
// literal raw that is a surrogate outside a high-low pair.
func loneSurrogate(raw []byte) (string, bool) {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		r := hexRune(raw[i+1 : i+5])
		if !utf16.IsSurrogate(r) {
			i += 4
			continue
		}
		if r < 0xdc00 && i+10 < len(raw) && raw[i+5] == '\\' && raw[i+6] == 'u' {
			if low := hexRune(raw[i+7 : i+11]); low >= 0xdc00 && low < 0xe000 {
				i += 10
				continue
			}
		}
		return string(raw[i-1 : i+5]), true
	}
	return "", false
}

func hexRune(hex []byte) rune {
	var r rune
	for _, c := range hex {
		switch {
		case c >= '0' && c <= '9':
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		default:
			r = r<<4 | rune(c-'A'+10)
		}
	}
	return r
}

func (e *yamlEmitter) closing() error {
	if _, err := e.dec.Token(); err != nil {
		return syntaxError(e.data, err)
	}
	return nil
}

func (e *yamlEmitter) scalar(s string, at yamlPlace) {
	if at == afterKey {
		e.buf.WriteByte(' ')
	}
	e.buf.WriteString(s)
}

// string writes s as a literal block when it spans lines, reads back exactly
// that way, and quote is false, and as a plain or double-quoted scalar
// otherwise.
func (e *yamlEmitter) string(s string, indent int, at yamlPlace, quote bool) {
	chomp, ok := literalChomp(s)
	if !ok || quote {
		e.scalar(yamlString(s), at)
		return
	}
	e.scalar("|"+chomp, at)
	if chomp != "-" {
		s = s[:len(s)-1]
	}
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			e.buf.WriteByte('\n')
		} else {
			e.newline(indent + 2)
			e.buf.WriteString(line)
		}
	}
}

func (e *yamlEmitter) newline(indent int) {
	e.buf.WriteByte('\n')
	for range indent {
		e.buf.WriteByte(' ')
	}
}

// yamlNumber returns a JSON number as YAML. JSON's number syntax is a subset
// of YAML 1.2's, but YAML 1.1 readers (go-yaml, PyYAML, and so YAMLToJSON)
// only read a float with a decimal point and a signed exponent, so 1e3 is
// written 1.0e+3. The digits are otherwise kept as written, never rounded.
func yamlNumber(n string) string {
	i := strings.IndexAny(n, "eE")
	if i < 0 {
		return n
	}
	mantissa, exponent := n[:i], n[i+1:]
	if !strings.Contains(mantissa, ".") {
		mantissa += ".0"
	}
	if exponent[0] != '+' && exponent[0] != '-' {
		exponent = "+" + exponent
	}
	return mantissa + n[i:i+1] + exponent
}

// yamlKey returns key as a mapping key that no reader takes for a merge key.
func yamlKey(key string) string {
	switch {
	case key == "<<":
		// Quoting alone is not enough: Ruby's Psych reads a quoted "<<" key
		// as a merge key too.
		return `!!str "<<"`
	case strings.HasSuffix(key, "<<"):
		// go-yaml reads a plain key ending in "<<" as a merge key.
		return yamlQuoted(key)
	}
	return yamlString(key)
}

// yamlString returns s as a plain scalar when that is safe, and double-quoted
// otherwise.
func yamlString(s string) string {
	if plainSafe(s) {
		return s
	}
	return yamlQuoted(s)
}

func yamlQuoted(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if yamlPrintable(r) {
				b.WriteRune(r)
			} else {
				// Every rune yamlPrintable rejects is in the Basic
				// Multilingual Plane.
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// yamlKeywords are the plain scalars that YAML 1.1 or 1.2 reads as a boolean,
// null, or special float, matched in any case.
var yamlKeywords = map[string]bool{
	"y": true, "yes": true, "n": true, "no": true, "on": true, "off": true,
	"true": true, "false": true, "null": true, ".inf": true, ".nan": true,
}

// plainSafe reports whether s reads back as exactly s when written plain.
// Rather than mirror every reader's resolution rules, it allows only text
// that none of them treat specially: s starts as plainStart allows, is not a
// keyword in any case, and continues with printable text without a tab, a
// ": " or " #" (which end a plain scalar), or a trailing space or ":".
func plainSafe(s string) bool {
	if !plainStart(s) || s[len(s)-1] == ' ' || yamlKeywords[strings.ToLower(s)] {
		return false
	}
	var prev rune
	for i, r := range s {
		switch {
		case r == '\t' || !yamlPrintable(r):
			return false
		case r == ':' && (i+1 == len(s) || s[i+1] == ' '):
			return false
		case r == '#' && prev == ' ':
			return false
		}
		prev = r
	}
	return true
}

// plainStart reports whether s starts with a letter, one of "_/$^(", "./",
// "../", or a dot before a letter (".github"): never a digit, sign, or YAML
// indicator, so s cannot be a number, date, alias, tag, document marker, or
// merge key.
func plainStart(s string) bool {
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}
	first, size := utf8.DecodeRuneInString(s)
	if first == '.' {
		next, _ := utf8.DecodeRuneInString(s[size:])
		return unicode.IsLetter(next)
	}
	return unicode.IsLetter(first) || strings.ContainsRune("_/$^(", first)
}

// literalChomp returns the chomping indicator for writing s as a literal
// block scalar, and false when s is better quoted: a single line, a first
// non-empty line that starts with whitespace (which a reader would take for
// indentation), a line that ends in whitespace (which editors strip, and
// go-yaml drops from the last line), or a character that is not printable or
// that a YAML 1.1 reader treats as a line break.
func literalChomp(s string) (string, bool) {
	body := strings.TrimRight(s, "\n")
	if first := strings.TrimLeft(body, "\n"); !strings.Contains(body, "\n") || first[0] == ' ' || first[0] == '\t' {
		return "", false
	}
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			return "", false
		}
	}
	for _, r := range body {
		if r != '\n' && r != '\t' && !yamlPrintable(r) {
			return "", false
		}
	}
	switch len(s) - len(body) {
	case 0:
		return "-", true
	case 1:
		return "", true
	default:
		return "+", true
	}
}

// yamlPrintable reports whether r may appear unescaped in a YAML scalar on a
// single line: YAML's printable set, minus the characters YAML 1.1 treats as
// line breaks (U+0085, U+2028, U+2029) and the byte order mark.
func yamlPrintable(r rune) bool {
	switch {
	case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
		return false
	case r == 0x2028 || r == 0x2029 || r == 0xfeff || r == 0xfffe || r == 0xffff:
		return false
	}
	return true
}
