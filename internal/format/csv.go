package format

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// CSVToJSON converts CSV from r (first record is the header row) to a
// pretty-printed JSON array of objects in w, one object per data row with the
// headers as keys in column order. Every value stays a JSON string: CSV
// carries no type information, and inferring types corrupts data (a zip code
// "01234" must not become the number 1234). The JSON is assembled by hand
// because marshalling through a map would lose column order.
func CSVToJSON(w io.Writer, r io.Reader, sep rune) error {
	cr := csv.NewReader(r)
	cr.Comma = sep
	headers, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("empty input")
		}
		return fmt.Errorf("reading CSV header: %w", err)
	}
	seen := make(map[string]bool, len(headers))
	keys := make([][]byte, len(headers))
	for i, h := range headers {
		if seen[h] {
			return fmt.Errorf("duplicate header %q", h)
		}
		seen[h] = true
		key, err := json.Marshal(h)
		if err != nil {
			return fmt.Errorf("encoding header %q: %w", h, err)
		}
		keys[i] = key
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	rows := 0
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("reading CSV: %w", err)
		}
		if rows > 0 {
			buf.WriteByte(',')
		}
		rows++
		buf.WriteString("\n  {")
		for i, field := range record {
			value, err := json.Marshal(field)
			if err != nil {
				return fmt.Errorf("encoding value %q: %w", field, err)
			}
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString("\n    ")
			buf.Write(keys[i])
			buf.WriteString(": ")
			buf.Write(value)
		}
		buf.WriteString("\n  }")
	}
	if rows > 0 {
		buf.WriteByte('\n')
	}
	buf.WriteByte(']')
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
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
	dec := json.NewDecoder(r)
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
	if err := cw.Write(columns); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}
	record := make([]string, len(columns))
	for _, row := range rows {
		for i, col := range columns {
			record[i] = row[col]
		}
		if err := cw.Write(record); err != nil {
			return fmt.Errorf("writing CSV: %w", err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("writing CSV: %w", err)
	}
	return nil
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
