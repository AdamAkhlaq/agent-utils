package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

// YAMLToJSON converts the single YAML document in r to pretty-printed JSON in
// w. The conversion works on the syntax tree, not an unmarshalled map, so key
// order and number representation survive; anchors and aliases are resolved,
// and non-string keys become JSON strings.
func YAMLToJSON(w io.Writer, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("empty input")
	}
	// yaml.YAMLToJSON silently converts only the first document, so multiple
	// documents are rejected up front instead of truncated.
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	if n := len(file.Docs); n > 1 {
		return fmt.Errorf("input has %d YAML documents; expected one", n)
	}
	compact, err := yaml.YAMLToJSON(data)
	if err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimRight(compact, "\n"), "", "  "); err != nil {
		return fmt.Errorf("formatting JSON: %w", err)
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
