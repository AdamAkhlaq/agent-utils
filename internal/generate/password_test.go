package generate

import (
	"strings"
	"testing"
)

func TestPassword(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		symbols bool
		charset string
	}{
		{name: "with symbols", length: 20, symbols: true, charset: passwordAlphanumerics + passwordSymbols},
		{name: "alphanumeric only", length: 20, symbols: false, charset: passwordAlphanumerics},
		{name: "length one", length: 1, symbols: true, charset: passwordAlphanumerics + passwordSymbols},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Password(tt.length, tt.symbols)
			if err != nil {
				t.Fatalf("Password() error = %v", err)
			}
			if len(got) != tt.length {
				t.Errorf("Password() length = %d, want %d", len(got), tt.length)
			}
			for _, c := range got {
				if !strings.ContainsRune(tt.charset, c) {
					t.Errorf("Password() = %q contains %q, not in allowed charset", got, c)
				}
			}
		})
	}

	t.Run("invalid length", func(t *testing.T) {
		for _, length := range []int{0, -1} {
			if _, err := Password(length, true); err == nil {
				t.Errorf("Password(%d) expected an error, got nil", length)
			}
		}
	})

	t.Run("no duplicates across calls", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 100 {
			got, err := Password(20, true)
			if err != nil {
				t.Fatalf("Password() error = %v", err)
			}
			if seen[got] {
				t.Fatalf("Password() returned duplicate %q", got)
			}
			seen[got] = true
		}
	})

	// Every charset character should appear across a large sample; a missing
	// one would mean part of the charset is unreachable (the probability of
	// this failing by chance is astronomically small).
	t.Run("full charset is reachable", func(t *testing.T) {
		counts := make(map[rune]int)
		for range 200 {
			got, err := Password(30, true)
			if err != nil {
				t.Fatalf("Password() error = %v", err)
			}
			for _, c := range got {
				counts[c]++
			}
		}
		for _, c := range passwordAlphanumerics + passwordSymbols {
			if counts[c] == 0 {
				t.Errorf("character %q never appeared in 6000 samples", c)
			}
		}
	})
}
