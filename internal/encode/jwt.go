package encode

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

	var out bytes.Buffer
	out.WriteString("{\n  \"header\": ")
	if err := writeJWTPart(&out, parts[0], "header"); err != nil {
		return err
	}
	out.WriteString(",\n  \"payload\": ")
	if err := writeJWTPart(&out, parts[1], "payload"); err != nil {
		return err
	}
	out.WriteString("\n}")
	if _, err := w.Write(out.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}

// writeJWTPart decodes one base64url token segment and appends it to out as
// JSON indented to sit inside the wrapper object.
func writeJWTPart(out *bytes.Buffer, part, name string) error {
	// RFC 7515 forbids padding, but some emitters add it anyway; stripping it
	// is harmless and decodes their tokens too.
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(part, "="))
	if err != nil {
		return fmt.Errorf("decoding %s: %w", name, err)
	}
	if err := json.Indent(out, raw, "  ", "  "); err != nil {
		return fmt.Errorf("%s is not valid JSON: %w", name, err)
	}
	return nil
}
