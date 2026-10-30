package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Pretty-printed JSON indents every line by its nesting depth, so its size
// grows with depth times indent width for each byte of input: 20 KB of
// brackets nested 9,999 deep is 200 MB at two spaces. Output may be the
// larger of outputFloor and outputFactor times the input size, so no input
// under 1 MiB can fail unless it is built to, and the size is checked before
// any output is written.
const (
	outputFloor  = 64 << 20
	outputFactor = 64
)

// ErrOutputLimit is wrapped by every error that reports pretty-printed output
// over its size limit.
var ErrOutputLimit = errors.New("output size limit")

func outputLimit(floor, inputSize int) int64 {
	return max(int64(floor), outputFactor*int64(inputSize))
}

func outputLimitError(floor int, limit int64, nesting, indentWidth int) error {
	spaces := "spaces"
	if indentWidth == 1 {
		spaces = "space"
	}
	return fmt.Errorf("%w: pretty-printed output would exceed %s (the larger of %s and %d times the input size): the JSON nests %d levels deep, with %d %s of indentation per level",
		ErrOutputLimit, byteCount(int(limit)), byteCount(floor), outputFactor, nesting, indentWidth, spaces)
}

// IndentJSON writes the valid JSON document doc to w, indented by indent per
// level exactly as json.Indent would, after checking that the result fits
// the output limit for an input of inputSize bytes. The output is streamed
// rather than built in memory, so once the check passes only a failed write
// can stop it.
func IndentJSON(w io.Writer, doc []byte, indent string, inputSize int) error {
	return indentJSON(w, doc, indent, outputFloor, inputSize)
}

// indentJSON is IndentJSON with the limit's floor as a parameter, so tests
// can reach its edge without writing 64 MiB.
func indentJSON(w io.Writer, doc []byte, indent string, floor, inputSize int) error {
	size, nesting := indentedSize(doc, len(indent))
	if limit := outputLimit(floor, inputSize); size > limit {
		return outputLimitError(floor, limit, nesting, len(indent))
	}
	if err := writeIndented(w, doc, indent); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// indentedSize returns the length of json.Indent's output for the valid JSON
// document doc, and how deeply its objects and arrays nest, without building
// it. It follows the same rules as writeIndented, which follows json.Indent
// byte for byte: leading whitespace is dropped and trailing whitespace kept,
// an empty object or array stays on one line, and every other line starts
// with one indent per level.
func indentedSize(doc []byte, indentLen int) (size int64, nesting int) {
	newline := func(depth int) int64 { return 1 + int64(depth)*int64(indentLen) }
	depth, open := 0, 0
	started, needIndent := false, false
	for i := 0; i < len(doc); {
		c := doc[i]
		if isJSONSpace(c) {
			if started && open == 0 {
				size++
			}
			i++
			continue
		}
		started = true
		if needIndent && c != '}' && c != ']' {
			needIndent = false
			depth++
			size += newline(depth)
		}
		end := i + 1
		switch c {
		case '"':
			end = stringEnd(doc, i)
		case '{', '[':
			needIndent = true
			open++
			nesting = max(nesting, open)
		case ',':
			size += newline(depth)
		case ':':
			size++
		case '}', ']':
			open--
			if needIndent {
				needIndent = false
			} else {
				depth--
				size += newline(depth)
			}
		default:
			end = literalEnd(doc, i)
		}
		size += int64(end - i)
		i = end
	}
	return size, nesting
}

// writeIndented writes the valid JSON document doc to w as json.Indent would
// with an empty prefix, following the same rules as indentedSize. It builds
// the output in a buffer of its own, written to w whenever it fills.
func writeIndented(w io.Writer, doc []byte, indent string) error {
	const chunk = 64 << 10
	buf := make([]byte, 0, chunk+1024)
	// lines holds a line break and enough indents for most depths, so a new
	// line is usually one append.
	lines := "\n" + strings.Repeat(indent, 32)
	newline := func(depth int) {
		if n := 1 + depth*len(indent); n <= len(lines) {
			buf = append(buf, lines[:n]...)
			return
		}
		buf = append(buf, lines...)
		for range depth - 32 {
			buf = append(buf, indent...)
		}
	}
	depth, open := 0, 0
	started, needIndent := false, false
	for i := 0; i < len(doc); {
		if len(buf) >= chunk {
			if _, err := w.Write(buf); err != nil {
				return err
			}
			buf = buf[:0]
		}
		c := doc[i]
		if isJSONSpace(c) {
			if started && open == 0 {
				buf = append(buf, c)
			}
			i++
			continue
		}
		started = true
		if needIndent && c != '}' && c != ']' {
			needIndent = false
			depth++
			newline(depth)
		}
		end := i + 1
		switch c {
		case '"':
			end = stringEnd(doc, i)
			buf = append(buf, doc[i:end]...)
		case '{', '[':
			needIndent = true
			open++
			buf = append(buf, c)
		case ',':
			buf = append(buf, c)
			newline(depth)
		case ':':
			buf = append(buf, ':', ' ')
		case '}', ']':
			open--
			if needIndent {
				needIndent = false
			} else {
				depth--
				newline(depth)
			}
			buf = append(buf, c)
		default:
			end = literalEnd(doc, i)
			buf = append(buf, doc[i:end]...)
		}
		i = end
	}
	_, err := w.Write(buf)
	return err
}

// stringEnd returns the index just past the JSON string whose opening quote
// is doc[start]; doc must be valid, so the string is closed. A quote ends it
// unless an odd number of backslashes escapes it.
func stringEnd(doc []byte, start int) int {
	i := start + 1
	for {
		j := bytes.IndexByte(doc[i:], '"')
		if j < 0 {
			return len(doc)
		}
		i += j
		backslashes := 0
		for k := i - 1; k > start && doc[k] == '\\'; k-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return i + 1
		}
		i++
	}
}

// literalEnd returns the index just past the number, true, false, or null
// that starts at doc[start].
func literalEnd(doc []byte, start int) int {
	i := start + 1
	for i < len(doc) {
		switch doc[i] {
		case ',', ']', '}', ' ', '\t', '\r', '\n':
			return i
		}
		i++
	}
	return i
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// writeJSON encodes v compactly, with HTML characters unescaped, and writes
// it to w pretty-printed with two spaces per level through IndentJSON.
func writeJSON(w io.Writer, v any, inputSize int) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return IndentJSON(w, bytes.TrimSuffix(buf.Bytes(), []byte("\n")), "  ", inputSize)
}
