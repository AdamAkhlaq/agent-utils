package text

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/adamakhlaq/agent-utils/v2/internal/format"
)

// MarkdownTable renders a JSON array from r as a GitHub-flavored Markdown
// pipe table in w. An array of objects becomes one row per object, with the
// columns being the union of keys in first-seen order: the first object's key
// order as written in the source, then keys that only appear in later objects
// appended in first-encountered order (the input is decoded token by token
// because unmarshalling into maps would randomize the order). Missing keys
// render as empty cells. An array of arrays is also accepted: the first inner
// array is the header row. Strings render as-is, numbers keep their exact
// source form ("1.10" stays "1.10"), booleans become true/false, and null and
// missing keys become empty cells. Nested objects and arrays are rejected:
// table cells are flat.
func MarkdownTable(w io.Writer, r io.Reader) error {
	in := &countingReader{r: r}
	data, err := format.ReadText(in)
	if err != nil {
		return err
	}
	t, err := parseJSONTable(data, in.n)
	if err != nil {
		return err
	}
	return t.render(w)
}

// parseJSONTable parses data, which came from inputSize bytes of input.
func parseJSONTable(data []byte, inputSize int) (*table, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty input")
		}
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return nil, fmt.Errorf("input must be a JSON array of objects or a JSON array of arrays, got %s", describeJSONToken(tok))
	}
	if !dec.More() {
		return nil, errors.New("empty JSON array: a table with no rows has no meaning")
	}
	var (
		mode     json.Delim
		t        = newTable(inputSize)
		colIndex = make(map[string]int)
		setIn    []int // per column, the element that last set it
		wideErr  error
	)
	for elem := 1; dec.More(); elem++ {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		delim, ok := tok.(json.Delim)
		if !ok || (delim != '{' && delim != '[') {
			return nil, fmt.Errorf("array element %d must be an object or an array, got %s", elem, describeJSONToken(tok))
		}
		if elem == 1 {
			mode = delim
		} else if delim != mode {
			return nil, fmt.Errorf("array element %d is %s but element 1 is %s; elements must all have the same shape", elem, describeJSONToken(tok), describeJSONToken(mode))
		}
		if delim == '{' {
			var row []cell
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, fmt.Errorf("invalid JSON: %w", err)
				}
				key := keyTok.(string)
				col, exists := colIndex[key]
				if exists && setIn[col] == elem {
					return nil, fmt.Errorf("array element %d: duplicate key %q", elem, key)
				}
				valTok, err := dec.Token()
				if err != nil {
					return nil, fmt.Errorf("invalid JSON: %w", err)
				}
				value, ok := scalarCell(valTok)
				if !ok {
					return nil, fmt.Errorf("array element %d, key %q: nested %ss are not supported; values must be strings, numbers, booleans, or null", elem, key, describeContainer(valTok))
				}
				if !exists {
					col = t.addColumn(key)
					colIndex[key] = col
					setIn = append(setIn, 0)
				}
				setIn[col] = elem
				row = t.addCell(row, col, value)
			}
			if _, err := dec.Token(); err != nil {
				return nil, fmt.Errorf("invalid JSON: %w", err)
			}
			if err := t.addRow(row); err != nil {
				return nil, err
			}
			continue
		}
		var row []cell
		cells := 0
		for ; dec.More(); cells++ {
			valTok, err := dec.Token()
			if err != nil {
				return nil, fmt.Errorf("invalid JSON: %w", err)
			}
			value, ok := scalarCell(valTok)
			if !ok {
				return nil, fmt.Errorf("array element %d, cell %d: nested %ss are not supported; values must be strings, numbers, booleans, or null", elem, cells+1, describeContainer(valTok))
			}
			switch {
			case elem == 1:
				t.addColumn(value)
			case cells < len(t.columns):
				row = t.addCell(row, cells, value)
			}
		}
		if _, err := dec.Token(); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		if elem == 1 {
			if err := t.check(); err != nil {
				return nil, err
			}
			continue
		}
		if cells > len(t.columns) && wideErr == nil {
			wideErr = fmt.Errorf("array element %d has %d cells but the header row has only %d", elem, cells, len(t.columns))
		}
		if err := t.addRow(row); err != nil {
			return nil, err
		}
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return nil, errors.New("unexpected data after the JSON array")
	}
	if mode == '{' {
		if len(t.columns) == 0 {
			return nil, errors.New("no columns to render: none of the array's objects has any keys")
		}
		return t, nil
	}
	if len(t.columns) == 0 {
		return nil, errors.New("header row (the first inner array) is empty")
	}
	if wideErr != nil {
		return nil, wideErr
	}
	return t, nil
}

