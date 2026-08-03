package encode

import (
	"encoding/base64"
	"fmt"
	"io"
)

// Base64 streams r through standard base64 encoding into w.
func Base64(w io.Writer, r io.Reader) error {
	enc := base64.NewEncoder(base64.StdEncoding, w)
	if _, err := io.Copy(enc, r); err != nil {
		return fmt.Errorf("encoding base64: %w", err)
	}
	// Close flushes the final partial block; without it output is truncated.
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encoding base64: %w", err)
	}
	return nil
}

// Base64Decode streams base64 input from r into w as decoded bytes.
// Newlines in the input are ignored, so piped shell output decodes cleanly.
func Base64Decode(w io.Writer, r io.Reader) error {
	dec := base64.NewDecoder(base64.StdEncoding, r)
	if _, err := io.Copy(w, dec); err != nil {
		return fmt.Errorf("decoding base64: %w", err)
	}
	return nil
}
