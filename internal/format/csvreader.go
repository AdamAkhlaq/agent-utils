package format

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"
)

// CSVReader reads RFC 4180 records. It replaces encoding/csv, which turns
// CRLF inside a quoted field into LF ("carriage returns before newline
// characters are silently removed") and so changes the data it reads.
//
// Every byte between a field's quotes is kept, line breaks included, and a
// doubled quote stands for one. Records end at LF or CRLF; a CR anywhere
// else outside quotes is an error, as RFC 4180 allows CR only inside quotes
// or in CRLF. Blank lines between records are skipped, and every record must
// have as many fields as the first, the header row.
type CSVReader struct {
	data   []byte
	sep    []byte
	pos    int
	fields int
}

// NewCSVReader reads all of r and returns a reader over its records, with
// sep as the field separator.
func NewCSVReader(r io.Reader, sep rune) (*CSVReader, error) {
	if sep == 0 || sep == '"' || sep == '\r' || sep == '\n' || sep == utf8.RuneError || !utf8.ValidRune(sep) {
		return nil, fmt.Errorf("invalid CSV separator %q", sep)
	}
	data, err := readText(r)
	if err != nil {
		return nil, err
	}
	return &CSVReader{data: data, sep: utf8.AppendRune(nil, sep), fields: -1}, nil
}

// Read returns the next record, or io.EOF after the last one.
func (c *CSVReader) Read() ([]string, error) {
	for c.pos < len(c.data) {
		n := lineBreak(c.data[c.pos:])
		if n == 0 {
			break
		}
		c.pos += n
	}
	if c.pos == len(c.data) {
		return nil, io.EOF
	}
	start := c.pos
	var record []string
	for {
		field, err := c.field()
		if err != nil {
			return nil, err
		}
		record = append(record, field)
		// Failing at the first surplus field keeps a huge malformed line
		// from being split into millions of fields first.
		if c.fields >= 0 && len(record) > c.fields {
			line, _ := lineCol(c.data, int64(start+1))
			return nil, fmt.Errorf("invalid CSV at line %d: record has more than the header row's %d fields", line, c.fields)
		}
		if c.pos == len(c.data) {
			break
		}
		if n := lineBreak(c.data[c.pos:]); n > 0 {
			c.pos += n
			break
		}
		c.pos += len(c.sep)
	}
	if c.fields < 0 {
		c.fields = len(record)
	} else if len(record) < c.fields {
		line, _ := lineCol(c.data, int64(start+1))
		return nil, fmt.Errorf("invalid CSV at line %d: record has %s but the header row has %d", line, count(len(record), "field"), c.fields)
	}
	return record, nil
}

func (c *CSVReader) field() (string, error) {
	if c.pos < len(c.data) && c.data[c.pos] == '"' {
		return c.quoted()
	}
	start, first := c.pos, c.sep[0]
	for ; c.pos < len(c.data); c.pos++ {
		switch b := c.data[c.pos]; {
		case b == '"':
			return "", c.errorAt(c.pos, `bare " in an unquoted field; quote the whole field and double each " inside it`)
		case b == '\n' || b == '\r' || (b == first && bytes.HasPrefix(c.data[c.pos:], c.sep)):
			return string(c.data[start:c.pos]), c.fieldEnd()
		}
	}
	return string(c.data[start:]), nil
}

func (c *CSVReader) quoted() (string, error) {
	open := c.pos
	c.pos++
	var value []byte
	for {
		i := bytes.IndexByte(c.data[c.pos:], '"')
		if i < 0 {
			return "", c.errorAt(open, "quoted field is never closed")
		}
		value = append(value, c.data[c.pos:c.pos+i]...)
		c.pos += i + 1
		if c.pos < len(c.data) && c.data[c.pos] == '"' {
			value = append(value, '"')
			c.pos++
			continue
		}
		return string(value), c.fieldEnd()
	}
}

// fieldEnd checks that a field is followed by a separator, a line break, or
// the end of the input.
func (c *CSVReader) fieldEnd() error {
	switch {
	case c.pos == len(c.data), lineBreak(c.data[c.pos:]) > 0, bytes.HasPrefix(c.data[c.pos:], c.sep):
		return nil
	case c.data[c.pos] == '\r':
		return c.errorAt(c.pos, `carriage return without a line feed outside quotes; CSV line breaks are \n or \r\n, so convert CR line endings to LF or quote a field that contains \r`)
	default:
		r, _ := utf8.DecodeRune(c.data[c.pos:])
		return c.errorAt(c.pos, `unexpected %q after a closing quote; remove it, or double each " inside the quoted field`, r)
	}
}

func (c *CSVReader) errorAt(pos int, format string, args ...any) error {
	line, col := lineCol(c.data, int64(pos+1))
	return fmt.Errorf("invalid CSV at line %d, column %d: %s", line, col, fmt.Sprintf(format, args...))
}

func lineBreak(b []byte) int {
	switch {
	case len(b) > 0 && b[0] == '\n':
		return 1
	case len(b) > 1 && b[0] == '\r' && b[1] == '\n':
		return 2
	}
	return 0
}
