package encode

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/adamakhlaq/agent-utils/v2/internal/format"
)

// JWTDecode renders the JWT in r as one pretty-printed JSON document with
// "header" and "payload" fields. It only decodes: the signature is not
// verified, so the claims must not be trusted on this basis alone. Segment
// JSON is re-indented, never unmarshalled, so claim order and number
// representation survive untouched. A leading "Bearer " (as pasted from an
// Authorization header) is ignored.
func JWTDecode(w io.Writer, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	token := strings.TrimSpace(string(data))
	token = strings.TrimPrefix(token, "Bearer ")
	parts := strings.Split(token, ".")
	switch len(parts) {
	case 3:
	case 5:
		return fmt.Errorf("token is a JWE (encrypted), which cannot be decoded without its key")
	default:
		return fmt.Errorf("not a JWT: expected 3 dot-separated parts, got %d", len(parts))
	}

	header, err := decodeJWTPart(parts[0], "header")
	if err != nil {
		return err
	}
	payload, err := decodeJWTPart(parts[1], "payload")
	if err != nil {
		return err
	}
	// Header and payload are valid JSON, so the wrapper is too, and indenting
	// it as one document sizes the whole output before any of it is written.
	doc := slices.Concat([]byte(`{"header":`), header, []byte(`,"payload":`), payload, []byte(`}`))
	return format.IndentJSON(w, doc, "  ", len(data))
}

// decodeJWTPart decodes one base64url token segment and checks that it is a
// JSON document, without indenting it.
func decodeJWTPart(part, name string) ([]byte, error) {
	// RFC 7515 forbids padding, but some emitters add it anyway; stripping it
	// is harmless and decodes their tokens too.
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(part, "="))
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", name, err)
	}
	// Indenting copies the bytes of strings through untouched, and JSON text
	// must be UTF-8 (RFC 8259), so invalid UTF-8 would leave the output
	// not JSON.
	if err := validUTF8(string(raw)); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", name, err)
	}
	if !json.Valid(raw) {
		var scratch bytes.Buffer
		err := json.Compact(&scratch, raw)
		return nil, fmt.Errorf("%s is not valid JSON: %w", name, err)
	}
	return raw, nil
}
