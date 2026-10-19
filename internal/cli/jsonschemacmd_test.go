package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The stub compiler accepts any schema path except "broken.json" and
// validates by content: "bad" has two violations, "one-bad" one, "garbage" is
// malformed, and anything else is valid. That isolates the CLI layer's
// argument handling, input routing, and error classification.
func newJSONSchemaStub() (cmd Command, compiled *string, validated *string) {
	compiled, validated = new(string), new(string)
	compile := func(schemaPath string) (func(r io.Reader) (string, int, error), error) {
		if schemaPath == "broken.json" {
			return nil, errors.New("invalid schema broken.json: /type: got number, want string")
		}
		*compiled = schemaPath
		return func(r io.Reader) (string, int, error) {
			data, err := io.ReadAll(r)
			if err != nil {
				return "", 0, err
			}
			*validated = string(data)
			switch *validated {
			case "garbage":
				return "", 0, errors.New("invalid JSON at line 1, column 1")
			case "bad":
				return "REPORT bad", 2, nil
			case "one-bad":
				return "REPORT one-bad", 1, nil
			}
			return "REPORT ok", 0, nil
		}, nil
	}
	return JSONSchemaCommand(compile), compiled, validated
}

func TestJSONSchemaCommand(t *testing.T) {
	dir := t.TempDir()
	instance := filepath.Join(dir, "instance.json")
	if err := os.WriteFile(instance, []byte("bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.json")
	tests := []struct {
		name          string
		args          []string
		stdin         string
		wantStdout    string
		wantSchema    string
		wantValidated string
		wantErr       string
		wantKind      string // "", "usage", "false", or "runtime"
	}{
		{name: "valid from stdin", args: []string{"s.json"}, stdin: "ok", wantStdout: "REPORT ok\n", wantSchema: "s.json", wantValidated: "ok"},
		{name: "invalid from stdin", args: []string{"s.json"}, stdin: "bad", wantStdout: "REPORT bad\n", wantSchema: "s.json", wantValidated: "bad", wantErr: "json-schema: stdin does not match s.json: 2 errors", wantKind: "false"},
		{name: "one violation", args: []string{"s.json"}, stdin: "one-bad", wantStdout: "REPORT one-bad\n", wantValidated: "one-bad", wantSchema: "s.json", wantErr: "does not match s.json: 1 error", wantKind: "false"},
		{name: "invalid from a file", args: []string{"s.json", instance}, stdin: "ok", wantStdout: "REPORT bad\n", wantSchema: "s.json", wantValidated: "bad", wantErr: instance + " does not match s.json", wantKind: "false"},
		{name: "malformed input", args: []string{"s.json"}, stdin: "garbage", wantSchema: "s.json", wantValidated: "garbage", wantErr: "json-schema: stdin: invalid JSON at line 1, column 1", wantKind: "runtime"},
		{name: "broken schema", args: []string{"broken.json"}, stdin: "ok", wantErr: "json-schema: invalid schema broken.json", wantKind: "runtime"},
		{name: "missing input file", args: []string{"s.json", missing}, wantSchema: "s.json", wantErr: "json-schema: open " + missing, wantKind: "runtime"},
		{name: "no arguments", args: nil, wantErr: "expected a schema file and at most one input file, got 0", wantKind: "usage"},
		{name: "three arguments", args: []string{"a", "b", "c"}, wantErr: "got 3 argument(s)", wantKind: "usage"},
		{name: "unknown flag", args: []string{"-x", "s.json"}, wantErr: "flag provided but not defined", wantKind: "usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, compiled, validated := newJSONSchemaStub()
			var stdout bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader(tt.stdin), &stdout, io.Discard)
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if *compiled != tt.wantSchema {
				t.Errorf("compiled schema = %q, want %q", *compiled, tt.wantSchema)
			}
			if *validated != tt.wantValidated {
				t.Errorf("validated input = %q, want %q", *validated, tt.wantValidated)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Run() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Run() error = %v, want it to contain %q", err, tt.wantErr)
			}
			var usageErr *UsageError
			var falseErr *FalseError
			kind := "runtime"
			switch {
			case errors.As(err, &usageErr):
				kind = "usage"
			case errors.As(err, &falseErr):
				kind = "false"
			}
			if kind != tt.wantKind {
				t.Errorf("Run() error kind = %s, want %s (err = %v)", kind, tt.wantKind, err)
			}
		})
	}
}

func TestJSONSchemaCommandHelp(t *testing.T) {
	cmd, compiled, _ := newJSONSchemaStub()
	err := cmd.Run([]string{"-h"}, strings.NewReader(""), io.Discard, io.Discard)
	var help *helpRequest
	if !errors.As(err, &help) {
		t.Fatalf("Run(-h) error = %v, want a help request", err)
	}
	if *compiled != "" {
		t.Errorf("Run(-h) compiled %q, want nothing compiled", *compiled)
	}
}
