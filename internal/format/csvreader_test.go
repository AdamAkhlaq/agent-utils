package format

import (
	"encoding/csv"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf8"
)

func readAllCSV(input string, sep rune) ([][]string, error) {
	cr, err := NewCSVReader(strings.NewReader(input), sep)
	if err != nil {
		return nil, err
	}
	var records [][]string
	for {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return records, nil
		}
		if err != nil {
			return records, err
		}
		records = append(records, record)
	}
}

func TestCSVReader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		sep     rune
		want    [][]string
		wantErr string
	}{
		{name: "lf records", input: "a,b\n1,2\n", want: [][]string{{"a", "b"}, {"1", "2"}}},
		{name: "crlf records", input: "a,b\r\n1,2\r\n", want: [][]string{{"a", "b"}, {"1", "2"}}},
		{name: "no final line break", input: "a,b\n1,2", want: [][]string{{"a", "b"}, {"1", "2"}}},
		{name: "crlf inside quotes kept", input: "a\r\n\"x\r\ny\"\r\n", want: [][]string{{"a"}, {"x\r\ny"}}},
		{name: "lf inside quotes kept", input: "a\n\"x\ny\"\n", want: [][]string{{"a"}, {"x\ny"}}},
		{name: "lone cr inside quotes kept", input: "a\n\"x\ry\"\n", want: [][]string{{"a"}, {"x\ry"}}},
		{name: "cr before closing quote kept", input: "a\n\"x\r\"\n", want: [][]string{{"a"}, {"x\r"}}},
		{name: "blank line inside quotes kept", input: "a\n\"x\n\ny\"\n", want: [][]string{{"a"}, {"x\n\ny"}}},
		{name: "doubled quotes", input: "a\n\"say \"\"hi\"\"\"\n", want: [][]string{{"a"}, {`say "hi"`}}},
		{name: "separator inside quotes", input: "a,b\n\"1,5\",2\n", want: [][]string{{"a", "b"}, {"1,5", "2"}}},
		{name: "empty fields", input: "a,b,c\n,\"\",\n", want: [][]string{{"a", "b", "c"}, {"", "", ""}}},
		{name: "trailing separator", input: "a,b\n1,", want: [][]string{{"a", "b"}, {"1", ""}}},
		{name: "blank lines skipped", input: "\na\n\n1\r\n\r\n2\n\n", want: [][]string{{"a"}, {"1"}, {"2"}}},
		{name: "whitespace is data", input: " a , b \n 1 , 2 \n", want: [][]string{{" a ", " b "}, {" 1 ", " 2 "}}},
		{name: "tab separator", input: "a\tb\n1\t2\n", sep: '\t', want: [][]string{{"a", "b"}, {"1", "2"}}},
		{name: "multibyte separator", input: "a§b\n1§\"2§3\"\n", sep: '§', want: [][]string{{"a", "b"}, {"1", "2§3"}}},
		{name: "empty input", input: "", want: nil},
		{name: "only line breaks", input: "\n\r\n\n", want: nil},
		{name: "blank crlf line skipped", input: "a\n\r\n1\n", want: [][]string{{"a"}, {"1"}}},
		{
			name:    "bare quote",
			input:   "a\nx\"y\n",
			wantErr: `invalid CSV at line 2, column 2: bare " in an unquoted field`,
		},
		{
			name:    "text after closing quote",
			input:   "a\n\"x\"y\n",
			wantErr: `invalid CSV at line 2, column 4: unexpected 'y' after a closing quote`,
		},
		{
			name:    "space after closing quote",
			input:   "a,b\n\"x\" ,2\n",
			wantErr: `invalid CSV at line 2, column 4: unexpected ' ' after a closing quote`,
		},
		{
			name:    "unclosed quote reports where it opens",
			input:   "a,b\n1,\"open\nmore\n",
			wantErr: "invalid CSV at line 2, column 3: quoted field is never closed",
		},
		{
			name:    "bare cr in a field",
			input:   "a\nx\ry\n",
			wantErr: "invalid CSV at line 2, column 2: carriage return without a line feed outside quotes",
		},
		{
			name:    "classic mac line endings",
			input:   "a\r1\r",
			wantErr: "invalid CSV at line 1, column 2: carriage return without a line feed",
		},
		{
			name:    "cr at the end of the input",
			input:   "a\nx\r",
			wantErr: "invalid CSV at line 2, column 2: carriage return without a line feed",
		},
		{
			name:    "cr after closing quote",
			input:   "a\n\"x\"\ry\n",
			wantErr: "invalid CSV at line 2, column 4: carriage return without a line feed",
		},
		{
			name:    "record with too few fields",
			input:   "a,b\n1\n",
			wantErr: "invalid CSV at line 2: record has 1 field but the header row has 2",
		},
		{
			name:    "record with too many fields after a multi-line field",
			input:   "a,b\n\"x\ny\",2\n1,2,3\n",
			wantErr: "invalid CSV at line 4: record has more than the header row's 2 fields",
		},
		{name: "quote as separator", input: "a\n", sep: '"', wantErr: `invalid CSV separator '"'`},
		{name: "line feed as separator", input: "a\n", sep: '\n', wantErr: `invalid CSV separator '\n'`},
		{name: "carriage return as separator", input: "a\n", sep: '\r', wantErr: `invalid CSV separator '\r'`},
		{name: "replacement character as separator", input: "a\n", sep: utf8.RuneError, wantErr: "invalid CSV separator '" + string(utf8.RuneError) + "'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sep := tt.sep
			if sep == 0 {
				sep = ','
			}
			got, err := readAllCSV(tt.input, sep)
			if tt.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to start with %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("records = %q, want %q", got, tt.want)
			}
		})
	}
	t.Run("NUL as separator", func(t *testing.T) {
		if _, err := NewCSVReader(strings.NewReader("a\n"), 0); err == nil || err.Error() != `invalid CSV separator '\x00'` {
			t.Fatalf("error = %v, want the invalid separator error", err)
		}
	})
	t.Run("read failure", func(t *testing.T) {
		_, err := NewCSVReader(iotest.ErrReader(io.ErrUnexpectedEOF), ',')
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("error = %v, want it to wrap %v", err, io.ErrUnexpectedEOF)
		}
	})
}

