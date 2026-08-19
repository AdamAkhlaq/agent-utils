package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestURLParseCommand(t *testing.T) {
	parseJSON := func(raw string) (string, error) {
		if raw == "" {
			return "", errors.New("empty input")
		}
		if raw == "bad" {
			return "", errors.New("parsing URL: bad")
		}
		return fmt.Sprintf("json:%s", raw), nil
	}
	cmd := URLParseCommand(parseJSON)

	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "url argument", args: []string{"https://example.com/a"}, want: "json:https://example.com/a\n"},
		{name: "stdin input", args: nil, stdin: "https://example.com/a", want: "json:https://example.com/a\n"},
		{name: "stdin first line only", args: nil, stdin: "https://example.com/a\nsecond line\n", want: "json:https://example.com/a\n"},
		{name: "stdin line is trimmed", args: nil, stdin: "  https://example.com/a \r\n", want: "json:https://example.com/a\n"},
		{name: "empty stdin is a runtime error", args: nil, stdin: "", wantErr: true, wantUsage: false},
		{name: "core error is runtime not usage", args: []string{"bad"}, wantErr: true, wantUsage: false},
		{name: "two url arguments", args: []string{"https://a.com", "https://b.com"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
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
