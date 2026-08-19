package text

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
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
	dec := json.NewDecoder(r)
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty input")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("input must be a JSON array of objects or a JSON array of arrays, got %s", describeJSONToken(tok))
	}
	if !dec.More() {
		return errors.New("empty JSON array: a table with no rows has no meaning")
	}
	var (
		mode     json.Delim
		columns  []string
		colSeen  = make(map[string]bool)
		objRows  []map[string]string
		listRows [][]string
	)
	for elem := 1; dec.More(); elem++ {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		delim, ok := tok.(json.Delim)
		if !ok || (delim != '{' && delim != '[') {
			return fmt.Errorf("array element %d must be an object or an array, got %s", elem, describeJSONToken(tok))
		}
		if elem == 1 {
			mode = delim
		} else if delim != mode {
			return fmt.Errorf("array element %d is %s but element 1 is %s; elements must all have the same shape", elem, describeJSONToken(tok), describeJSONToken(mode))
		}
		if delim == '{' {
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
				cell, ok := scalarCell(valTok)
				if !ok {
					return fmt.Errorf("array element %d, key %q: nested %ss are not supported; values must be strings, numbers, booleans, or null", elem, key, describeContainer(valTok))
				}
				if !colSeen[key] {
					colSeen[key] = true
					columns = append(columns, key)
				}
				row[key] = cell
			}
			if _, err := dec.Token(); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			objRows = append(objRows, row)
			continue
		}
		var row []string
		for dec.More() {
			valTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			cell, ok := scalarCell(valTok)
			if !ok {
				return fmt.Errorf("array element %d, cell %d: nested %ss are not supported; values must be strings, numbers, booleans, or null", elem, len(row)+1, describeContainer(valTok))
			}
			row = append(row, cell)
		}
		if _, err := dec.Token(); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		listRows = append(listRows, row)
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		return errors.New("unexpected data after the JSON array")
	}
	if mode == '{' {
		if len(columns) == 0 {
			return errors.New("no columns to render: none of the array's objects has any keys")
		}
		rows := make([][]string, len(objRows))
		for i, row := range objRows {
			cells := make([]string, len(columns))
			for j, col := range columns {
				cells[j] = row[col]
			}
			rows[i] = cells
		}
		return renderTable(w, columns, rows)
	}
	header := listRows[0]
	if len(header) == 0 {
		return errors.New("header row (the first inner array) is empty")
	}
	for i, row := range listRows[1:] {
		if len(row) > len(header) {
			return fmt.Errorf("array element %d has %d cells but the header row has only %d", i+2, len(row), len(header))
		}
	}
	return renderTable(w, header, listRows[1:])
}

// MarkdownTableCSV renders CSV from r (first record is the header row) as a
// GitHub-flavored Markdown pipe table in w. Quoted fields (embedded commas,
// quotes, newlines) and CRLF line endings are handled per RFC 4180, matching
// csv2json.
func MarkdownTableCSV(w io.Writer, r io.Reader) error {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty input")
		}
		return fmt.Errorf("reading CSV header: %w", err)
	}
	var rows [][]string
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading CSV: %w", err)
		}
		rows = append(rows, record)
	}
	return renderTable(w, header, rows)
}

// renderTable writes header and rows as a GFM pipe table, padding every cell
// with spaces to the widest cell in its column so the pipes align vertically
// in monospace. Widths are counted in runes; East Asian wide characters
// occupy two terminal columns and may still misalign visually. Rows shorter
// than the header are padded with empty cells.
func renderTable(w io.Writer, header []string, rows [][]string) error {
	// The GFM separator row needs at least three dashes per column.
	widths := make([]int, len(header))
	for i := range widths {
		widths[i] = 3
	}
	escape := func(row []string) []string {
		cells := make([]string, len(header))
		for i := range header {
			if i < len(row) {
				cells[i] = escapeCell(row[i])
			}
			if n := utf8.RuneCountInString(cells[i]); n > widths[i] {
				widths[i] = n
			}
		}
		return cells
	}
	escHeader := escape(header)
	escRows := make([][]string, len(rows))
	for i, row := range rows {
		escRows[i] = escape(row)
	}
	var b strings.Builder
	writeRow := func(cells []string) {
		b.WriteByte('|')
		for i, cell := range cells {
			b.WriteByte(' ')
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell)))
			b.WriteString(" |")
		}
		b.WriteByte('\n')
	}
	writeRow(escHeader)
	b.WriteByte('|')
	for _, width := range widths {
		b.WriteByte(' ')
		b.WriteString(strings.Repeat("-", width))
		b.WriteString(" |")
	}
	b.WriteByte('\n')
	for _, row := range escRows {
		writeRow(row)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
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
