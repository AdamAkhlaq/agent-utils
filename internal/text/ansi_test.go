package text

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "16-color",
			input: "\x1b[31mred\x1b[0m plain\n",
			want:  "red plain\n",
		},
		{
			name:  "bold plus color",
			input: "\x1b[1;32mok\x1b[0m",
			want:  "ok",
		},
		{
			name:  "256-color",
			input: "\x1b[38;5;208morange\x1b[0m",
			want:  "orange",
		},
		{
			name:  "truecolor",
			input: "\x1b[38;2;255;135;0mtruecolor\x1b[m",
			want:  "truecolor",
		},
		{
			name:  "cursor movement",
			input: "\x1b[1A\x1b[10;20Hmoved",
			want:  "moved",
		},
		{
			name:  "private mode cursor hide and show",
			input: "\x1b[?25lworking\x1b[?25h",
			want:  "working",
		},
		{
			name:  "progress bar erase line keeps carriage returns",
			input: "downloading 10%\r\x1b[2Kdownloading 100%\n",
			want:  "downloading 10%\rdownloading 100%\n",
		},
		{
			name:  "OSC window title BEL-terminated",
			input: "\x1b]0;my title\x07after",
			want:  "after",
		},
		{
			name:  "OSC window title ST-terminated",
			input: "\x1b]2;my title\x1b\\after",
			want:  "after",
		},
		{
			name:  "OSC 8 hyperlink",
			input: "\x1b]8;;https://example.com\x1b\\link text\x1b]8;;\x1b\\ done",
			want:  "link text done",
		},
		{
			name:  "DCS string",
			input: "\x1bPq#0;2;0;0;0#0~~\x1b\\after",
			want:  "after",
		},
		{
			name:  "charset designation ESC ( B",
			input: "\x1b(Btext",
			want:  "text",
		},
		{
			name:  "two-byte sequences ESC = and ESC >",
			input: "\x1b=keypad\x1b>",
			want:  "keypad",
		},
		{
			name:  "bare ESC before a control byte is dropped alone",
			input: "a\x1b\nb",
			want:  "a\nb",
		},
		{
			name:  "doubled ESC before CSI",
			input: "\x1b\x1b[31mred",
			want:  "red",
		},
		{
			name:  "malformed CSI aborted by newline keeps the newline",
			input: "\x1b[31\nplain",
			want:  "\nplain",
		},
		{
			name:  "incomplete CSI at EOF is dropped",
			input: "text\x1b[31",
			want:  "text",
		},
		{
			name:  "incomplete OSC at EOF is dropped",
			input: "text\x1b]0;never ends",
			want:  "text",
		},
		{
			name:  "lone ESC at EOF is dropped",
			input: "end\x1b",
			want:  "end",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "pure ASCII passthrough is byte-identical",
			input: "plain text\nwith\ttabs and \r\n line endings\n",
			want:  "plain text\nwith\ttabs and \r\n line endings\n",
		},
		{
			name:  "UTF-8 passthrough is byte-identical",
			input: "héllo wörld 日本語 🚀\n",
			want:  "héllo wörld 日本語 🚀\n",
		},
		{
			name:  "emoji adjacent to escapes",
			input: "\x1b[32m✅ passed\x1b[0m 🚀\x1b[31m❌ failed\x1b[0m",
			want:  "✅ passed 🚀❌ failed",
		},
		{
			name: "realistic CI log",
			input: "\x1b[1mRun go test ./...\x1b[0m\n" +
				"\x1b[2K\rdownloading modules 42%\x1b[2K\rdownloading modules 100%\n" +
				"\x1b[32m✓\x1b[0m internal/encode: 14 passed \x1b[90m(0.02s)\x1b[0m\n" +
				"\x1b[31mFAIL\x1b[0m internal/format: TestYAML \x1b[90m(0.01s)\x1b[0m\n" +
				"\x1b]8;;https://ci.example.com/runs/1234\x1b\\view full run\x1b]8;;\x1b\\\n",
			want: "Run go test ./...\n" +
				"\rdownloading modules 42%\rdownloading modules 100%\n" +
				"✓ internal/encode: 14 passed (0.02s)\n" +
				"FAIL internal/format: TestYAML (0.01s)\n" +
				"view full run\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			if err := StripANSI(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatalf("StripANSI(%q) returned error: %v", tt.input, err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripANSIReadError(t *testing.T) {
	readErr := errors.New("boom")
	var out strings.Builder
	err := StripANSI(&out, iotest.ErrReader(readErr))
	if !errors.Is(err, readErr) {
		t.Fatalf("StripANSI with failing reader returned %v, want wrapped %v", err, readErr)
	}
}
