package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestImgResizeCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      [4]int // width, height, max, quality
		wantErr   bool
		wantUsage bool
	}{
		{name: "width alone", args: []string{"-width", "40"}, want: [4]int{40, 0, 0, 75}},
		{name: "height alone", args: []string{"-height", "25"}, want: [4]int{0, 25, 0, 75}},
		{name: "width and height together", args: []string{"-width", "30", "-height", "30"}, want: [4]int{30, 30, 0, 75}},
		{name: "max alone", args: []string{"-max", "512"}, want: [4]int{0, 0, 512, 75}},
		{name: "custom quality", args: []string{"-max", "512", "-quality", "90"}, want: [4]int{0, 0, 512, 90}},
		{name: "no sizing flag", args: nil, wantErr: true, wantUsage: true},
		{name: "quality without a sizing flag", args: []string{"-quality", "90"}, wantErr: true, wantUsage: true},
		{name: "max combined with width", args: []string{"-width", "40", "-max", "512"}, wantErr: true, wantUsage: true},
		{name: "max combined with height", args: []string{"-height", "40", "-max", "512"}, wantErr: true, wantUsage: true},
		{name: "explicit zero width", args: []string{"-width", "0"}, wantErr: true, wantUsage: true},
		{name: "negative height", args: []string{"-height", "-5"}, wantErr: true, wantUsage: true},
		{name: "explicit zero max", args: []string{"-max", "0"}, wantErr: true, wantUsage: true},
		{name: "quality too low", args: []string{"-max", "512", "-quality", "0"}, wantErr: true, wantUsage: true},
		{name: "quality too high", args: []string{"-max", "512", "-quality", "101"}, wantErr: true, wantUsage: true},
		{name: "unknown flag", args: []string{"-w", "40"}, wantErr: true, wantUsage: true},
		{name: "too many arguments", args: []string{"-max", "512", "a", "b", "c"}, wantErr: true, wantUsage: true},
		{name: "missing input file", args: []string{"-max", "512", "no-such-file"}, wantErr: true, wantUsage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got [4]int
			cmd := ImgResizeCommand(func(w io.Writer, r io.Reader, width, height, max, quality int) error {
				got = [4]int{width, height, max, quality}
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
				if gotUsage := errors.As(err, &usageErr); gotUsage != tt.wantUsage {
					t.Fatalf("Run() usage error = %v (err = %v), want %v", gotUsage, err, tt.wantUsage)
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Run() passed (width, height, max, quality) = %v, want %v", got, tt.want)
			}
			if gotOut, want := stdout.String(), "data"; gotOut != want {
				t.Errorf("Run() stdout = %q, want %q", gotOut, want)
			}
		})
	}

	t.Run("runtime error from the transform is not a usage error", func(t *testing.T) {
		cmd := ImgResizeCommand(func(w io.Writer, r io.Reader, width, height, max, quality int) error {
			return errors.New("decoding image: boom")
		})
		var stdout, stderr bytes.Buffer
		err := cmd.Run([]string{"-max", "512"}, strings.NewReader("data"), &stdout, &stderr)
		if err == nil {
			t.Fatal("Run() expected an error, got nil")
		}
		var usageErr *UsageError
		if errors.As(err, &usageErr) {
			t.Fatalf("Run() error = %v, want a runtime error, got a usage error", err)
		}
	})
}
