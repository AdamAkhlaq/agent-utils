package generate

import (
	"strings"
	"testing"
)

func TestLoremWords(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{name: "first word", n: 1, want: "Lorem"},
		{name: "first five", n: 5, want: "Lorem ipsum dolor sit amet,"},
		{name: "zero", n: 0, want: ""},
		{name: "negative", n: -3, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LoremWords(tt.n); got != tt.want {
				t.Errorf("LoremWords(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}

	t.Run("cycles beyond the passage", func(t *testing.T) {
		passageLen := len(strings.Fields(loremPassage))
		got := strings.Fields(LoremWords(passageLen + 2))
		if len(got) != passageLen+2 {
			t.Fatalf("LoremWords() returned %d words, want %d", len(got), passageLen+2)
		}
		if got[passageLen] != "Lorem" || got[passageLen+1] != "ipsum" {
			t.Errorf("cycle restarts with %q %q, want %q %q", got[passageLen], got[passageLen+1], "Lorem", "ipsum")
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		if LoremWords(30) != LoremWords(30) {
			t.Error("LoremWords() is not deterministic")
		}
	})
}

func TestLoremParagraphs(t *testing.T) {
	t.Run("one paragraph is the passage", func(t *testing.T) {
		got := LoremParagraphs(1)
		if got != loremPassage {
			t.Errorf("LoremParagraphs(1) = %q, want the canonical passage", got)
		}
		if !strings.HasPrefix(got, "Lorem ipsum dolor sit amet") {
			t.Errorf("LoremParagraphs(1) starts with %q", got[:30])
		}
	})

	t.Run("three paragraphs blank-line separated", func(t *testing.T) {
		got := LoremParagraphs(3)
		if n := strings.Count(got, "\n\n"); n != 2 {
			t.Errorf("LoremParagraphs(3) has %d blank-line separators, want 2", n)
		}
		for i, p := range strings.Split(got, "\n\n") {
			if p != loremPassage {
				t.Errorf("paragraph %d differs from the canonical passage", i+1)
			}
		}
	})

	t.Run("zero is empty", func(t *testing.T) {
		if got := LoremParagraphs(0); got != "" {
			t.Errorf("LoremParagraphs(0) = %q, want empty", got)
		}
	})
}
