package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adamakhlaq/agent-utils/v2/internal/img"
)

// These tests pin the exit-code contract end to end through the real command
// registry: an agent branches on these codes alone, so a predicate's "no"
// must never share a code with a failure.
func TestExitCodes(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  int
	}{
		{name: "success", args: []string{"hex"}, stdin: "hi", want: 0},
		{name: "runtime failure", args: []string{"hex", "-d"}, stdin: "zz", want: 1},
		{name: "missing input file", args: []string{"hex", missing}, want: 1},
		{name: "usage error", args: []string{"hex", "-x"}, want: 2},
		{name: "unknown command", args: []string{"nonsense"}, want: 2},
		{name: "help", args: []string{"--help"}, want: 0},
		{name: "help help", args: []string{"help", "help"}, want: 0},
		{name: "help -h", args: []string{"help", "-h"}, want: 0},
		{name: "cidr contains true", args: []string{"cidr", "contains", "10.0.0.0/8", "10.1.2.3"}, want: 0},
		{name: "cidr contains false", args: []string{"cidr", "contains", "10.0.0.0/8", "192.168.0.1"}, want: 3},
		{name: "cidr overlaps false", args: []string{"cidr", "overlaps", "10.0.0.0/25", "10.0.0.128/25"}, want: 3},
		{name: "cidr malformed prefix is bad data", args: []string{"cidr", "info", "10.0.0.0/33"}, want: 1},
		{name: "cidr malformed address is bad data", args: []string{"cidr", "contains", "10.0.0.0/8", "10.0.0.256"}, want: 1},
		{name: "cidr split length out of range", args: []string{"cidr", "split", "10.0.0.0/24", "20"}, want: 2},
		{name: "cidr split too many subnets", args: []string{"cidr", "split", "10.0.0.0/8", "31"}, want: 2},
		{name: "cidr split malformed prefix", args: []string{"cidr", "split", "bad", "24"}, want: 1},
		{name: "color malformed value", args: []string{"color", "-to", "hex", "nonsense"}, want: 1},
		{name: "color bad flag value", args: []string{"color", "-to", "cmyk", "#fff"}, want: 2},
		{name: "semver compare malformed version", args: []string{"semver", "-compare", "1.2", "1.3"}, want: 1},
		{name: "time malformed timestamp", args: []string{"time", "garbage"}, want: 1},
		{name: "urlparse malformed url", args: []string{"urlparse", "http://[::1"}, want: 1},
		{name: "calc success", args: []string{"calc", "-2^2"}, want: 0},
		{name: "calc syntax error", args: []string{"calc", "1 +"}, want: 1},
		{name: "calc division by zero", args: []string{"calc", "1/0"}, want: 1},
		{name: "calc empty expression", args: []string{"calc"}, stdin: "\n", want: 1},
		{name: "calc oversized result", args: []string{"calc", "10^10^10"}, want: 1},
		{name: "calc bad scale", args: []string{"calc", "-scale", "-1", "1/3"}, want: 2},
		{name: "calc unquoted expression", args: []string{"calc", "1", "+", "2"}, want: 2},
		{name: "semver check true", args: []string{"semver", "-check", ">=1.0.0", "1.2.3"}, want: 0},
		{name: "semver check false", args: []string{"semver", "-check", ">=2.0.0", "1.2.3"}, want: 3},
		{name: "semver check invalid version", args: []string{"semver", "-check", ">=1.0.0", "bad"}, want: 1},
		{name: "json-fmt check valid", args: []string{"json-fmt", "-check"}, stdin: `{"a": 1}`, want: 0},
		{name: "json-fmt check invalid", args: []string{"json-fmt", "-check"}, stdin: `{"a": }`, want: 3},
		{name: "json-fmt check missing file", args: []string{"json-fmt", "-check", missing}, want: 1},
		{name: "json-fmt pretty invalid", args: []string{"json-fmt"}, stdin: `{"a": }`, want: 1},
		{name: "hash verify match", args: []string{"hash", "-c", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"}, stdin: "abc", want: 0},
		{name: "hash verify mismatch", args: []string{"hash", "-c", strings.Repeat("0", 64)}, stdin: "abc", want: 3},
		{name: "hash verify missing file", args: []string{"hash", "-c", strings.Repeat("0", 64), missing}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := run(tt.args, strings.NewReader(tt.stdin), &stdout, io.Discard)
			if got := exitCode(err); got != tt.want {
				t.Errorf("exit code = %d (err = %v), want %d", got, err, tt.want)
			}
		})
	}
}

func TestRecoverPanic(t *testing.T) {
	err := recoverPanic(func() error { panic("boom") })
	if err == nil || !strings.Contains(err.Error(), "internal error") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("recoverPanic() error = %v, want an internal error carrying the panic value", err)
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exit code = %d, want 1", got)
	}
	want := errors.New("plain")
	if got := recoverPanic(func() error { return want }); got != want {
		t.Errorf("recoverPanic() = %v, want the function's own error %v", got, want)
	}
}

