// Package text holds small plain-text transforms.
package text

import (
	"strings"
	"unicode"
)

// Slugify reduces s to a lowercase slug: runs of anything that isn't a letter
// or digit become single hyphens, with none leading or trailing. Unicode
// letters are kept, not transliterated ("café" stays "café"): URLs handle
// them, and transliterating well needs data tables beyond the stdlib.
func Slugify(s string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			pendingHyphen = true
			continue
		}
		if pendingHyphen && b.Len() > 0 {
			b.WriteByte('-')
		}
		pendingHyphen = false
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
