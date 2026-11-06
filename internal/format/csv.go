package format

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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
// scheme would be lossy. Every row has a cell for every column, so output is
// limited as IndentJSON limits it, checked before anything is written.
func JSONToCSV(w io.Writer, r io.Reader, sep rune) error {
	return jsonToCSV(w, r, sep, outputFloor)
}

// jsonToCSV is JSONToCSV with the output limit's floor as a parameter, so
// tests can reach its edge without writing 64 MiB.
func jsonToCSV(w io.Writer, r io.Reader, sep rune, floor int) error {
	if err := checkCSVSeparator(sep); err != nil {
		return err
	}
	data, err := ReadText(r)
	if err != nil {
		return err
	}
	table, err := decodeJSONTable(data)
	if err != nil {
		return err
	}
	if len(table.ends) == 0 {
		return nil
	}
	if len(table.columns) == 0 {
		return fmt.Errorf("no columns to write: none of the array's objects has any keys")
	}
	out := bufio.NewWriterSize(w, 64<<10)
	cw := &csvWriter{out: out, sep: sep, sepStr: string(sep)}
	limit := outputLimit(floor, len(data))
	if cw.tableSize(table, limit) > limit {
		return limitError("CSV output", floor, limit, fmt.Sprintf("%s each have a cell, even an empty one, in all %s (one per distinct key across the objects); convert objects with different keys separately",
			count(len(table.ends), "row"), count(len(table.columns), "column")))
	}
	cw.table(table)
	if cw.err == nil {
		cw.err = out.Flush()
	}
	if cw.err != nil {
		return fmt.Errorf("writing CSV: %w", cw.err)
	}
	return nil
}

// jsonTable is a JSON array of flat objects: its columns, the distinct keys
// in first-seen order, and every object's cells. The cells are numbered in
// document order and kept in fixed-size chunks, which never move once
// allocated, unlike one slice that copies everything each time it grows;
// ends holds where each object's cells end, so an empty object costs one int.
type jsonTable struct {
	columns []string
	chunks  [][]jsonCell
	cells   int
	ends    []int
}

// jsonCell is one key's value in an object, by its column's index.
type jsonCell struct {
	column int
	value  string
}

const cellChunk = 4096

func (t *jsonTable) add(c jsonCell) {
	if t.cells%cellChunk == 0 {
		t.chunks = append(t.chunks, make([]jsonCell, 0, cellChunk))
	}
	last := len(t.chunks) - 1
	t.chunks[last] = append(t.chunks[last], c)
	t.cells++
}

// row calls fn for each cell of object i.
func (t *jsonTable) row(i int, fn func(jsonCell)) {
	start := 0
	if i > 0 {
		start = t.ends[i-1]
	}
	for k := start; k < t.ends[i]; k++ {
		fn(t.chunks[k/cellChunk][k%cellChunk])
	}
}

// decodeJSONTable decodes a JSON array of flat objects.
func decodeJSONTable(data []byte) (*jsonTable, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("empty input")
		}
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return nil, fmt.Errorf("input must be a JSON array of objects, got %s", describeJSONToken(tok))
	}
	t := &jsonTable{}
	index := make(map[string]int)
	// lastRow holds, for each column, the 1-based number of the last
	// object that had its key, which finds a duplicate key in an object
	// without a map per object.
	var lastRow []int
	for dec.More() {
		elem := len(t.ends) + 1
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		if delim, ok := tok.(json.Delim); !ok || delim != '{' {
			return nil, fmt.Errorf("array element %d must be an object, got %s", elem, describeJSONToken(tok))
		}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, fmt.Errorf("invalid JSON: %w", err)
			}
			key := keyTok.(string)
			col, ok := index[key]
			if !ok {
				col = len(t.columns)
				index[key] = col
				t.columns = append(t.columns, key)
				lastRow = append(lastRow, 0)
			}
			if lastRow[col] == elem {
				return nil, fmt.Errorf("array element %d: duplicate key %q", elem, key)
			}
			lastRow[col] = elem
			valTok, err := dec.Token()
			if err != nil {
				return nil, fmt.Errorf("invalid JSON: %w", err)
			}
			var cell string
			switch v := valTok.(type) {
			case string:
				cell = v
			case json.Number:
				cell = v.String()
			case bool:
				cell = strconv.FormatBool(v)
			case json.Delim:
				kind := "object"
				if v == '[' {
					kind = "array"
				}
				return nil, fmt.Errorf("array element %d, key %q: nested %ss are not supported; values must be strings, numbers, booleans, or null", elem, key, kind)
			}
			t.add(jsonCell{col, cell})
		}
		if _, err := dec.Token(); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		t.ends = append(t.ends, t.cells)
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return nil, fmt.Errorf("unexpected data after the JSON array")
	}
	if err := CheckJSONSurrogates(data); err != nil {
		return nil, err
	}
	return t, nil
}

