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

func TestConvertCommandOutputFile(t *testing.T) {
	failing := ConvertCommand("fail2nothing", "always fails", func(w io.Writer, r io.Reader) error {
		if _, err := io.WriteString(w, "partial"); err != nil {
			return err
		}
		return errors.New("decode failed")
	})
	shout := ConvertCommand("shout2whisper", "test converter", lower)
	write := func(t *testing.T, path, data string, perm os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), perm); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, perm); err != nil {
			t.Fatal(err)
		}
	}
	read := func(t *testing.T, path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	t.Run("failure leaves an existing output untouched", func(t *testing.T) {
		dir := t.TempDir()
		in, out := filepath.Join(dir, "in"), filepath.Join(dir, "keep")
		write(t, in, "HI", 0o644)
		write(t, out, "precious", 0o644)
		if err := failing.Run([]string{in, out}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
			t.Fatal("Run() expected an error, got nil")
		}
		if got := read(t, out); got != "precious" {
			t.Errorf("output after failure = %q, want it untouched", got)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 2 {
			t.Errorf("temporary file left behind: %v", entries)
		}
	})

	t.Run("failure creates no new output", func(t *testing.T) {
		dir := t.TempDir()
		in, out := filepath.Join(dir, "in"), filepath.Join(dir, "new")
		write(t, in, "HI", 0o644)
		if err := failing.Run([]string{in, out}, strings.NewReader(""), io.Discard, io.Discard); err == nil {
			t.Fatal("Run() expected an error, got nil")
		}
		if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("output exists after failure (stat err = %v)", err)
		}
	})

	t.Run("output may be the input file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "same")
		write(t, path, "HI", 0o644)
		if err := shout.Run([]string{path, path}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if got := read(t, path); got != "hi" {
			t.Errorf("in-place result = %q, want %q", got, "hi")
		}
	})

	t.Run("replacing an output keeps its permissions", func(t *testing.T) {
		dir := t.TempDir()
		in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out")
		write(t, in, "HI", 0o644)
		write(t, out, "old", 0o600)
		if err := shout.Run([]string{in, out}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		info, err := os.Stat(out)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("output mode = %v, want 0600", got)
		}
	})

	t.Run("panic leaves no temporary file", func(t *testing.T) {
		dir := t.TempDir()
		in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out")
		write(t, in, "HI", 0o644)
		panicky := ConvertCommand("panic2png", "panics mid-write", func(w io.Writer, r io.Reader) error {
			io.WriteString(w, "partial")
			panic("decoder bug")
		})
		func() {
			defer func() { recover() }()
			panicky.Run([]string{in, out}, strings.NewReader(""), io.Discard, io.Discard)
		}()
		if entries, _ := os.ReadDir(dir); len(entries) != 1 {
			t.Errorf("files after panic = %v, want only the input", entries)
		}
	})

	t.Run("pending outputs are removed on request", func(t *testing.T) {
		dir := t.TempDir()
		out, err := openOutput(filepath.Join(dir, "out"), io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(out, "partial"); err != nil {
			t.Fatal(err)
		}
		RemovePendingOutputs()
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("files after RemovePendingOutputs = %v, want none", entries)
		}
		out.Abort()
	})

	t.Run("special files are written directly", func(t *testing.T) {
		if _, err := os.Stat(os.DevNull); err != nil {
			t.Skip(os.DevNull, "unavailable")
		}
		dir := t.TempDir()
		in := filepath.Join(dir, "in")
		write(t, in, "HI", 0o644)
		if err := shout.Run([]string{in, os.DevNull}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if _, err := os.Stat(os.DevNull); err != nil {
			t.Errorf("%s gone after writing to it: %v", os.DevNull, err)
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
		{name: "custom quality via -q", args: []string{"-q", "95"}, wantQuality: 95},
		{name: "custom quality via -quality", args: []string{"-quality", "40"}, wantQuality: 40},
		{name: "quality too low", args: []string{"-q", "0"}, wantErr: true, wantUsage: true},
		{name: "quality too high", args: []string{"-quality", "101"}, wantErr: true, wantUsage: true},
		{name: "both -q and -quality", args: []string{"-q", "50", "-quality", "90"}, wantErr: true, wantUsage: true},
		{name: "-q twice still last wins", args: []string{"-q", "50", "-q", "60"}, wantQuality: 60},
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
