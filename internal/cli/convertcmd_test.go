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

func TestConvertCommand(t *testing.T) {
	cmd := ConvertCommand("shout2whisper", "test converter", lower)

	t.Run("stdin to stdout", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if err := cmd.Run(nil, strings.NewReader("HI"), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := stdout.String(), "hi"; got != want {
			t.Errorf("Run() stdout = %q, want %q", got, want)
		}
	})

	t.Run("file to stdout", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "in.txt")
		if err := os.WriteFile(path, []byte("HI"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if err := cmd.Run([]string{path}, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got, want := stdout.String(), "hi"; got != want {
			t.Errorf("Run() stdout = %q, want %q", got, want)
		}
	})

	t.Run("file to file leaves stdout empty", func(t *testing.T) {
		dir := t.TempDir()
		inPath := filepath.Join(dir, "in.txt")
		outPath := filepath.Join(dir, "out.txt")
		if err := os.WriteFile(inPath, []byte("HI"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if err := cmd.Run([]string{inPath, outPath}, strings.NewReader(""), &stdout, &stderr); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		got, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatal(err)
		}
		if want := "hi"; string(got) != want {
			t.Errorf("output file = %q, want %q", got, want)
		}
		if stdout.Len() != 0 {
			t.Errorf("Run() stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("errors", func(t *testing.T) {
		tests := []struct {
			name      string
			args      []string
			wantUsage bool
		}{
			{name: "three arguments", args: []string{"a", "b", "c"}, wantUsage: true},
			{name: "same input and output", args: []string{"x.png", "./x.png"}, wantUsage: true},
			{name: "bad flag", args: []string{"-x"}, wantUsage: true},
			{name: "missing input file", args: []string{"no-such-file"}, wantUsage: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				err := cmd.Run(tt.args, strings.NewReader(""), &stdout, &stderr)
				if err == nil {
					t.Fatal("Run() expected an error, got nil")
				}
				var usageErr *UsageError
				if got := errors.As(err, &usageErr); got != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", got, err, tt.wantUsage)
				}
			})
		}
	})
}

func TestPNGToJPEGCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantQuality int
		wantErr     bool
		wantUsage   bool
	}{
		{name: "default quality", args: nil, wantQuality: 85},
		{name: "custom quality", args: []string{"-q", "95"}, wantQuality: 95},
		{name: "quality too low", args: []string{"-q", "0"}, wantErr: true, wantUsage: true},
		{name: "quality too high", args: []string{"-q", "101"}, wantErr: true, wantUsage: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuality := 0
			cmd := PNGToJPEGCommand(func(w io.Writer, r io.Reader, quality int) error {
				gotQuality = quality
				_, err := io.Copy(w, r)
				return err
			})
			var stdout, stderr bytes.Buffer
			err := cmd.Run(tt.args, strings.NewReader("data"), &stdout, &stderr)
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
			if gotQuality != tt.wantQuality {
				t.Errorf("Run() quality = %d, want %d", gotQuality, tt.wantQuality)
			}
			if got, want := stdout.String(), "data"; got != want {
				t.Errorf("Run() stdout = %q, want %q", got, want)
			}
		})
	}
}
