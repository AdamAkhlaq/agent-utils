package format

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if err := json.Compact(&buf, data); err != nil {
		// json.Compact reports every SyntaxError at offset 0 (its scanner
		// doesn't count bytes, unlike json.Indent's validation pass), so
		// re-run json.Indent on the failure path to recover the position.
		var scratch bytes.Buffer
		if ierr := json.Indent(&scratch, data, "", ""); ierr != nil {
			err = ierr
		}
		return syntaxError(data, err)
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// JSONValid checks that r holds a single syntactically valid JSON document,
// returning nil or an error pinpointing the first problem.
func JSONValid(r io.Reader) error {
	return JSONCompact(io.Discard, r)
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
		return fmt.Errorf("invalid JSON: %w", err)
	}
	line, col := lineCol(data, syn.Offset)
	return fmt.Errorf("invalid JSON at line %d, column %d: %w", line, col, err)
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
