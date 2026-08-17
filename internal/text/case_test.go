package text

import (
	"strings"
	"testing"
)

func TestCase(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		target string
		want   string
	}{
		{name: "camel to snake", input: "camelCase", target: "snake", want: "camel_case"},
		{name: "pascal to kebab", input: "PascalCase", target: "kebab", want: "pascal-case"},
		{name: "snake to camel", input: "snake_case", target: "camel", want: "snakeCase"},
		{name: "kebab to pascal", input: "kebab-case", target: "pascal", want: "KebabCase"},
		{name: "acronym to snake", input: "HTTPServer", target: "snake", want: "http_server"},
		{name: "acronym normalizes in camel", input: "HTTPServer", target: "camel", want: "httpServer"},
		{name: "trailing acronym", input: "userID", target: "snake", want: "user_id"},
		{name: "spaces to camel", input: "hello world", target: "camel", want: "helloWorld"},
		{name: "digit word", input: "version 2 update", target: "snake", want: "version_2_update"},
		{name: "already snake", input: "already_snake", target: "snake", want: "already_snake"},
		{name: "mixed acronyms", input: "XMLHttpRequest", target: "snake", want: "xml_http_request"},
		{name: "inner acronym", input: "JSONToYAML", target: "kebab", want: "json-to-yaml"},
		{name: "to screaming", input: "camelCase", target: "screaming", want: "CAMEL_CASE"},
		{name: "unicode word title-cased", input: "café au lait", target: "pascal", want: "CaféAuLait"},
		{name: "punctuation as delimiter", input: "hello, world!", target: "kebab", want: "hello-world"},
		{name: "empty input", input: "", target: "snake", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Case(tt.input, tt.target)
			if err != nil {
				t.Fatalf("Case(%q, %q) error = %v", tt.input, tt.target, err)
			}
			if got != tt.want {
				t.Errorf("Case(%q, %q) = %q, want %q", tt.input, tt.target, got, tt.want)
			}
		})
	}

	t.Run("unknown target", func(t *testing.T) {
		_, err := Case("anything", "shouting")
		if err == nil {
			t.Fatal("Case() expected an error for an unknown target, got nil")
		}
		for _, target := range caseTargets {
			if !strings.Contains(err.Error(), target) {
				t.Errorf("Case() error %q does not list valid target %q", err, target)
			}
		}
	})
}
