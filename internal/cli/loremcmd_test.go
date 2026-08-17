package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The stubs echo their argument, so these tests exercise only the CLI
// layer's flag handling and routing.
func TestLoremCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "default is one paragraph", args: nil, want: "paragraphs(1)\n"},
		{name: "paragraph count", args: []string{"-p", "3"}, want: "paragraphs(3)\n"},
		{name: "word count", args: []string{"-w", "40"}, want: "words(40)\n"},
		{name: "words and paragraphs conflict", args: []string{"-w", "10", "-p", "2"}, wantErr: true, wantUsage: true},
		{name: "explicit default paragraph with words conflicts", args: []string{"-p", "1", "-w", "10"}, wantErr: true, wantUsage: true},
		{name: "zero words", args: []string{"-w", "0"}, wantErr: true, wantUsage: true},
		{name: "zero paragraphs", args: []string{"-p", "0"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "unexpected argument", args: []string{"stray"}, wantErr: true, wantUsage: true},
	}
	cmd := LoremCommand(
		func(n int) string { return fmt.Sprintf("words(%d)", n) },
		func(n int) string { return fmt.Sprintf("paragraphs(%d)", n) },
	)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("Run() stdout = %q, want %q", got, tt.want)
			}
		})
	}
}
