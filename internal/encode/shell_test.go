package encode

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "safe characters stay bare", input: "a-Z_0.9/x:y,z@w%v+u=t", want: "a-Z_0.9/x:y,z@w%v+u=t"},
		{name: "empty string", input: "", want: "''"},
		{name: "space", input: "a b", want: "'a b'"},
		{name: "single quote", input: "it's", want: `'it'"'"'s'`},
		{name: "only a single quote", input: "'", want: `''"'"''`},
		{name: "double quotes and backslash", input: `say "\n"`, want: `'say "\n"'`},
		{name: "expansion characters", input: "$HOME `id` $(id) *", want: "'$HOME `id` $(id) *'"},
		{name: "newline and tab", input: "a\nb\tc", want: "'a\nb\tc'"},
		{name: "leading tilde", input: "~/x", want: "'~/x'"},
		{name: "leading dash stays bare", input: "-rf", want: "-rf"},
		{name: "leading equals is quoted for zsh", input: "=ls", want: "'=ls'"},
		{name: "inner equals stays bare", input: "--opt=v", want: "--opt=v"},
		{name: "unicode", input: "café ☕", want: "'café ☕'"},
		{name: "invalid UTF-8 is allowed", input: "a\xffb", want: "'a\xffb'"},
		{name: "NUL", input: "a\x00b", wantErr: "NUL byte at byte offset 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ShellQuote(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ShellQuote() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ShellQuote() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ShellQuote() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShellUnquote(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "bare word", input: "abc", want: "abc"},
		{name: "single quotes", input: "'a b $x `y` \\'", want: "a b $x `y` \\"},
		{name: "quoted empty string", input: "''", want: ""},
		{name: "double quoted empty string", input: `""`, want: ""},
		{name: "embedded single quote, double quote form", input: `'it'"'"'s'`, want: "it's"},
		{name: "embedded single quote, backslash form", input: `'it'\''s'`, want: "it's"},
		{name: "double quote escapes", input: `"a\$b\` + "`" + `c\"d\\e"`, want: "a$b`c\"d\\e"},
		{name: "double quote keeps other backslashes", input: `"a\nb\'c"`, want: `a\nb\'c`},
		{name: "double quote line continuation", input: "\"a\\\nb\"", want: "ab"},
		{name: "double quotes keep newlines", input: "\"a\nb\"", want: "a\nb"},
		{name: "backslash escapes outside quotes", input: `a\ b\$c\'d\"e\\f`, want: `a b$c'd"e\f`},
		{name: "line continuation outside quotes", input: "ab\\\ncd", want: "abcd"},
		{name: "concatenated parts", input: `a'b c'"d e"f\ g`, want: "ab cd ef g"},
		{name: "surrounding blanks", input: " \t\n'a b'\n\t ", want: "a b"},
		{name: "leading line continuation", input: "\\\nabc", want: "abc"},
		{name: "trailing line continuation", input: "abc\\\n", want: "abc"},
		{name: "unicode", input: "café'☕'", want: "café☕"},
		{name: "invalid UTF-8 is allowed", input: "'a\xffb'", want: "a\xffb"},
		{name: "inner tilde is literal", input: "a~b", want: "a~b"},
		{name: "tilde after quoted text is literal", input: "'a'~", want: "a~"},
		{name: "hash after an empty quoted string is literal", input: "''#", want: "#"},
		{name: "escaped tilde", input: `\~`, want: "~"},
		{name: "inner hash is literal", input: "a#b", want: "a#b"},
		{name: "inner equals is literal", input: "--opt=v", want: "--opt=v"},
		{name: "closing bracket is literal", input: "a]", want: "a]"},
		{name: "carriage return is part of the word", input: "a\r", want: "a\r"},
		{name: "empty input", input: "", wantErr: "no shell word in input"},
		{name: "only blanks", input: " \n\t", wantErr: "no shell word in input"},
		{name: "two words", input: "a b", wantErr: "more than one shell word: another starts at byte offset 2"},
		{name: "two words across a newline", input: "'a'\n'b'", wantErr: "more than one shell word"},
		{name: "two words across a line continuation", input: "a \\\nb", wantErr: "another starts at byte offset 4"},
		{name: "unterminated single quote", input: "a'bc", wantErr: "unterminated single quote starting at byte offset 1"},
		{name: "unterminated double quote", input: `"abc\"`, wantErr: "unterminated double quote starting at byte offset 0"},
		{name: "trailing backslash", input: `abc\`, wantErr: "backslash at byte offset 3 escapes nothing"},
		{name: "parameter expansion", input: "$HOME", wantErr: "unquoted $ at byte offset 0 starts an expansion"},
		{name: "ANSI-C quoting", input: "$'a'", wantErr: "unquoted $"},
		{name: "command substitution", input: "a$(id)", wantErr: "unquoted $ at byte offset 1"},
		{name: "backticks", input: "`id`", wantErr: "starts a command substitution"},
		{name: "expansion in double quotes", input: `"a $b"`, wantErr: "$ at byte offset 3 inside double quotes starts an expansion"},
		{name: "backticks in double quotes", input: "\"`id`\"", wantErr: "inside double quotes starts a command substitution"},
		{name: "glob star", input: "*.go", wantErr: "unquoted * at byte offset 0 is a glob pattern character"},
		{name: "glob question mark", input: "a?", wantErr: "is a glob pattern character"},
		{name: "glob bracket", input: "[ab]", wantErr: "is a glob pattern character"},
		{name: "brace expansion", input: "a{b,c}", wantErr: "unquoted { at byte offset 1 is a brace"},
		{name: "closing brace ends a zsh command group", input: "a}", wantErr: "unquoted } at byte offset 1 is a brace"},
		{name: "leading tilde", input: "~/x", wantErr: "unquoted ~ at byte offset 0 is a tilde expansion"},
		{name: "tilde after equals", input: "--dir=~/x", wantErr: "is a tilde expansion"},
		{name: "tilde after colon", input: "a:~", wantErr: "is a tilde expansion"},
		{name: "leading equals", input: "=ls", wantErr: "starts a command path expansion in zsh"},
		{name: "tilde after an empty quoted string", input: "''~", wantErr: "unquoted ~ at byte offset 2 is a tilde expansion"},
		{name: "equals after an empty quoted string", input: `""=ls`, wantErr: "unquoted = at byte offset 2 starts a command path expansion in zsh"},
		{name: "comment", input: "#x", wantErr: "starts a comment"},
		{name: "pipe", input: "a|b", wantErr: "unquoted | at byte offset 1 is a shell operator"},
		{name: "semicolon", input: "a;", wantErr: "is a shell operator"},
		{name: "redirect", input: "a>b", wantErr: "is a shell operator"},
		{name: "subshell", input: "(a)", wantErr: "is a shell operator"},
		{name: "ampersand", input: "a&", wantErr: "is a shell operator"},
		{name: "NUL unquoted", input: "a\x00", wantErr: "NUL byte at byte offset 1"},
		{name: "NUL in single quotes", input: "'a\x00'", wantErr: "NUL byte at byte offset 2"},
		{name: "NUL in double quotes", input: "\"a\x00\"", wantErr: "NUL byte at byte offset 2"},
		{name: "escaped NUL", input: "\\\x00", wantErr: "NUL byte at byte offset 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ShellUnquote(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ShellUnquote() = %q, error = %v, want an error containing %q", got, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ShellUnquote() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ShellUnquote() = %q, want %q", got, tt.want)
			}
		})
	}
}

func FuzzShellRoundTrip(f *testing.F) {
	for _, s := range []string{"", "abc", "it's", "a b\nc", "=x", "~", "'\"\\$`", "café", "a\xffb", "a\x00b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		quoted, err := ShellQuote(s)
		if strings.IndexByte(s, 0) >= 0 {
			if err == nil {
				t.Fatalf("ShellQuote(%q) accepted a NUL byte", s)
			}
			return
		}
		if err != nil {
			t.Fatalf("ShellQuote(%q) error = %v", s, err)
		}
		got, err := ShellUnquote(quoted)
		if err != nil {
			t.Fatalf("ShellUnquote(%q) error = %v", quoted, err)
		}
		if got != s {
			t.Fatalf("ShellUnquote(ShellQuote(%q)) = %q", s, got)
		}
	})
}

// Whatever ShellUnquote accepts must survive a round trip through ShellQuote.
func FuzzShellUnquote(f *testing.F) {
	for _, s := range []string{"a", `'a'"b"\c`, "$x", "\"\\\n\"", "a b", "\\"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, err := ShellUnquote(s)
		if err != nil {
			return
		}
		quoted, err := ShellQuote(v)
		if err != nil {
			t.Fatalf("ShellQuote(%q) error = %v", v, err)
		}
		if got, err := ShellUnquote(quoted); err != nil || got != v {
			t.Fatalf("ShellUnquote(%q) = %q, %v, want %q", quoted, got, err, v)
		}
	})
}
