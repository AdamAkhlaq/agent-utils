package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubQREncode(w io.Writer, text string, size int) error {
	_, err := fmt.Fprintf(w, "PNG(%s,%d)", text, size)
	return err
}

func stubQRDecode(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return "DEC:" + string(data), nil
}

func TestQRCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     string
		want      string
		wantErr   bool
		wantUsage bool
	}{
		{name: "encode", args: []string{"hello"}, want: "PNG(hello,256)"},
		{name: "encode with size", args: []string{"-s", "512", "hello"}, want: "PNG(hello,512)"},
		{name: "decode from stdin", args: []string{"-d"}, stdin: "img", want: "DEC:img\n"},
		{name: "no text argument", args: nil, wantErr: true, wantUsage: true},
		{name: "two text arguments", args: []string{"a", "b"}, wantErr: true, wantUsage: true},
		{name: "zero size", args: []string{"-s", "0", "hello"}, wantErr: true, wantUsage: true},
		{name: "bad flag", args: []string{"-x"}, wantErr: true, wantUsage: true},
		{name: "size with decode", args: []string{"-d", "-s", "100"}, stdin: "img", wantErr: true, wantUsage: true},
		{name: "largest size", args: []string{"-s", "1000", "hello"}, want: "PNG(hello,1000)"},
		{name: "size over the largest", args: []string{"-s", "1001", "hello"}, wantErr: true, wantUsage: true},
	}
	cmd := QRCommand(stubQREncode, stubQRDecode, 1000)
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

func TestQRCommandWritesOutputFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.png")
	cmd := QRCommand(stubQREncode, stubQRDecode, 1000)
	var stdout, stderr bytes.Buffer
	if err := cmd.Run([]string{"-o", path, "hello"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "PNG(hello,256)"; got != want {
		t.Errorf("output file = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout with -o, got %q", stdout.String())
	}
}

func TestQRCommandFailureLeavesOutputFileUntouched(t *testing.T) {
	cmd := QRCommand(stubQREncode, stubQRDecode, 1000)
	for name, args := range map[string][]string{
		"missing text":          {},
		"zero size":             {"-s", "0", "hello"},
		"missing decode input":  {"-d", "no-such-file.png"},
		"size with decode mode": {"-d", "-s", "100"},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "keep.png")
			if err := os.WriteFile(path, []byte("precious"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Run(append([]string{"-o", path}, args...), strings.NewReader("img"), io.Discard, io.Discard); err == nil {
				t.Fatal("Run() expected an error, got nil")
			}
			if data, _ := os.ReadFile(path); string(data) != "precious" {
				t.Errorf("output file = %q after a failed run, want it untouched", data)
			}
		})
	}
}

func TestQRCommandDecodesFileArgument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.png")
	if err := os.WriteFile(path, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := QRCommand(stubQREncode, stubQRDecode, 1000)
	var stdout, stderr bytes.Buffer
	if err := cmd.Run([]string{"-d", path}, strings.NewReader("ignored"), &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "DEC:img\n"; got != want {
		t.Errorf("Run() stdout = %q, want %q", got, want)
	}
}
