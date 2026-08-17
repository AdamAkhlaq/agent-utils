package format

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
