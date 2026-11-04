package format

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CSVToJSON converts CSV from r (first record is the header row, read by
// CSVReader) to a pretty-printed JSON array of objects in w, one object per
// data row with the headers as keys in column order. Every value stays a JSON
// string: CSV carries no type information, and inferring types corrupts data
// (a zip code "01234" must not become the number 1234). The JSON is assembled
// by hand because marshalling through a map would lose column order. Every
// row repeats every header name, so output is limited as IndentJSON limits
// it. Every record is read and the size checked before anything is written,
// so once writing starts only a failed write can stop it.
func CSVToJSON(w io.Writer, r io.Reader, sep rune) error {
	return csvToJSON(w, r, sep, outputFloor)
}

// csvToJSON is CSVToJSON with the output limit's floor as a parameter, so
// tests can reach its edge without writing 64 MiB.
func csvToJSON(w io.Writer, r io.Reader, sep rune, floor int) error {
	cr, err := NewCSVReader(r, sep)
	if err != nil {
		return err
	}
	headers, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("empty input")
		}
		return err
	}
	seen := make(map[string]bool, len(headers))
	keys := make([][]byte, len(headers))
	for i, h := range headers {
		if seen[h] {
			return fmt.Errorf("duplicate header %q", h)
		}
		seen[h] = true
		keys[i] = appendJSONString(nil, h)
	}
	// The rows are converted twice: first only counting the output, which
	// finds every malformed record and checks the size before anything is
	// written, and then writing it, which cannot fail on the same input
	// except in the write itself. Building the output in memory instead
	// would cost up to three times the limit as the buffer grows.
	body := cr.pos
	limit := outputLimit(floor, len(cr.data))
	counted := newCSVJSON(keys, nil, limit)
	if err := counted.array(cr); err != nil {
		return err
	}
	if counted.size > limit {
		return counted.limitError(floor)
	}
	cr.pos = body
	out := bufio.NewWriterSize(w, 64<<10)
	written := newCSVJSON(keys, out, limit)
	if err := written.array(cr); err != nil {
		return err
	}
	if err := out.Flush(); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// csvJSON writes CSV records as a pretty-printed JSON array of objects, or
// only counts its bytes when out is nil.
type csvJSON struct {
	keys  [][]byte // the header names, encoded as JSON strings
	out   *bufio.Writer
	err   error // the first failed write
	limit int64
	size  int64 // bytes of output so far, counted until they pass limit
	rows  int
	emit  func(string)
}

func newCSVJSON(keys [][]byte, out *bufio.Writer, limit int64) *csvJSON {
	j := &csvJSON{keys: keys, out: out, limit: limit}
	j.emit = j.writeString
	return j
}

// count adds n bytes to the size. It stops once the size passes the limit,
// so it cannot overflow: output can grow with the square of the input.
func (j *csvJSON) count(n int) {
	if j.size <= j.limit {
		j.size += int64(n)
	}
}

func (j *csvJSON) write(b []byte) {
	j.count(len(b))
	if j.out != nil && j.err == nil {
		_, j.err = j.out.Write(b)
	}
}

func (j *csvJSON) writeString(s string) {
	j.count(len(s))
	if j.out != nil && j.err == nil {
		_, j.err = j.out.WriteString(s)
	}
}

// array converts the records left in cr.
func (j *csvJSON) array(cr *CSVReader) error {
	j.writeString("[")
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if j.err != nil {
			return fmt.Errorf("writing output: %w", j.err)
		}
		if j.rows > 0 {
			j.writeString(",")
		}
		j.rows++
		j.writeString("\n  {")
		for i, field := range record {
			if i > 0 {
				j.writeString(",")
			}
			j.writeString("\n    ")
			j.write(j.keys[i])
			j.writeString(": ")
			escapeJSONString(field, j.emit)
		}
		j.writeString("\n  }")
	}
	if j.rows > 0 {
		j.writeString("\n")
	}
	j.writeString("]")
	return nil
}

