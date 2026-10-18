package main

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
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
