package format

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// ErrInvalidUTF8 is wrapped by every error that reports input that is not
// UTF-8 text, which sets bad data apart from a failure to read it.
var ErrInvalidUTF8 = errors.New("invalid UTF-8")

// utf8BOM is the byte order mark in UTF-8. Windows tools (Excel's "CSV
// UTF-8", Notepad, PowerShell 5's -Encoding utf8) write it at the start of
// text files as an encoding signature. It is not content in any format this
// package reads (YAML 1.2 and XML 1.0 say so outright, and RFC 8259 lets a
// JSON parser skip it), so one at the very start is dropped instead of
// becoming part of the first key or failing the parse. One anywhere else is
// left alone.
var utf8BOM = []byte("\xef\xbb\xbf")

// otherBOMs are the byte order marks of the encodings a text file is most
// often in when it is not UTF-8 (PowerShell 5 writes UTF-16LE by default).
// None of them is valid UTF-8, so naming the encoding is what turns "invalid
// UTF-8 at line 1, column 1" into an error that says what to do. The UTF-32LE
// mark starts with the UTF-16LE one, so it is checked first.
var otherBOMs = []struct {
	mark     []byte
	encoding string
}{
	{[]byte("\xff\xfe\x00\x00"), "UTF-32LE"},
	{[]byte("\x00\x00\xfe\xff"), "UTF-32BE"},
	{[]byte("\xff\xfe"), "UTF-16LE"},
	{[]byte("\xfe\xff"), "UTF-16BE"},
}

// ReadText reads all of r as UTF-8 text, without a leading byte order mark.
// Input that is not valid UTF-8 is an error naming the line and column of
// the first bad byte (or the encoding its byte order mark announces), rather
// than being decoded into U+FFFD or passed through as is.
func ReadText(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return checkText(data)
}

// checkText returns data without a leading UTF-8 byte order mark, or an
// error if data is not UTF-8.
func checkText(data []byte) ([]byte, error) {
	for _, b := range otherBOMs {
		if bytes.HasPrefix(data, b.mark) {
			return nil, fmt.Errorf("%w: this is %s text (it starts with the byte order mark % X); convert it to UTF-8 first, for example with iconv -f %s -t UTF-8",
				ErrInvalidUTF8, b.encoding, b.mark, b.encoding)
		}
	}
	data = bytes.TrimPrefix(data, utf8BOM)
	if err := checkUTF8(data); err != nil {
		return nil, err
	}
	return data, nil
}

// checkUTF8 reports the position of the first invalid UTF-8 sequence in data.
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
	return fmt.Errorf("%w at line %d, column %d (0x%02x)", ErrInvalidUTF8, line, col, data[offset])
}
