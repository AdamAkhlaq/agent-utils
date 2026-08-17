package text

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// caseTargets lists the casings Case accepts, in the order shown to users.
var caseTargets = []string{"snake", "camel", "pascal", "kebab", "screaming"}

// Case converts s to the target casing: snake ("user_id"), camel ("userId"),
// pascal ("UserId"), kebab ("user-id"), or screaming ("USER_ID"). Words are
// detected by splitting on runs of non-alphanumeric characters and at camel
// boundaries, including acronym runs ("HTTPServer" is HTTP + Server). Every
// word is lowercased before joining, so acronyms normalize ("HTTPServer" in
// camel is "httpServer", not "hTTPServer"): deterministic and standard for
// case converters; preserving acronym capitalization would need a dictionary.
// An empty s converts to "".
func Case(s, target string) (string, error) {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	switch target {
	case "snake":
		return strings.Join(words, "_"), nil
	case "kebab":
		return strings.Join(words, "-"), nil
	case "screaming":
		return strings.ToUpper(strings.Join(words, "_")), nil
	case "camel":
		for i := 1; i < len(words); i++ {
			words[i] = titleWord(words[i])
		}
		return strings.Join(words, ""), nil
	case "pascal":
		for i := range words {
			words[i] = titleWord(words[i])
		}
		return strings.Join(words, ""), nil
	default:
		return "", fmt.Errorf("unknown case target %q (valid: %s)", target, strings.Join(caseTargets, ", "))
	}
}

// splitWords breaks s into words: runs of non-alphanumeric characters are
// delimiters, and alphanumeric runs are further split at camel boundaries.
func splitWords(s string) []string {
	var words []string
	var run []rune
	flush := func() {
		if len(run) > 0 {
			words = append(words, splitCamel(run)...)
			run = run[:0]
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			run = append(run, r)
			continue
		}
		flush()
	}
	flush()
	return words
}

// splitCamel splits one alphanumeric run at camel boundaries: before an upper
// letter that follows a lower letter or digit ("camelCase", "userID"), and
// before the last upper of an all-upper run followed by a lower letter, which
// keeps acronyms whole ("HTTPServer" is HTTP + Server, not HTTPS + erver).
func splitCamel(run []rune) []string {
	var words []string
	start := 0
	for i := 1; i < len(run); i++ {
		afterLowerOrDigit := unicode.IsUpper(run[i]) &&
			(unicode.IsLower(run[i-1]) || unicode.IsDigit(run[i-1]))
		lastUpperBeforeLower := unicode.IsUpper(run[i]) && unicode.IsUpper(run[i-1]) &&
			i+1 < len(run) && unicode.IsLower(run[i+1])
		if afterLowerOrDigit || lastUpperBeforeLower {
			words = append(words, string(run[start:i]))
			start = i
		}
	}
	return append(words, string(run[start:]))
}

// titleWord upper-cases the first rune of an already-lowercase word,
// rune-aware so unicode initials work.
func titleWord(w string) string {
	r, size := utf8.DecodeRuneInString(w)
	return string(unicode.ToUpper(r)) + w[size:]
}
