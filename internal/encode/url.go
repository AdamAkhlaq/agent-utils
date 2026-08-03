package encode

import (
	"fmt"
	"io"
	"net/url"
	"strings"
)

// URL percent-encodes input from r into w with query-component semantics
// (space becomes +). Encoding needs the whole input, so this buffers.
func URL(w io.Writer, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("url-encoding: %w", err)
	}
	if _, err := io.WriteString(w, url.QueryEscape(string(data))); err != nil {
		return fmt.Errorf("url-encoding: %w", err)
	}
	return nil
}

// URLDecode percent-decodes input from r into w. Surrounding whitespace is
// ignored, so piped shell output decodes cleanly.
func URLDecode(w io.Writer, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("url-decoding: %w", err)
	}
	decoded, err := url.QueryUnescape(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("url-decoding: %w", err)
	}
	if _, err := io.WriteString(w, decoded); err != nil {
		return fmt.Errorf("url-decoding: %w", err)
	}
	return nil
}