// MarkdownTableCSV renders CSV from r (first record is the header row) as a
// GitHub-flavored Markdown pipe table in w, read by the same reader as
// csv2json so the two never disagree about what a file holds.
func MarkdownTableCSV(w io.Writer, r io.Reader) error {
	t, err := parseCSVTable(r)
	if err != nil {
		return err
	}
	return t.render(w)
}

func parseCSVTable(r io.Reader) (*table, error) {
	in := &countingReader{r: r}
	// The reader reads all of its input up front, so in.n is the input size.
	cr, err := format.NewCSVReader(in, ',')
	if err != nil {
		return nil, err
	}
	header, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty input")
		}
		return nil, err
	}
	t := newTable(in.n)
	for _, name := range header {
		t.addColumn(name)
	}
	if err := t.check(); err != nil {
		return nil, err
	}
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return t, nil
		}
		if err != nil {
			return nil, err
		}
		row := make([]cell, 0, len(record))
		for col, value := range record {
			row = t.addCell(row, col, value)
		}
		if err := t.addRow(row); err != nil {
			return nil, err
		}
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// maxPaddedWidth is the widest column a table pads. Padding only aligns
// the pipes in the raw text (a renderer draws the same table without it), and
// padding every row to one huge cell would multiply the output by the row
// count, so a wider column is written unpadded.
const maxPaddedWidth = 80

var spaces = strings.Repeat(" ", maxPaddedWidth)

// A rendered table may be up to 64 MiB, or 64 times its input if that is
// more. A table past that even without padding is nearly all empty cells: a
// row of them costs 3 bytes of JSON ({} or []) but 3 bytes per column.
const (
	maxOutputFloor = 64 << 20
	maxOutputRatio = 64
)

// table holds escaped cells as they are parsed. Rows are sparse (a cell a row
// lacks renders empty), so a short row costs only the cells it has, and each
// column keeps the totals that give the rendered size before any of it is
// written.
type table struct {
	header    []string
	columns   []column
	rows      [][]cell
	bytes     int64 // over every cell, the header included
	inputSize int
	limit     int64
}

type column struct {
	width        int // widest cell in runes; the GFM separator needs at least 3
	bytes, runes int // over the header and every cell
}

type cell struct {
	col  int
	text string
}

func newTable(inputSize int) *table {
	return &table{inputSize: inputSize, limit: max(maxOutputFloor, maxOutputRatio*int64(inputSize))}
}

func (t *table) addColumn(name string) int {
	text := escapeCell(name)
	t.header = append(t.header, text)
	t.columns = append(t.columns, column{width: 3})
	t.count(len(t.columns)-1, text)
	return len(t.columns) - 1
}

func (t *table) addCell(row []cell, col int, value string) []cell {
	text := escapeCell(value)
	t.count(col, text)
	return append(row, cell{col: col, text: text})
}

func (t *table) count(col int, text string) {
	c := &t.columns[col]
	n := utf8.RuneCountInString(text)
	c.width = max(c.width, n)
	c.bytes += len(text)
	c.runes += n
	t.bytes += int64(len(text))
}

func (t *table) addRow(row []cell) error {
	t.rows = append(t.rows, row)
	return t.check()
}

