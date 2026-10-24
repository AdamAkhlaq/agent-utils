package text

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestMarkdownTableBoundedPadding(t *testing.T) {
	atLimit := strings.Repeat("x", 80)
	wide := strings.Repeat("x", 81)
	unicode := strings.Repeat("é", 80)
	tests := []struct {
		name   string
		render func(io.Writer, io.Reader) error
		input  string
		want   string
	}{
		{
			name: "csv at limit still aligns", render: MarkdownTableCSV,
			input: "a\n" + atLimit + "\ny\n",
			want:  "| a" + strings.Repeat(" ", 79) + " |\n| " + strings.Repeat("-", 80) + " |\n| " + atLimit + " |\n| y" + strings.Repeat(" ", 79) + " |\n",
		},
		{
			name: "csv over limit is unpadded", render: MarkdownTableCSV,
			input: "a\n" + wide + "\ny\n",
			want:  "| a |\n| --- |\n| " + wide + " |\n| y |\n",
		},
		{
			name: "json objects over limit", render: MarkdownTable,
			input: `[{"a":"` + wide + `"},{"a":"y"}]`,
			want:  "| a |\n| --- |\n| " + wide + " |\n| y |\n",
		},
		{
			name: "json arrays with wide header and missing cell", render: MarkdownTable,
			input: `[["` + wide + `"],["y"],[]]`,
			want:  "| " + wide + " |\n| --- |\n| y |\n|  |\n",
		},
		{
			name: "wide final row changes the whole column", render: MarkdownTableCSV,
			input: "a\ny\n" + wide + "\n",
			want:  "| a |\n| --- |\n| y |\n| " + wide + " |\n",
		},
		{
			name: "narrow columns keep their padding", render: MarkdownTableCSV,
			input: "a,b\n" + wide + ",z\ny,q\n",
			want:  "| a | b   |\n| --- | --- |\n| " + wide + " | z   |\n| y | q   |\n",
		},
		{
			name: "width counts runes rather than bytes", render: MarkdownTableCSV,
			input: "a\n" + unicode + "\n",
			want:  "| a" + strings.Repeat(" ", 79) + " |\n| " + strings.Repeat("-", 80) + " |\n| " + unicode + " |\n",
		},
		{
			name: "escaping counts toward column width", render: MarkdownTableCSV,
			input: "a\n" + strings.Repeat("|", 41) + "\n",
			want:  "| a |\n| --- |\n| " + strings.Repeat(`\|`, 41) + " |\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := tt.render(&out, strings.NewReader(tt.input)); err != nil {
				t.Fatal(err)
			}
			if out.String() != tt.want {
				t.Errorf("output = %q, want %q", out.String(), tt.want)
			}
		})
	}
}

func TestMarkdownTableValidatesBeforeWriting(t *testing.T) {
	tests := []struct {
		name   string
		render func(io.Writer, io.Reader) error
		input  string
	}{
		{"csv late ragged row", MarkdownTableCSV, "a,b\n" + strings.Repeat("x,y\n", 2000) + "bad\n"},
		{"json trailing data", MarkdownTable, `[{"a":"` + strings.Repeat("x", 10000) + `"}] trailing`},
		{"json late nested value", MarkdownTable, `[{"a":"` + strings.Repeat("x", 10000) + `"},{"a":[]}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := tt.render(&out, strings.NewReader(tt.input)); err == nil {
				t.Fatal("expected invalid input to fail")
			}
			if out.Len() != 0 {
				t.Errorf("wrote %d bytes before rejecting invalid input", out.Len())
			}
		})
	}
}

type tableFailWriter struct {
	err       error
	writes    int
	failAfter int
}

func (w *tableFailWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.failAfter {
		return 0, w.err
	}
	return len(p), nil
}

func TestMarkdownTableWriteErrors(t *testing.T) {
	writeErr := errors.New("output unavailable")
	for _, failAfter := range []int{0, 1} {
		name := "first write"
		if failAfter == 1 {
			name = "later write"
		}
		t.Run(name, func(t *testing.T) {
			writer := &tableFailWriter{err: writeErr, failAfter: failAfter}
			err := MarkdownTableCSV(writer, strings.NewReader("a\n"+strings.Repeat("value\n", 2000)))
			if !errors.Is(err, writeErr) {
				t.Errorf("error = %v, want it to wrap %v", err, writeErr)
			}
		})
	}
}

type tableByteCounter int

func (w *tableByteCounter) Write(p []byte) (int, error) {
	*w += tableByteCounter(len(p))
	return len(p), nil
}

func FuzzMarkdownTable(f *testing.F) {
	f.Add(false, `[{"a":"value"}]`)
	f.Add(false, `[["a"],["x"],[]]`)
	f.Add(true, "a\n"+strings.Repeat("x", 81)+"\ny\n")
	f.Add(true, "a,b\nx,y\nbad\n")
	f.Fuzz(func(t *testing.T, csv bool, input string) {
		if len(input) > 1024 {
			t.Skip("keep fuzz probes small")
		}
		render := MarkdownTable
		if csv {
			render = MarkdownTableCSV
		}
		var written tableByteCounter
		err := render(&written, strings.NewReader(input))
		if err != nil && written != 0 {
			t.Fatalf("invalid input produced %d output bytes: %v", written, err)
		}
	})
}
