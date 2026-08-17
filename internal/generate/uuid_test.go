package generate

import (
	"regexp"
	"testing"
)

// canonical form with version 4 pinned and the variant nibble limited to
// 8, 9, a, or b (the RFC 4122 "10xx" variant bits).
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUID(t *testing.T) {
	t.Run("matches v4 canonical form", func(t *testing.T) {
		id, err := UUID()
		if err != nil {
			t.Fatalf("UUID() error = %v", err)
		}
		if !uuidPattern.MatchString(id) {
			t.Errorf("UUID() = %q, want it to match %s", id, uuidPattern)
		}
	})

	t.Run("no duplicates across many calls", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 1000 {
			id, err := UUID()
			if err != nil {
				t.Fatalf("UUID() error = %v", err)
			}
			if seen[id] {
				t.Fatalf("UUID() returned duplicate %q", id)
			}
			seen[id] = true
		}
	})
}
