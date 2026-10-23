package format

import (
	"bytes"
	"fmt"
	"io"
)

// utf8BOM is the byte order mark in UTF-8. Windows tools (Excel's "CSV
// UTF-8", Notepad, PowerShell 5's -Encoding utf8) write it at the start of
// text files as an encoding signature. It is not content in any format this
// package reads (YAML 1.2 and XML 1.0 say so outright, and RFC 8259 lets a
// JSON parser skip it), so one at the very start is dropped instead of
// becoming part of the first key or failing the parse. One anywhere else is
// left alone.
var utf8BOM = []byte("\xef\xbb\xbf")

func trimBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}

// readText reads all of r, without a leading byte order mark.
func readText(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return trimBOM(data), nil
}