// FuzzCSVReader checks CSVReader against encoding/csv, the reader it
// replaces. Wherever both succeed they must agree once CRLF inside fields is
// normalized to LF, the one change encoding/csv makes to the data, and a
// leading byte order mark is trimmed, which encoding/csv keeps. Each must
// fail wherever the other does, except that a CR outside quotes not followed
// by LF is an error here and data (or, at the very end, dropped) there.
func FuzzCSVReader(f *testing.F) {
	seeds := []string{
		"a,b\n1,2\n",
		"a,b\r\n\"x\r\ny\",2\r\n",
		"a\n\"say \"\"hi\"\"\"\n",
		"a,b\n\"1,5\",\n",
		"\na\n\n1\r\n\r\n2\n",
		"a\nx\"y\n",
		"a\n\"x\"y\n",
		"a\n\"open\n",
		"a\r1\r",
		"a,b\n1\n",
		"a;b\n1;\"2;3\"\n",
		"a\tb\n\"1\t\"\t2\n",
		"a§b\n1§2\n",
		"\ufeff\"a,b\",c\n1,2\n",
	}
	for _, s := range seeds {
		f.Add(s, uint8(0))
	}
	separators := []rune{',', ';', '\t', '|', ' ', 'x', '§', '€', '😀'}
	f.Fuzz(func(t *testing.T, input string, sepIndex uint8) {
		if !utf8.ValidString(input) {
			t.Skip("encoding/csv decodes runes and CSVReader compares bytes, which agree only on valid UTF-8")
		}
		sep := separators[int(sepIndex)%len(separators)]
		got, err := readAllCSV(input, sep)

		std := csv.NewReader(strings.NewReader(strings.TrimPrefix(input, "\ufeff")))
		std.Comma = sep
		want, stdErr := std.ReadAll()

		bareCR := err != nil && strings.Contains(err.Error(), "carriage return without a line feed")
		switch {
		case err == nil && stdErr != nil:
			t.Fatalf("CSVReader accepted input encoding/csv rejects (%v): %q", stdErr, got)
		case err != nil && stdErr == nil && !bareCR:
			t.Fatalf("CSVReader rejected input encoding/csv accepts: %v", err)
		case err == nil:
			var normalized [][]string
			for _, record := range got {
				fields := make([]string, len(record))
				for j, field := range record {
					fields[j] = strings.ReplaceAll(field, "\r\n", "\n")
				}
				normalized = append(normalized, fields)
			}
			if len(want) == 0 {
				want = nil
			}
			if !reflect.DeepEqual(normalized, want) {
				t.Fatalf("records = %q, encoding/csv read %q", got, want)
			}
		}
	})
}