func (j *csvJSON) limitError(floor int) error {
	keyBytes := 0
	for _, k := range j.keys {
		keyBytes += len(k)
	}
	repeat := fmt.Sprintf("all %d header names as JSON keys, %d bytes of them", len(j.keys), keyBytes)
	if len(j.keys) == 1 {
		repeat = fmt.Sprintf("the header name as a JSON key, %d bytes", keyBytes)
	}
	return limitError("JSON output", floor, j.limit, fmt.Sprintf("%s each repeat %s per row; split the rows across several files, each with the header row",
		count(j.rows, "row"), repeat))
}

// JSONToCSV converts a JSON array of flat objects from r to CSV in w, header
// row first. The input is decoded token by token so columns come out as the
// union of keys in first-seen order, exactly as they appear in the document
// (unmarshalling into maps would randomize the order). Strings are written
// as-is, numbers keep their exact source form ("1.10" stays "1.10"), booleans
// become true/false, and null and missing keys become empty cells. Nested
// objects and arrays are rejected: CSV cells are flat, and any flattening
// scheme would be lossy.
func JSONToCSV(w io.Writer, r io.Reader, sep rune) error {
	data, err := ReadText(r)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("empty input")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("input must be a JSON array of objects, got %s", describeJSONToken(tok))
	}
	var columns []string
	seen := make(map[string]bool)
	var rows []map[string]string
	for dec.More() {
		elem := len(rows) + 1
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		if delim, ok := tok.(json.Delim); !ok || delim != '{' {
			return fmt.Errorf("array element %d must be an object, got %s", elem, describeJSONToken(tok))
		}
		row := make(map[string]string)
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			key := keyTok.(string)
			if _, exists := row[key]; exists {
				return fmt.Errorf("array element %d: duplicate key %q", elem, key)
			}
			valTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			var cell string
			switch v := valTok.(type) {
			case string:
				cell = v
			case json.Number:
				cell = v.String()
			case bool:
				cell = strconv.FormatBool(v)
			case nil:
				cell = ""
			case json.Delim:
				kind := "object"
				if v == '[' {
					kind = "array"
				}
				return fmt.Errorf("array element %d, key %q: nested %ss are not supported; values must be strings, numbers, booleans, or null", elem, key, kind)
			}
			if !seen[key] {
				seen[key] = true
				columns = append(columns, key)
			}
			row[key] = cell
		}
		if _, err := dec.Token(); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		rows = append(rows, row)
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		return fmt.Errorf("unexpected data after the JSON array")
	}
	if len(rows) == 0 {
		return nil
	}
	if len(columns) == 0 {
		return fmt.Errorf("no columns to write: none of the array's objects has any keys")
	}
	cw := csv.NewWriter(w)
	cw.Comma = sep
	if err := writeRecord(cw, w, columns, sep); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}
	record := make([]string, len(columns))
	for _, row := range rows {
		for i, col := range columns {
			record[i] = row[col]
		}
		if err := writeRecord(cw, w, record, sep); err != nil {
			return fmt.Errorf("writing CSV: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("writing CSV: %w", err)
	}
	return nil
}

// writeRecord writes record through cw, quoting the first field by hand where
// encoding/csv would leave it bare and a reader would then lose data: a record
// of one empty field is a blank line, which CSV readers skip (Python's
// csv.writer writes "" for the same reason), and a first field starting with
// U+FEFF reads as a byte order mark at the start of the output.
func writeRecord(cw *csv.Writer, w io.Writer, record []string, sep rune) error {
	first := record[0]
	if (len(record) > 1 || first != "") && !strings.HasPrefix(first, "\ufeff") {
		return cw.Write(record)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	quoted := `"` + strings.ReplaceAll(first, `"`, `""`) + `"`
	if len(record) == 1 {
		_, err := io.WriteString(w, quoted+"\n")
		return err
	}
	if _, err := io.WriteString(w, quoted+string(sep)); err != nil {
		return err
	}
	return cw.Write(record[1:])
}

func describeJSONToken(tok json.Token) string {
	switch v := tok.(type) {
	case json.Delim:
		if v == '{' {
			return "an object"
		}
		return "an array"
	case string:
		return "a string"
	case json.Number:
		return "a number"
	case bool:
		return "a boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", tok)
	}
}
