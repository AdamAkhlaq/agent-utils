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

// JSONToYAML converts the JSON document in r to YAML in w, the inverse of
// YAMLToJSON. Key order and number representation survive, and strings that
// would read as YAML booleans or numbers ("yes", "123") come out quoted, so
// feeding the output back through YAMLToJSON reproduces the original document.
func JSONToYAML(w io.Writer, r io.Reader) error {
	data, err := readInput(r)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("empty input")
	}
	// Validate before converting: yaml.JSONToYAML's errors carry no input
	// position, while encoding/json's do. Mirrors JSONCompact, including its
	// json.Indent re-run to recover the offset json.Compact reports as 0.
	var scratch bytes.Buffer
	if err := json.Compact(&scratch, data); err != nil {
		var indented bytes.Buffer
		if ierr := json.Indent(&indented, data, "", ""); ierr != nil {
			err = ierr
		}
		return syntaxError(data, err)
	}
	out, err := yaml.JSONToYAML(data)
	if err != nil {
		return fmt.Errorf("converting to YAML: %w", err)
	}
	if _, err := w.Write(bytes.TrimSuffix(out, []byte("\n"))); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	return nil
}
