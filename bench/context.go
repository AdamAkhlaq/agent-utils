package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// contextTokenTolerance is how far apart, in estimated tokens, the two
// context paragraphs may be. They are the only text that differs between
// arms, so a longer treatment prompt would be a confound.
const contextTokenTolerance = 5

var tokenPiecePattern = regexp.MustCompile(`[A-Za-z]+|[0-9]|[^\sA-Za-z0-9]`)

// estimateTokens approximates a BPE token count without a tokenizer: every
// run of letters, every digit, and every punctuation character is one piece,
// and a run of letters longer than six counts one more per extra six. It is
// not the model's tokenizer, so it is used only to compare two texts
// measured the same way, never to report an absolute count.
func estimateTokens(text string) int {
	n := 0
	for _, piece := range tokenPiecePattern.FindAllString(text, -1) {
		n++
		if len(piece) > 6 && isLetters(piece) {
			n += (len(piece) - 1) / 6
		}
	}
	return n
}

func isLetters(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return r < 'A' || (r > 'Z' && r < 'a') || r > 'z'
	}) < 0
}

// contextLengthProblem reports when two context files are further apart than
// the tolerance, by both the piece estimate and the plainer bytes/4 rule,
// so a mismatch one estimator misses is caught by the other.
func contextLengthProblem(toolText, neutralText string) string {
	type estimator struct {
		name string
		fn   func(string) int
	}
	for _, e := range []estimator{
		{"pieces", estimateTokens},
		{"bytes/4", func(s string) int { return (len(s) + 2) / 4 }},
	} {
		a, b := e.fn(toolText), e.fn(neutralText)
		if d := a - b; d > contextTokenTolerance || d < -contextTokenTolerance {
			return fmt.Sprintf("context/tool.md and context/neutral.md differ by %d estimated tokens (%s: %d vs %d); the limit is %d",
				abs(d), e.name, a, b, contextTokenTolerance)
		}
	}
	return ""
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// agentSnippet extracts the README's agent snippet, the block users are told
// to add to CLAUDE.md, so context/tool.md can be checked against it verbatim.
func agentSnippet(readme string) (string, error) {
	const open = "```markdown\n## agent-utils\n"
	start := strings.Index(readme, open)
	if start < 0 {
		return "", errors.New("README.md: the agent snippet (a ```markdown block starting with \"## agent-utils\") was not found")
	}
	body := readme[start+len("```markdown\n"):]
	end := strings.Index(body, "\n```")
	if end < 0 {
		return "", errors.New("README.md: the agent snippet's code block is not closed")
	}
	return body[:end+1], nil
}
