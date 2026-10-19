package encode

import (
	"errors"
	"fmt"
	"strings"
)

// ShellQuote returns s as one POSIX shell word that evaluates to exactly s.
// It follows Python's shlex.quote: a string made only of ASCII letters,
// digits, and @%+=:,./-_ stays bare, the empty string becomes two single
// quotes, and anything else is wrapped in single quotes, with each ' written
// as '"'"'. The one deviation is that a leading = is quoted too, because zsh
// expands a bare =name to the path of the command name. Any byte other than
// NUL is allowed, invalid UTF-8 included; NUL cannot be passed in a shell
// argument, so it is an error.
func ShellQuote(s string) (string, error) {
	if i := strings.IndexByte(s, 0); i >= 0 {
		return "", fmt.Errorf("NUL byte at byte offset %d cannot appear in a shell word", i)
	}
	if s == "" {
		return "''", nil
	}
	if isShellSafe(s) {
		return s, nil
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'", nil
}

func isShellSafe(s string) bool {
	if s[0] == '=' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case strings.IndexByte("@%+=:,./-_", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// ShellUnquote parses s as exactly one POSIX shell word and returns its
// literal value. It understands single quotes, double quotes (where a
// backslash escapes only $, `, ", \ and newline), backslash escapes outside
// quotes, and backslash-newline line continuations; blanks (space, tab,
// newline) around the word are ignored. Anything a shell would not read
// literally is an error: parameter expansion, command substitution, $'...'
// quoting, and these unquoted: glob characters, braces, a ~ that would
// expand, zsh's leading =, a comment, an operator, or a second word.
func ShellUnquote(s string) (string, error) {
	start := skipShellBlanks(s, 0)
	if start == len(s) {
		return "", errors.New("no shell word in input")
	}
	var b strings.Builder
	// prev is the last byte added outside quotes, or 0 after a quoted or
	// escaped one, which is what decides whether a ~ expands.
	var prev byte
	i := start
	for i < len(s) {
		c := s[i]
		switch c {
		case ' ', '\t', '\n':
			if next := skipShellBlanks(s, i); next < len(s) {
				return "", fmt.Errorf("more than one shell word: another starts at byte offset %d", next)
			}
			return b.String(), nil
		case 0:
			return "", fmt.Errorf("NUL byte at byte offset %d cannot appear in a shell word", i)
		case '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return "", fmt.Errorf("unterminated single quote starting at byte offset %d", i)
			}
			quoted := s[i+1 : i+1+end]
			if j := strings.IndexByte(quoted, 0); j >= 0 {
				return "", fmt.Errorf("NUL byte at byte offset %d cannot appear in a shell word", i+1+j)
			}
			b.WriteString(quoted)
			i += end + 2
			prev = 0
		case '"':
			next, err := readDoubleQuoted(&b, s, i)
			if err != nil {
				return "", err
			}
			i = next
			prev = 0
		case '\\':
			switch {
			case i+1 == len(s):
				return "", fmt.Errorf("backslash at byte offset %d escapes nothing", i)
			case s[i+1] == '\n':
			case s[i+1] == 0:
				return "", fmt.Errorf("NUL byte at byte offset %d cannot appear in a shell word", i+1)
			default:
				b.WriteByte(s[i+1])
				prev = 0
			}
			i += 2
		default:
			if err := checkUnquoted(c, i, i == start, b.Len() == 0, prev); err != nil {
				return "", err
			}
			b.WriteByte(c)
			i++
			prev = c
		}
	}
	return b.String(), nil
}

// skipShellBlanks returns the offset of the first byte at or after i that is
// neither a blank nor part of a backslash-newline line continuation.
func skipShellBlanks(s string, i int) int {
	for i < len(s) {
		switch {
		case s[i] == ' ' || s[i] == '\t' || s[i] == '\n':
			i++
		case strings.HasPrefix(s[i:], "\\\n"):
			i += 2
		default:
			return i
		}
	}
	return i
}

// readDoubleQuoted appends the value of the double-quoted string opening at
// s[open] to b and returns the offset just past its closing quote.
func readDoubleQuoted(b *strings.Builder, s string, open int) (int, error) {
	for i := open + 1; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			return i + 1, nil
		case '\\':
			if i+1 < len(s) {
				switch s[i+1] {
				case '$', '`', '"', '\\':
					b.WriteByte(s[i+1])
					i++
					continue
				case '\n':
					i++
					continue
				}
			}
			b.WriteByte(c)
		case '$':
			return 0, fmt.Errorf("$ at byte offset %d inside double quotes starts an expansion; only literal words can be decoded", i)
		case '`':
			return 0, fmt.Errorf("` at byte offset %d inside double quotes starts a command substitution; only literal words can be decoded", i)
		case 0:
			return 0, fmt.Errorf("NUL byte at byte offset %d cannot appear in a shell word", i)
		default:
			b.WriteByte(c)
		}
	}
	return 0, fmt.Errorf("unterminated double quote starting at byte offset %d", open)
}

// checkUnquoted rejects an unquoted byte that a shell would not take
// literally. first reports whether it is the word's first byte, empty whether
// the word's value is still empty (zsh expands ~ and =ls even after an empty
// quoted string), and prev is the unquoted byte before it (0 if there is
// none).
func checkUnquoted(c byte, i int, first, empty bool, prev byte) error {
	var why string
	switch {
	case c == '$':
		why = "starts an expansion"
	case c == '`':
		why = "starts a command substitution"
	case c == '*' || c == '?' || c == '[':
		why = "is a glob pattern character"
	case c == '{' || c == '}':
		why = "is a brace, which bash and zsh can read as brace expansion or a command group"
	case strings.IndexByte("|&;<>()", c) >= 0:
		why = "is a shell operator"
	case c == '~' && (empty || prev == '=' || prev == ':'):
		why = "is a tilde expansion"
	case c == '=' && empty:
		why = "starts a command path expansion in zsh"
	case c == '#' && first:
		why = "starts a comment"
	default:
		return nil
	}
	return fmt.Errorf("unquoted %c at byte offset %d %s; only literal words can be decoded", c, i, why)
}
