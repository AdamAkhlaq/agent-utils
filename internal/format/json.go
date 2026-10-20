package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// JSON pretty-prints the JSON document in r into w, using indent as the
// per-level indentation. It reformats the raw bytes rather than unmarshalling,
// so key order and exact number representation survive untouched.
func JSON(w io.Writer, r io.Reader, indent string) error {
	data, err := readInput(r)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", indent); err != nil {
		return syntaxError(data, err)
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// JSONCompact minifies the JSON document in r into w, preserving key order
// and number representation like JSON does.
func JSONCompact(w io.Writer, r io.Reader) error {
	data, err := readInput(r)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := compact(&buf, data); err != nil {
		return err
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// ErrInvalidJSON is wrapped by every error that reports malformed JSON, which
// sets a bad document apart from a failure to read it.
var ErrInvalidJSON = errors.New("invalid JSON")

// JSONValid checks that r holds a single syntactically valid JSON document,
// returning nil or an error pinpointing the first problem.
func JSONValid(r io.Reader) error {
	return JSONCompact(io.Discard, r)
}

// checkSyntax reports whether data holds exactly one JSON document, so
// trailing data fails too, with the line and column of the first problem.
func checkSyntax(data []byte) error {
	var scratch bytes.Buffer
	return compact(&scratch, data)
}

// compact minifies the single JSON document in data into buf.
func compact(buf *bytes.Buffer, data []byte) error {
	if err := json.Compact(buf, data); err != nil {
		// json.Compact reports every SyntaxError at offset 0 (its scanner
		// doesn't count bytes, unlike json.Indent's validation pass), so
		// re-run json.Indent on the failure path to recover the position.
		var scratch bytes.Buffer
		if ierr := json.Indent(&scratch, data, "", ""); ierr != nil {
			err = ierr
		}
		return syntaxError(data, err)
	}
	return nil
}

// readInput trims trailing whitespace, because json.Indent mirrors it from
// input to output and the CLI layer owns the final newline. Leading whitespace
// stays, so syntax-error offsets keep matching the input the user piped in.
func readInput(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return bytes.TrimRight(data, " \t\r\n"), nil
}

// syntaxError converts encoding/json's byte-offset errors into line and
// column, which is what a human scanning a file actually needs.
func syntaxError(data []byte, err error) error {
	var syn *json.SyntaxError
	if !errors.As(err, &syn) {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	line, col := lineCol(data, syn.Offset)
	return fmt.Errorf("%w at line %d, column %d: %w", ErrInvalidJSON, line, col, err)
}

func lineCol(data []byte, offset int64) (line, col int) {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	if offset < 0 {
		offset = 0
	}
	prefix := data[:offset]
	line = 1 + bytes.Count(prefix, []byte{'\n'})
	col = len(prefix) - (bytes.LastIndexByte(prefix, '\n') + 1)
	if col == 0 {
		col = 1
	}
	return line, col
}

// checkUTF8 reports the position of the first invalid UTF-8 sequence in data,
// which encoding/json would otherwise silently decode as U+FFFD.
func checkUTF8(data []byte) error {
	if utf8.Valid(data) {
		return nil
	}
	offset := 0
	for offset < len(data) {
		r, size := utf8.DecodeRune(data[offset:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		offset += size
	}
	line, col := lineCol(data, int64(offset+1))
	return fmt.Errorf("invalid UTF-8 at line %d, column %d", line, col)
}

// nextTokenOffset returns the byte offset in data where dec's next token
// starts: the decoder's offset sits just past the previous token, before any
// separator.
func nextTokenOffset(data []byte, dec *json.Decoder) int64 {
	off := dec.InputOffset()
	for off < int64(len(data)) && strings.IndexByte(" \t\r\n,:", data[off]) >= 0 {
		off++
	}
	return off
}
