package text

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
	"testing/iotest"
)

func TestMarkdownTable(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "objects preserve first object's key order",
			input: `[{"zebra":"1","alpha":"2"}]`,
			want: "| zebra | alpha |\n" +
				"| ----- | ----- |\n" +
				"| 1     | 2     |\n",
		},
		{
			name:  "later-only keys appended and missing keys empty",
			input: `[{"name":"Jane","age":30},{"name":"Bob","city":"Oslo","age":4}]`,
			want: "| name | age | city |\n" +
				"| ---- | --- | ---- |\n" +
				"| Jane | 30  |      |\n" +
				"| Bob  | 4   | Oslo |\n",
		},
		{
			name:  "first object empty picks up later keys",
			input: `[{},{"a":"v"}]`,
			want: "| a   |\n" +
				"| --- |\n" +
				"|     |\n" +
				"| v   |\n",
		},
		{
			name:  "numbers keep source form, booleans literal, null empty",
			input: `[{"n":1.10,"b":true,"x":null}]`,
			want: "| n    | b    | x   |\n" +
				"| ---- | ---- | --- |\n" +
				"| 1.10 | true |     |\n",
		},
		{
			name:  "pipes and newlines escaped",
			input: `[{"cmd":"a|b","note":"one\ntwo"}]`,
			want: "| cmd  | note       |\n" +
				"| ---- | ---------- |\n" +
				"| a\\|b | one<br>two |\n",
		},
		{
			name:  "crlf and cr newlines become one br each",
			input: `[{"a":"x\r\ny","b":"p\rq"}]`,
			want: "| a      | b      |\n" +
				"| ------ | ------ |\n" +
				"| x<br>y | p<br>q |\n",
		},
		{
			name:  "unicode width counted in runes",
			input: `[{"café":"naïve"}]`,
			want: "| café  |\n" +
				"| ----- |\n" +
				"| naïve |\n",
		},
		{
			name:  "leading and trailing cell whitespace preserved",
			input: `[{"a":" x "}]`,
			want: "| a   |\n" +
				"| --- |\n" +
				"|  x  |\n",
		},
		{
			name:  "array of arrays with first row as header",
			input: `[["a","b"],["1","22222"]]`,
			want: "| a   | b     |\n" +
				"| --- | ----- |\n" +
				"| 1   | 22222 |\n",
		},
		{
			name:  "array of arrays short row padded",
			input: `[["x","y"],["only"]]`,
			want: "| x    | y   |\n" +
				"| ---- | --- |\n" +
				"| only |     |\n",
		},
		{
			name:  "array of arrays header only",
			input: `[["a","b"]]`,
			want: "| a   | b   |\n" +
				"| --- | --- |\n",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace-only input", input: "  \n", wantErr: "empty input"},
		{name: "empty array", input: `[]`, wantErr: "empty JSON array"},
		{name: "invalid JSON", input: `[{"a":`, wantErr: "invalid JSON"},
		{name: "top-level object", input: `{"a":1}`, wantErr: "must be a JSON array of objects or a JSON array of arrays, got an object"},
		{name: "top-level scalar", input: `"hi"`, wantErr: "got a string"},
		{
			name:  "leading byte order mark skipped",
			input: "\xef\xbb\xbf" + `[{"name":"Jane"}]`,
			want: "| name |\n" +
				"| ---- |\n" +
				"| Jane |\n",
		},
		{name: "byte order mark only is empty input", input: "\xef\xbb\xbf", wantErr: "empty input"},
		{name: "scalar element", input: `[1,2]`, wantErr: "array element 1 must be an object or an array, got a number"},
		{name: "mixed object then array", input: `[{"a":"1"},["b"]]`, wantErr: "array element 2 is an array but element 1 is an object"},
		{name: "mixed array then object", input: `[["a"],{"b":"1"}]`, wantErr: "array element 2 is an object but element 1 is an array"},
		{name: "nested object value", input: `[{"a":{"b":1}}]`, wantErr: `array element 1, key "a": nested objects are not supported`},
		{name: "nested array value", input: `[{"a":[1]}]`, wantErr: `array element 1, key "a": nested arrays are not supported`},
		{name: "nested value in array row", input: `[["a"],[["x"]]]`, wantErr: "array element 2, cell 1: nested arrays are not supported"},
		{name: "duplicate key in object", input: `[{"a":1,"a":2}]`, wantErr: `array element 1: duplicate key "a"`},
		{name: "objects without keys", input: `[{},{}]`, wantErr: "no columns to render"},
		{name: "empty header row", input: `[[]]`, wantErr: "header row (the first inner array) is empty"},
		{name: "row wider than header", input: `[["a"],["1","2"]]`, wantErr: "array element 2 has 2 cells but the header row has only 1"},
		{
			name:  "column at the padding limit is padded",
			input: `[["a","b"],["` + strings.Repeat("x", 80) + `","y"]]`,
			want: "| a" + strings.Repeat(" ", 79) + " | b   |\n" +
				"| " + strings.Repeat("-", 80) + " | --- |\n" +
				"| " + strings.Repeat("x", 80) + " | y   |\n",
		},
		{
			name:  "column past the padding limit is not padded",
			input: `[["a","b"],["` + strings.Repeat("x", 81) + `","y"],["z","w"]]`,
			want: "| a | b   |\n" +
				"| --- | --- |\n" +
				"| " + strings.Repeat("x", 81) + " | y   |\n" +
				"| z | w   |\n",
		},
		{
			name:  "padding limit counts runes, not bytes",
			input: `[["a"],["` + strings.Repeat("é", 80) + `"]]`,
			want: "| a" + strings.Repeat(" ", 79) + " |\n" +
				"| " + strings.Repeat("-", 80) + " |\n" +
				"| " + strings.Repeat("é", 80) + " |\n",
		},
		{
			name:  "padding limit counts the escaped cell",
			input: `[["a"],["` + strings.Repeat("|", 41) + `"]]`,
			want: "| a |\n" +
				"| --- |\n" +
				"| " + strings.Repeat(`\|`, 41) + " |\n",
		},
		{name: "trailing data after array", input: `[["a"]] 2`, wantErr: "unexpected data after the JSON array"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := MarkdownTable(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("MarkdownTable() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("MarkdownTable() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MarkdownTable() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("MarkdownTable() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestMarkdownTableCSV(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "header and rows aligned",
			input: "name,age\nJane,30\nBob,4\n",
			want: "| name | age |\n" +
				"| ---- | --- |\n" +
				"| Jane | 30  |\n" +
				"| Bob  | 4   |\n",
		},
		{
			name:  "quoted newline and pipe escaped",
			input: "a,b\n\"l1\nl2\",p|q\n",
			want: "| a        | b    |\n" +
				"| -------- | ---- |\n" +
				"| l1<br>l2 | p\\|q |\n",
		},
		{
			name:  "crlf inside a quoted field is one line break",
			input: "a\r\n\"l1\r\nl2\"\r\n",
			want: "| a        |\n" +
				"| -------- |\n" +
				"| l1<br>l2 |\n",
		},
		{
			name:  "crlf line endings",
			input: "a,b\r\n1,2\r\n",
			want: "| a   | b   |\n" +
				"| --- | --- |\n" +
				"| 1   | 2   |\n",
		},
		{
			name:  "header row only",
			input: "a,b\n",
			want: "| a   | b   |\n" +
				"| --- | --- |\n",
		},
		{
			name:  "leading byte order mark skipped",
			input: "\xef\xbb\xbfname,age\r\nJane,30\r\n",
			want: "| name | age |\n" +
				"| ---- | --- |\n" +
				"| Jane | 30  |\n",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "byte order mark only is empty input", input: "\xef\xbb\xbf", wantErr: "empty input"},
		{name: "ragged row", input: "a,b\n1", wantErr: "invalid CSV at line 2: record has 1 field but the header row has 2"},
		{name: "bare carriage return", input: "a\r1\r", wantErr: "invalid CSV at line 1, column 2: carriage return without a line feed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := MarkdownTableCSV(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("MarkdownTableCSV() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("MarkdownTableCSV() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("MarkdownTableCSV() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("MarkdownTableCSV() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestMarkdownTableReadFailure(t *testing.T) {
	readErr := errors.New("disk on fire")
	for name, render := range map[string]func(io.Writer, io.Reader) error{"json": MarkdownTable, "csv": MarkdownTableCSV} {
		t.Run(name, func(t *testing.T) {
			if err := render(io.Discard, iotest.ErrReader(readErr)); !errors.Is(err, readErr) {
				t.Fatalf("error = %v, want it to wrap %v", err, readErr)
			}
		})
	}
}

func TestMarkdownTableRejectsNonUTF8(t *testing.T) {
	tests := []struct {
		name    string
		render  func(io.Writer, io.Reader) error
		input   string
		wantErr string
	}{
		{name: "json invalid byte", render: MarkdownTable, input: "[{\"a\": \"x\xffy\"}]", wantErr: "invalid UTF-8 at line 1, column 10"},
		{name: "csv invalid byte", render: MarkdownTableCSV, input: "a\nx\xffy\n", wantErr: "invalid UTF-8 at line 2, column 2"},
		{name: "json UTF-16", render: MarkdownTable, input: "\xff\xfe[\x00]\x00", wantErr: "this is UTF-16LE"},
		{name: "csv UTF-16", render: MarkdownTableCSV, input: "\xff\xfea\x00\n\x00", wantErr: "this is UTF-16LE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := tt.render(&out, strings.NewReader(tt.input))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
			if out.Len() != 0 {
				t.Errorf("wrote %q, want no output", out.String())
			}
		})
	}
}

func TestMarkdownTableOutputIsBounded(t *testing.T) {
	keys := make([]string, 8000)
	for i := range keys {
		keys[i] = fmt.Sprintf(`{"%x":0}`, i)
	}
	tests := []struct {
		name    string
		render  func(io.Writer, io.Reader) error
		input   string
		maxOut  int
		wantErr string
	}{
		{
			// One huge cell used to pad every other row in its column to
			// its width: 120 KB of input produced 1 GB of output.
			name:   "one wide cell does not pad its column",
			render: MarkdownTableCSV,
			input:  "a\n" + strings.Repeat("x", 100_000) + "\n" + strings.Repeat("y\n", 10_000),
			maxOut: 2 * 120_003,
		},
		{
			// Padding alone would take this valid table to 83 MB, past
			// both 64 MiB and 64 times its 1 MB of input.
			name:   "padding that would pass the limit is dropped",
			render: MarkdownTableCSV,
			input:  strings.Repeat("h"+strings.Repeat("x", 79)+",", 19) + strings.Repeat("x", 80) + "\n" + strings.Repeat(strings.Repeat(",", 19)+"\n", 50_000),
			maxOut: 4 << 20,
		},
		{
			name:    "short rows under a wide header",
			render:  MarkdownTable,
			input:   "[[" + strings.Repeat(`"",`, 7999) + `""]` + strings.Repeat(",[]", 8000) + "]",
			wantErr: "the table would be more than 67108864 bytes even without padding (8000 columns by 2793 rows so far, from 48003 bytes of input;",
		},
		{
			name:    "every object adds a column",
			render:  MarkdownTable,
			input:   "[" + strings.Join(keys, ",") + "]",
			wantErr: "the table would be more than 67108864 bytes even without padding",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := tt.render(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				if out.Len() != 0 {
					t.Errorf("wrote %d bytes before failing", out.Len())
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if out.Len() > tt.maxOut {
				t.Errorf("output is %d bytes for %d bytes of input, want at most %d", out.Len(), len(tt.input), tt.maxOut)
			}
		})
	}
}

// The output limit is only as good as the size it checks, so the computed
// size must match what render writes.
func TestTableSizeIsExact(t *testing.T) {
	jsonInputs := []string{
		`[{"zebra":"1","alpha":"2"}]`,
		`[{"name":"Jane","age":30},{"name":"Bob","city":"Oslo","age":4}]`,
		`[{},{"a":"v"},{}]`,
		`[["a","b","c"],["1"],[],["x","y","z"]]`,
		`[["h"]]`,
		`[{"a":"x|y\nz","é":"ü"}]`,
		`[["a","b"],["` + strings.Repeat("x", 80) + `","` + strings.Repeat("y", 81) + `"]]`,
		`[["` + strings.Repeat("é", 81) + `"],["` + strings.Repeat("日", 3) + `"]]`,
	}
	csvInputs := []string{
		"a,b\n1,2\n",
		"a\n\"x\r\ny\"\n",
		"a|b,c\n|,\n",
		"h\n" + strings.Repeat("é", 90) + "\n\n",
	}
	check := func(t *testing.T, tbl *table) {
		t.Helper()
		for _, pad := range []bool{true, false} {
			var out bytes.Buffer
			if err := tbl.write(&out, pad); err != nil {
				t.Fatalf("write() error = %v", err)
			}
			if got := tbl.size(pad, math.MaxInt64); got != int64(out.Len()) {
				t.Errorf("size(pad=%t) = %d, write wrote %d bytes:\n%s", pad, got, out.Len(), out.String())
			}
		}
	}
	for _, input := range jsonInputs {
		t.Run(input, func(t *testing.T) {
			tbl, err := parseJSONTable([]byte(input), len(input))
			if err != nil {
				t.Fatalf("parseJSONTable() error = %v", err)
			}
			check(t, tbl)
		})
	}
	for _, input := range csvInputs {
		t.Run(input, func(t *testing.T) {
			tbl, err := parseCSVTable(strings.NewReader(input))
			if err != nil {
				t.Fatalf("parseCSVTable() error = %v", err)
			}
			check(t, tbl)
		})
	}
}

func TestMarkdownTableWriteFailure(t *testing.T) {
	writeErr := errors.New("pipe closed")
	err := MarkdownTable(failingWriter{writeErr}, strings.NewReader(`[{"a":"1"}]`))
	if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "writing output") {
		t.Fatalf("error = %v, want a writing output error wrapping %v", err, writeErr)
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }
