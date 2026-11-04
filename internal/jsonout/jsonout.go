// Package jsonout renders values as the pretty-printed JSON documents that
// commands print.
package jsonout

import (
	"encoding/json"
	"strings"
)

// Marshal returns v as JSON indented by two spaces per level, without a
// trailing newline. Unlike json.MarshalIndent it writes <, >, and & as is
// rather than as <, >, and &: the output is for terminals,
// files, and jq, not for embedding in HTML, and every command writes these
// characters the same way.
func Marshal(v any) (string, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}
