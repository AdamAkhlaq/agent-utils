package generate

import "strings"

// the canonical Lorem Ipsum passage, unchanged since typesetters started
// using it; four sentences, 69 words
const loremPassage = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, " +
	"sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
	"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris " +
	"nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in " +
	"reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla " +
	"pariatur. Excepteur sint occaecat cupidatat non proident, sunt in " +
	"culpa qui officia deserunt mollit anim id est laborum."

// LoremWords returns the first n words of the canonical passage, cycling
// through it when n is longer. Output is deterministic: filler text's job is
// layout, and reproducible output composes better in scripts and tests.
func LoremWords(n int) string {
	if n < 1 {
		return ""
	}
	words := strings.Fields(loremPassage)
	out := make([]string, n)
	for i := range out {
		out[i] = words[i%len(words)]
	}
	return strings.Join(out, " ")
}

// LoremParagraphs returns n copies of the canonical passage as blank-line
// separated paragraphs.
func LoremParagraphs(n int) string {
	if n < 1 {
		return ""
	}
	paragraphs := make([]string, n)
	for i := range paragraphs {
		paragraphs[i] = loremPassage
	}
	return strings.Join(paragraphs, "\n\n")
}
