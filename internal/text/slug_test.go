package text

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple sentence", input: "Hello, World!", want: "hello-world"},
		{name: "already a slug", input: "already-a-slug", want: "already-a-slug"},
		{name: "surrounding whitespace", input: "  padded  ", want: "padded"},
		{name: "punctuation runs collapse", input: "a --- b  &  c", want: "a-b-c"},
		{name: "underscores and digits", input: "foo_bar 123", want: "foo-bar-123"},
		{name: "unicode letters kept and lowercased", input: "Café au Lait", want: "café-au-lait"},
		{name: "only punctuation", input: "?!...", want: ""},
		{name: "empty input", input: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.input); got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