// csvWriter writes CSV records ending in LF. It quotes a field exactly where
// encoding/csv's Writer does, and also where encoding/csv would leave it bare
// and a reader would then lose data: a record of one empty field is a blank
// line, which CSV readers skip (Python's csv.writer writes "" for the same
// reason), and a first field starting with U+FEFF reads as a byte order mark
// at the start of the output.
type csvWriter struct {
	out    *bufio.Writer
	err    error // the first failed write
	sep    rune
	sepStr string
}

func (w *csvWriter) writeString(s string) {
	if w.err == nil {
		_, w.err = w.out.WriteString(s)
	}
}

// table writes the header row and one record per object.
func (w *csvWriter) table(t *jsonTable) {
	w.record(t.columns)
	record := make([]string, len(t.columns))
	for i := range t.ends {
		t.row(i, func(c jsonCell) { record[c.column] = c.value })
		w.record(record)
		t.row(i, func(c jsonCell) { record[c.column] = "" })
	}
}

func (w *csvWriter) record(fields []string) {
	for i, field := range fields {
		if i > 0 {
			w.writeString(w.sepStr)
		}
		if !w.quoted(field, i, len(fields)) {
			w.writeString(field)
			continue
		}
		w.writeString(`"`)
		for {
			quote := strings.IndexByte(field, '"')
			if quote < 0 {
				break
			}
			w.writeString(field[:quote+1])
			w.writeString(`"`)
			field = field[quote+1:]
		}
		w.writeString(field)
		w.writeString(`"`)
	}
	w.writeString("\n")
}

// tableSize returns the size of the table as table writes it, computed from
// the cells each object has rather than by walking every column of every
// row, so it costs time in proportion to the input. It stops counting once
// the size passes limit.
func (w *csvWriter) tableSize(t *jsonTable, limit int64) int64 {
	size := w.recordSize(t.columns)
	n := len(t.columns)
	perRow := int64(n-1)*int64(len(w.sepStr)) + 1
	for i := 0; i < len(t.ends) && size <= limit; i++ {
		size += perRow
		hasFirst := false
		t.row(i, func(c jsonCell) {
			size += w.fieldSize(c.value, c.column, n)
			hasFirst = hasFirst || c.column == 0
		})
		if !hasFirst {
			size += w.fieldSize("", 0, n)
		}
	}
	return size
}

// recordSize returns the size of fields as record writes them.
func (w *csvWriter) recordSize(fields []string) int64 {
	size := int64(len(fields)-1)*int64(len(w.sepStr)) + 1
	for i, field := range fields {
		size += w.fieldSize(field, i, len(fields))
	}
	return size
}

// fieldSize returns the size of field as record writes it, as field i of n.
func (w *csvWriter) fieldSize(field string, i, n int) int64 {
	if !w.quoted(field, i, n) {
		return int64(len(field))
	}
	return int64(len(field) + 2 + strings.Count(field, `"`))
}

// quoted reports whether record quotes field, as field i of n.
func (w *csvWriter) quoted(field string, i, n int) bool {
	if i == 0 && (n == 1 && field == "" || strings.HasPrefix(field, "\ufeff")) {
		return true
	}
	if field == "" {
		return false
	}
	if field == `\.` || strings.ContainsRune(field, w.sep) || strings.ContainsAny(field, "\"\r\n") {
		return true
	}
	r, _ := utf8.DecodeRuneInString(field)
	return unicode.IsSpace(r)
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
