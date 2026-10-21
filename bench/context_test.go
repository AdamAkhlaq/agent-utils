package main

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"hello, world.", 4},
		{"`agent-utils`", 5},
		{"2024-03-09", 10},
		{"transformations", 3},
		{"internationalization", 4},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := estimateTokens(tt.text); got != tt.want {
				t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestContextLengthProblem(t *testing.T) {
	base := strings.Repeat("word ", 100)
	tests := []struct {
		name          string
		tool, neutral string
		want          string
	}{
		{"identical", base, base, ""},
		{"within tolerance", base, base + "one two three", ""},
		{"too many pieces", base, base + "one two three four five six", "differ by 6 estimated tokens (pieces: 100 vs 106)"},
		{"too many bytes", base, base + strings.Repeat("x", 30), "differ by 8 estimated tokens (bytes/4: 125 vs 133)"},
		{"shorter neutral", base + "one two three four five six", base, "differ by 6 estimated tokens (pieces: 106 vs 100)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contextLengthProblem(tt.tool, tt.neutral)
			if tt.want == "" && got != "" {
				t.Fatalf("unexpected problem: %s", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Fatalf("problem = %q, want one containing %q", got, tt.want)
			}
		})
	}
}

func TestAgentSnippet(t *testing.T) {
	tests := []struct {
		name    string
		readme  string
		want    string
		wantErr string
	}{
		{"found", "intro\n\n```markdown\n## agent-utils\n\nline one\nline two\n```\n\nafter\n", "## agent-utils\n\nline one\nline two\n", ""},
		{"another markdown block first", "```markdown\n## other\n```\n```markdown\n## agent-utils\nx\n```\n", "## agent-utils\nx\n", ""},
		{"missing", "no snippet here\n", "", "was not found"},
		{"unclosed", "```markdown\n## agent-utils\nx\n", "", "not closed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := agentSnippet(tt.readme)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("snippet = %q, want %q", got, tt.want)
			}
		})
	}
}