// Every command must be documented well enough for an agent to use it from
// -h alone, and examples may only name commands that exist.
func TestRegistryDocumentation(t *testing.T) {
	commands := registry()
	for name, cmd := range commands {
		t.Run(name, func(t *testing.T) {
			if cmd.Summary == "" {
				t.Error("missing Summary")
			}
			if !strings.HasPrefix(cmd.Usage, "agent-utils "+name) {
				t.Errorf("Usage = %q, want it to start with %q", cmd.Usage, "agent-utils "+name)
			}
			if len(cmd.Examples) == 0 {
				t.Error("no Examples")
			}
			invokes := regexp.MustCompile(`agent-utils ` + regexp.QuoteMeta(name) + `(\s|$)`)
			for _, ex := range cmd.Examples {
				if !invokes.MatchString(ex.Command) {
					t.Errorf("example %q does not invoke agent-utils %s", ex.Command, name)
				}
			}
		})
	}
	for name := range examples {
		if _, ok := commands[name]; !ok {
			t.Errorf("examples has an entry for unknown command %q", name)
		}
	}
}

// The manifest asks every real command for -h with streams that fail if
// touched, so this also proves no command does work before parsing flags.
func TestCommandsManifest(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"commands"}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("commands error = %v", err)
	}
	var entries []struct {
		Name  string `json:"name"`
		Flags []struct {
			Name string `json:"name"`
		} `json:"flags"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if len(entries) != len(registry()) {
		t.Errorf("manifest lists %d commands, registry has %d", len(entries), len(registry()))
	}
	for _, e := range entries {
		if e.Name == "hash" && len(e.Flags) != 2 {
			t.Errorf("hash flags = %+v, want -a and -c", e.Flags)
		}
	}
}

// docs/commands holds the long-form reference for each command; this keeps it
// from drifting out of step with the binary as commands and flags change.
func TestCommandDocs(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"commands"}, strings.NewReader(""), &stdout, io.Discard); err != nil {
		t.Fatalf("commands error = %v", err)
	}
	var entries []struct {
		Name  string   `json:"name"`
		Usage []string `json:"usage"`
		Flags []struct {
			Name string `json:"name"`
		} `json:"flags"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	documented := map[string]bool{}
	for _, e := range entries {
		documented[e.Name] = true
		t.Run(e.Name, func(t *testing.T) {
			page, err := os.ReadFile(filepath.Join("docs", "commands", e.Name+".md"))
			if err != nil {
				t.Fatalf("missing docs page: %v", err)
			}
			for _, line := range e.Usage {
				if !strings.Contains(string(page), line) {
					t.Errorf("page does not show usage line %q", line)
				}
			}
			for _, f := range e.Flags {
				if !strings.Contains(string(page), "`"+f.Name+"`") && !strings.Contains(string(page), "`"+f.Name+" ") {
					t.Errorf("page does not document flag %s", f.Name)
				}
			}
		})
	}
	pages, err := filepath.Glob(filepath.Join("docs", "commands", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		if name := strings.TrimSuffix(filepath.Base(p), ".md"); !documented[name] {
			t.Errorf("%s documents %q, which is not a command", p, name)
		}
	}
}

// TestExamples runs every example that states its output and needs no shell
// (no pipes, redirects, or variables), so the outputs shown by -h and the
// manifest cannot drift from what the binary prints.
func TestExamples(t *testing.T) {
	t.Chdir(t.TempDir())
	var qrPNG bytes.Buffer
	if err := img.QR(&qrPNG, "https://example.com", 256); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"qr.png", "photo.png"} {
		if err := os.WriteFile(name, qrPNG.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	quoted := regexp.MustCompile(`'[^']*'|"[^"]*"`)
	ran := 0
	for name, exs := range examples {
		for _, ex := range exs {
			if ex.Output == "" || strings.ContainsAny(quoted.ReplaceAllString(ex.Command, ""), "|<>$;&") {
				continue
			}
			ran++
			t.Run(name+"/"+ex.Command, func(t *testing.T) {
				words, err := shellWords(ex.Command)
				if err != nil {
					t.Fatal(err)
				}
				if len(words) == 0 || words[0] != "agent-utils" {
					t.Fatalf("example does not start with agent-utils: %q", ex.Command)
				}
				var stdout bytes.Buffer
				if err := run(words[1:], strings.NewReader(""), &stdout, io.Discard); err != nil {
					t.Fatalf("example failed: %v", err)
				}
				if got := strings.TrimSuffix(stdout.String(), "\n"); got != ex.Output {
					t.Errorf("example prints %q, its stated output is %q", got, ex.Output)
				}
			})
		}
	}
	if ran == 0 {
		t.Fatal("no example was runnable; the filter is broken")
	}
}

// shellWords splits a command line the way a POSIX shell would for the
// subset the examples use: whitespace-separated words, single quotes, and
// double quotes without escapes.
func shellWords(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case c == '\'' || c == '"':
			end := strings.IndexByte(line[i+1:], c)
			if end == -1 {
				return nil, fmt.Errorf("unterminated %c quote in %q", c, line)
			}
			word.WriteString(line[i+1 : i+1+end])
			i += end + 1
			inWord = true
		case c == ' ' || c == '\t':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}