// check fails once the table is over the limit even without padding. That
// size only grows as cells, rows, and columns are added, so a table that will
// be rejected stops being parsed as soon as it is.
func (t *table) check() error {
	if t.size(false, t.limit) <= t.limit {
		return nil
	}
	return fmt.Errorf("the table would be more than %d bytes even without padding (%d columns by %d rows so far, from %d bytes of input; the limit is 64 MiB, or 64 times the input if that is more): nearly all of it would be empty cells, from rows much shorter than the header or objects that do not share keys, so make the rows share columns or split the input",
		t.limit, len(t.columns), len(t.rows), t.inputSize)
}

// padded reports whether column c is padded to align its pipes when the
// table is padded at all.
func (c column) padded() bool { return c.width <= maxPaddedWidth }

// size returns the rendered length in bytes, with or without padding. With
// padding it stops summing once it is past stop, so that it cannot overflow.
func (t *table) size(pad bool, stop int64) int64 {
	lines := int64(len(t.rows) + 2)
	total := 2 * lines // the leading pipe and the line break
	if !pad {
		// Each cell is " " + text + " |", and each separator " --- |".
		return total + t.bytes + int64(len(t.columns))*(3*lines+3)
	}
	for _, c := range t.columns {
		if c.padded() {
			// Each cell is " " + text + padding to width + " |", in bytes
			// text plus width minus its runes plus 3; the separator row is
			// width dashes plus 3.
			total += int64(c.bytes-c.runes) + lines*int64(c.width+3)
		} else {
			total += int64(c.bytes) + 3*lines + 3
		}
		if total > stop {
			return total
		}
	}
	return total
}

// render writes the table as a GFM pipe table, padding every cell with spaces
// to the widest cell in its column so the pipes align vertically in
// monospace, unless that cell is wider than maxPaddedWidth. Padding is
// cosmetic, so a table that padding would take past the limit is written
// without any. Widths are counted in runes; East Asian wide characters occupy
// two terminal columns and may still misalign visually.
func (t *table) render(w io.Writer) error {
	if err := t.check(); err != nil {
		return err
	}
	return t.write(w, t.size(true, t.limit) <= t.limit)
}

func (t *table) write(w io.Writer, pad bool) error {
	bw := bufio.NewWriter(w)
	writeRow := func(cells []string) {
		bw.WriteByte('|')
		for i, text := range cells {
			bw.WriteByte(' ')
			bw.WriteString(text)
			if c := t.columns[i]; pad && c.padded() {
				bw.WriteString(spaces[:c.width-utf8.RuneCountInString(text)])
			}
			bw.WriteString(" |")
		}
		bw.WriteByte('\n')
	}
	writeRow(t.header)
	bw.WriteByte('|')
	for _, c := range t.columns {
		bw.WriteByte(' ')
		if pad && c.padded() {
			bw.WriteString(strings.Repeat("-", c.width))
		} else {
			bw.WriteString("---")
		}
		bw.WriteString(" |")
	}
	bw.WriteByte('\n')
	cells := make([]string, len(t.columns))
	for _, row := range t.rows {
		clear(cells)
		for _, c := range row {
			cells[c.col] = c.text
		}
		writeRow(cells)
	}
	// bufio.Writer keeps the first write error and returns it from Flush.
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// escapeCell makes a value safe inside a Markdown table cell: pipes would end
// the cell, so they become \|, and newlines would end the row, so they become
// <br> (the GFM way to break a line inside a cell).
func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return strings.ReplaceAll(s, "|", `\|`)
}

func scalarCell(tok json.Token) (string, bool) {
	switch v := tok.(type) {
	case string:
		return v, true
	case json.Number:
		return v.String(), true
	case bool:
		return strconv.FormatBool(v), true
	case nil:
		return "", true
	default:
		return "", false
	}
}

func describeContainer(tok json.Token) string {
	if tok == json.Delim('[') {
		return "array"
	}
	return "object"
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
