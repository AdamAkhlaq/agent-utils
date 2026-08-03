package encode

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Hex streams r through hex encoding into w.
func Hex(w io.Writer, r io.Reader) error {
	// Unlike base64, hex has no partial-block state, so there is no Close.
	if _, err := io.Copy(hex.NewEncoder(w), r); err != nil {
		return fmt.Errorf("encoding hex: %w", err)
	}
	return nil
}

// HexDecode decodes hex input from r into w as raw bytes. Surrounding
// whitespace is ignored, so piped shell output decodes cleanly.
func HexDecode(w io.Writer, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("decoding hex: %w", err)
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("decoding hex: %w", err)
	}
	if _, err := w.Write(decoded); err != nil {
		return fmt.Errorf("decoding hex: %w", err)
	}
	return nil
}
