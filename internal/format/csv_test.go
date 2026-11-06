package format

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestCSVToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		sep     rune
		want    string
		wantErr string
	}{
		{
			name:  "column order preserved",
			input: "zebra,alpha\n1,2\n3,4",
			want:  "[\n  {\n    \"zebra\": \"1\",\n    \"alpha\": \"2\"\n  },\n  {\n    \"zebra\": \"3\",\n    \"alpha\": \"4\"\n  }\n]",
		},
		{
			name:  "values stay strings",
			input: "zip,count\n01234,42",
			want:  "[\n  {\n    \"zip\": \"01234\",\n    \"count\": \"42\"\n  }\n]",
		},
		{
			name:  "quoted field with comma newline and quote",
			input: "name,note\n\"Doe, Jane\",\"line \"\"one\"\"\nline two\"",
			want:  "[\n  {\n    \"name\": \"Doe, Jane\",\n    \"note\": \"line \\\"one\\\"\\nline two\"\n  }\n]",
		},
		{
			name:  "crlf inside a quoted field kept",
			input: "id,note\r\n1,\"line one\r\nline two\"\r\n",
			want:  "[\n  {\n    \"id\": \"1\",\n    \"note\": \"line one\\r\\nline two\"\n  }\n]",
		},
		{
			name:  "html characters written as is",
			input: "<k>\n<b>&amp;</b>\u2028",
			want:  "[\n  {\n    \"<k>\": \"<b>&amp;</b>\\u2028\"\n  }\n]",
		},
		{
			name:  "crlf line endings",
			input: "a,b\r\n1,2\r\n",
			want:  "[\n  {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  }\n]",
		},
		{
			name:  "semicolon separator",
			input: "a;b\n1;2",
			sep:   ';',
			want:  "[\n  {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  }\n]",
		},
		{
			name:  "tab separator",
			input: "a\tb\n1\t2",
			sep:   '\t',
			want:  "[\n  {\n    \"a\": \"1\",\n    \"b\": \"2\"\n  }\n]",
		},
		{
			name:  "header row only",
			input: "a,b\n",
			want:  "[]",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "duplicate headers", input: "a,a\n1,2", wantErr: `duplicate header "a"`},
		{name: "ragged row", input: "a,b\n1", wantErr: "invalid CSV at line 2: record has 1 field but the header row has 2"},
		{name: "bare carriage return", input: "a\r1\r", wantErr: "invalid CSV at line 1, column 2: carriage return without a line feed outside quotes"},
		{name: "quote as separator", input: "a\n", sep: '"', wantErr: `invalid CSV separator '"'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sep := tt.sep
			if sep == 0 {
				sep = ','
			}
			var out bytes.Buffer
			err := CSVToJSON(&out, strings.NewReader(tt.input), sep)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("CSVToJSON() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CSVToJSON() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CSVToJSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("CSVToJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJSONToCSV(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		sep     rune
		want    string
		wantErr string
	}{
		{
			name:  "key order preserved",
			input: `[{"zebra":"1","alpha":"2"},{"zebra":"3","alpha":"4"}]`,
			want:  "zebra,alpha\n1,2\n3,4\n",
		},
		{
			name:  "union of keys in first-seen order",
			input: `[{"a":"1"},{"b":"2","a":"3"},{"c":"4"}]`,
			want:  "a,b,c\n1,,\n3,2,\n,,4\n",
		},
		{
			name:  "missing keys become empty cells",
			input: `[{"a":"1","b":"2"},{"a":"3"}]`,
			want:  "a,b\n1,2\n3,\n",
		},
		{
			name:  "null becomes empty cell",
			input: `[{"a":null,"b":"x"}]`,
			want:  "a,b\n,x\n",
		},
		{
			name:  "numbers keep source form",
			input: `[{"price":1.10,"big":9007199254740993,"exp":1e3}]`,
			want:  "price,big,exp\n1.10,9007199254740993,1e3\n",
		},
		{
			name:  "booleans",
			input: `[{"yes":true,"no":false}]`,
			want:  "yes,no\ntrue,false\n",
		},
		{
			name:  "strings needing quoting",
			input: `[{"name":"Doe, Jane","note":"line \"one\"\nline two"}]`,
			want:  "name,note\n\"Doe, Jane\",\"line \"\"one\"\"\nline two\"\n",
		},
		{
			name:  "semicolon separator",
			input: `[{"a":"1;x","b":"2"}]`,
			sep:   ';',
			want:  "a;b\n\"1;x\";2\n",
		},
		{
			name:  "tab separator",
			input: `[{"a":"1","b":"2"}]`,
			sep:   '\t',
			want:  "a\tb\n1\t2\n",
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  "",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n", wantErr: "empty input"},
		{name: "top-level object", input: `{"a":1}`, wantErr: "input must be a JSON array of objects, got an object"},
		{name: "top-level string", input: `"hi"`, wantErr: "input must be a JSON array of objects, got a string"},
		{name: "element not an object", input: `[{"a":"1"},"x"]`, wantErr: "array element 2 must be an object, got a string"},
		{name: "nested object value", input: `[{"a":"1"},{"addr":{"city":"Oslo"}}]`, wantErr: `array element 2, key "addr": nested objects are not supported`},
		{name: "nested array value", input: `[{"tags":["x"]}]`, wantErr: `array element 1, key "tags": nested arrays are not supported`},
		{name: "duplicate key", input: `[{"a":"1","a":"2"}]`, wantErr: `array element 1: duplicate key "a"`},
		{name: "lone surrogate", input: `[{"a":"1"},{"b":"x\udc00"}]`, wantErr: `string at line 1, column 17 escapes a lone UTF-16 surrogate (\udc00), which has no UTF-8 form`},
		{name: "lone surrogate in a key", input: `[{"\ud800":"1"}]`, wantErr: `string at line 1, column 3 escapes a lone UTF-16 surrogate (\ud800)`},
		{name: "duplicate key reported before a lone surrogate", input: `[{"a":"\udc00","a":1}]`, wantErr: `array element 1: duplicate key "a"`},
		{name: "syntax error after a lone surrogate", input: `[{"a":"\udc00"`, wantErr: `invalid JSON`},
		{name: "objects without keys", input: `[{},{}]`, wantErr: "no columns to write"},
		{name: "trailing data", input: `[] "extra"`, wantErr: "unexpected data after the JSON array"},
		{name: "quote separator", input: `[{"a":"1"}]`, sep: '"', wantErr: "invalid CSV separator"},
		{name: "newline separator", input: `[{"a":"1"}]`, sep: '\n', wantErr: "invalid CSV separator"},
		{name: "first column missing", input: `[{"a":"x"},{"b":"y"}]`, want: "a,b\nx,\n,y\n"},
		{name: "one column, missing value", input: `[{"a":"x"},{}]`, want: "a\nx\n\"\"\n"},
		{name: "invalid JSON", input: `[{"a":}]`, wantErr: "invalid JSON"},
		{name: "truncated input", input: `[{"a":"1"}`, wantErr: "invalid JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sep := tt.sep
			if sep == 0 {
				sep = ','
			}
			var out bytes.Buffer
			err := JSONToCSV(&out, strings.NewReader(tt.input), sep)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("JSONToCSV() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("JSONToCSV() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("JSONToCSV() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("JSONToCSV() = %q, want %q", got, tt.want)
			}
			if out.Len() > 0 {
				checkCSVTableSize(t, tt.input, sep, out.Len())
			}
		})
	}
}

func TestJSONToCSVQuotesFieldsReadersWouldLose(t *testing.T) {
	for _, tt := range []struct{ json, csv string }{
		{`[{"a":""},{"a":"x"},{"a":null}]`, "a\n\"\"\nx\n\"\"\n"},
		{`[{"":"x"}]`, "\"\"\nx\n"},
		{`[{"\ufeffa":"1","b":"2"}]`, "\"\ufeffa\",b\n1,2\n"},
		{`[{"\ufeffsay \"hi\"":"1"}]`, "\"\ufeffsay \"\"hi\"\"\"\n1\n"},
		{`[{"\ufeffa":"","b":"x y"}]`, "\"\ufeffa\",b\n,x y\n"},
	} {
		var out bytes.Buffer
		if err := JSONToCSV(&out, strings.NewReader(tt.json), ','); err != nil {
			t.Fatalf("JSONToCSV(%s) error = %v", tt.json, err)
		}
		if out.String() != tt.csv {
			t.Errorf("JSONToCSV(%s) = %q, want %q", tt.json, out.String(), tt.csv)
		}
		var back bytes.Buffer
		if err := CSVToJSON(&back, &out, ','); err != nil {
			t.Fatalf("CSVToJSON() error = %v", err)
		}
		var rows []map[string]string
		if err := json.Unmarshal(back.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		var want []map[string]*string
		if err := json.Unmarshal([]byte(tt.json), &want); err != nil {
			t.Fatal(err)
		}
		wantRows := make([]map[string]string, len(want))
		for i, row := range want {
			wantRows[i] = map[string]string{}
			for k, v := range row {
				if v != nil {
					wantRows[i][k] = *v
				} else {
					wantRows[i][k] = ""
				}
			}
		}
		if !reflect.DeepEqual(rows, wantRows) {
			t.Errorf("round trip of %s = %q, want %q", tt.json, rows, wantRows)
		}
	}
}

func TestJSONToCSVRoundTripsWithCSVToJSON(t *testing.T) {
	csvIn := "name,note\n\"Doe, Jane\",\"line \"\"one\"\"\nline two\"\nBob,plain\nAda,\"crlf\r\nkept\"\n"
	var jsonOut bytes.Buffer
	if err := CSVToJSON(&jsonOut, strings.NewReader(csvIn), ','); err != nil {
		t.Fatalf("CSVToJSON() error = %v", err)
	}
	var csvOut bytes.Buffer
	if err := JSONToCSV(&csvOut, &jsonOut, ','); err != nil {
		t.Fatalf("JSONToCSV() error = %v", err)
	}
	if got := csvOut.String(); got != csvIn {
		t.Errorf("round trip = %q, want %q", got, csvIn)
	}
}

func TestCSVToJSONOutputIsValidJSON(t *testing.T) {
	input := "name,city\n\"Doe, Jane\",Oslo\nBob,\"New\nYork\""
	var out bytes.Buffer
	if err := CSVToJSON(&out, strings.NewReader(input), ','); err != nil {
		t.Fatalf("CSVToJSON() error = %v", err)
	}
	if err := JSONValid(&out); err != nil {
		t.Errorf("CSVToJSON() output is not valid JSON: %v", err)
	}
}

// wideCSV is a header of cols names, each length bytes long, and rows rows of
// empty cells: one byte of input per cell, and a whole header name of output.
func wideCSV(cols, length, rows int) string {
	names := make([]string, cols)
	for i := range names {
		names[i] = fmt.Sprintf("%0*d", length, i)
	}
	return strings.Join(names, ",") + strings.Repeat("\n"+strings.Repeat(",", cols-1), rows)
}

// TestCSVToJSONOutputLimit checks the limit at its edge, and that nothing is
// written past it.
func TestCSVToJSONOutputLimit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "long header names", input: wideCSV(10, 100, 200), wantErr: "200 rows each repeat all 10 header names as JSON keys, 1020 bytes of them per row; split the rows across several files, each with the header row"},
		{name: "many columns", input: wideCSV(1000, 100, 150), wantErr: "150 rows each repeat all 1000 header names as JSON keys, 102000 bytes of them per row"},
		{name: "escaped header name", input: strings.Repeat("\x01", 100) + strings.Repeat("\n\"\"", 300), wantErr: "300 rows each repeat the header name as a JSON key, 602 bytes per row"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var want bytes.Buffer
			if err := CSVToJSON(&want, strings.NewReader(tt.input), ','); err != nil {
				t.Fatal(err)
			}
			if 64*len(tt.input) >= want.Len() {
				t.Fatalf("output is %d bytes, not over 64 times the %d-byte input", want.Len(), len(tt.input))
			}
			var out bytes.Buffer
			if err := csvToJSON(&out, strings.NewReader(tt.input), ',', want.Len()); err != nil || out.String() != want.String() {
				t.Fatalf("at a floor of exactly the output size: error = %v, output equal %v", err, out.String() == want.String())
			}
			out.Reset()
			err := csvToJSON(&out, strings.NewReader(tt.input), ',', want.Len()-1)
			if !errors.Is(err, ErrOutputLimit) || !strings.Contains(err.Error(), fmt.Sprintf("JSON output would exceed %d bytes (the larger of %[1]d bytes and 64 times the input size): %s", want.Len()-1, tt.wantErr)) {
				t.Fatalf("one byte below: error = %v, want the output limit", err)
			}
			if out.Len() != 0 {
				t.Errorf("one byte below: wrote %d bytes before failing", out.Len())
			}
		})
	}
}

// TestCSVToJSONDataErrorsFirst checks that a malformed record is reported
// even when it comes after the output has passed its limit.
func TestCSVToJSONDataErrorsFirst(t *testing.T) {
	input := wideCSV(100, 1000, 2000) + "\n,,"
	err := CSVToJSON(&bytes.Buffer{}, strings.NewReader(input), ',')
	if err == nil || !strings.Contains(err.Error(), "invalid CSV at line 2002: record has 3 fields but the header row has 100") {
		t.Fatalf("CSVToJSON() error = %v, want the malformed record", err)
	}
}

func TestCSVToJSONWriteError(t *testing.T) {
	err := CSVToJSON(failWriter{}, strings.NewReader("a,b\n1,2"), ',')
	if err == nil || err.Error() != "writing output: disk full" {
		t.Fatalf("CSVToJSON() error = %v, want the write error", err)
	}
}

// FuzzCSVToJSON checks that a failed conversion writes nothing, that the
// output is valid JSON holding exactly the records CSVReader reads, and that
// the count measures exactly what is written: at a floor of the output size
// the conversion passes, and one byte below it fails without writing.
func FuzzCSVToJSON(f *testing.F) {
	for _, s := range []string{"a,b\n1,2\n", "a,b\r\n\"x\r\ny\",<&>\n", "a\n\"say \"\"hi\"\"\"\n", "a,a\n1,2", "a\n\"open\n", "a,b\n1\n",
		"\ufeffa\n\u2028\n", "a\tb\n\x01\t\x7f\n", strings.Repeat("<", 100) + strings.Repeat("\n\"\"", 300)} {
		f.Add(s, uint8(0))
	}
	f.Add(strings.Repeat("k", 300)+","+strings.Repeat("j", 300)+strings.Repeat("\n,", 50), uint8(0))
	separators := []rune{',', '\t', ';', '\u00e9', '\u65e5', '\U0001f600'}
	f.Fuzz(func(t *testing.T, input string, sepIndex uint8) {
		sep := separators[int(sepIndex)%len(separators)]
		var out bytes.Buffer
		if err := CSVToJSON(&out, strings.NewReader(input), sep); err != nil {
			if out.Len() != 0 {
				t.Fatalf("CSVToJSON() wrote %d bytes before failing: %v", out.Len(), err)
			}
			return
		}
		records, err := readAllCSV(input, sep)
		if err != nil {
			t.Fatal(err)
		}
		var rows []map[string]string
		if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
			t.Fatalf("output is not a JSON array of objects: %v\n%s", err, out.String())
		}
		if len(rows) != len(records)-1 {
			t.Fatalf("output has %d rows, input has %d", len(rows), len(records)-1)
		}
		for i, row := range rows {
			for j, h := range records[0] {
				if row[h] != records[i+1][j] {
					t.Fatalf("row %d, column %q = %q, want %q", i, h, row[h], records[i+1][j])
				}
			}
		}
		var edge bytes.Buffer
		if err := csvToJSON(&edge, strings.NewReader(input), sep, out.Len()); err != nil || edge.String() != out.String() {
			t.Fatalf("at a floor of exactly the output size: error = %v, output equal %v", err, edge.String() == out.String())
		}
		if 64*len(strings.TrimPrefix(input, "\ufeff")) >= out.Len() {
			return
		}
		edge.Reset()
		if err := csvToJSON(&edge, strings.NewReader(input), sep, out.Len()-1); !errors.Is(err, ErrOutputLimit) || edge.Len() != 0 {
			t.Fatalf("one byte below: error = %v, wrote %d bytes", err, edge.Len())
		}
	})
}

// checkCSVTableSize checks that tableSize computes exactly the size of the
// CSV that JSONToCSV wrote for input.
func checkCSVTableSize(t *testing.T, input string, sep rune, written int) {
	t.Helper()
	data, err := ReadText(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	table, err := decodeJSONTable(data)
	if err != nil {
		t.Fatal(err)
	}
	cw := &csvWriter{sep: sep, sepStr: string(sep)}
	if size := cw.tableSize(table, math.MaxInt64); size != int64(written) {
		t.Fatalf("tableSize() = %d, JSONToCSV wrote %d bytes", size, written)
	}
}

// distinctKeysJSON is an array of n objects, each with its own key: about 11
// bytes of input per object, and n cells per row of output.
func distinctKeysJSON(n int) string {
	objects := make([]string, n)
	for i := range objects {
		objects[i] = fmt.Sprintf(`{"k%d":1}`, i)
	}
	return "[" + strings.Join(objects, ",") + "]"
}

// TestJSONToCSVOutputLimit checks the limit at its edge, and that nothing is
// written past it.
func TestJSONToCSVOutputLimit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		sep     rune
		wantErr string
	}{
		{name: "distinct keys", input: distinctKeysJSON(1000), sep: ',', wantErr: "1000 rows each have a cell, even an empty one, in all 1000 columns (one per distinct key across the objects); convert objects with different keys separately"},
		{name: "multibyte separator", input: distinctKeysJSON(300), sep: '\U0001f600', wantErr: "300 rows each have a cell, even an empty one, in all 300 columns"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var want bytes.Buffer
			if err := JSONToCSV(&want, strings.NewReader(tt.input), tt.sep); err != nil {
				t.Fatal(err)
			}
			if 64*len(tt.input) >= want.Len() {
				t.Fatalf("output is %d bytes, not over 64 times the %d-byte input", want.Len(), len(tt.input))
			}
			var out bytes.Buffer
			if err := jsonToCSV(&out, strings.NewReader(tt.input), tt.sep, want.Len()); err != nil || out.String() != want.String() {
				t.Fatalf("at a floor of exactly the output size: error = %v, output equal %v", err, out.String() == want.String())
			}
			out.Reset()
			err := jsonToCSV(&out, strings.NewReader(tt.input), tt.sep, want.Len()-1)
			if !errors.Is(err, ErrOutputLimit) || !strings.Contains(err.Error(), fmt.Sprintf("CSV output would exceed %d bytes (the larger of %[1]d bytes and 64 times the input size): %s", want.Len()-1, tt.wantErr)) {
				t.Fatalf("one byte below: error = %v, want the output limit", err)
			}
			if out.Len() != 0 {
				t.Errorf("one byte below: wrote %d bytes before failing", out.Len())
			}
		})
	}
}

// referenceCSVRecord is record as json2csv wrote it before it counted its
// output: through encoding/csv's Writer, with the first field quoted by hand
// where a reader would otherwise lose it.
func referenceCSVRecord(record []string, sep rune) string {
	var b strings.Builder
	cw := csv.NewWriter(&b)
	cw.Comma = sep
	first := record[0]
	if (len(record) > 1 || first != "") && !strings.HasPrefix(first, "\ufeff") {
		_ = cw.Write(record)
		cw.Flush()
		return b.String()
	}
	b.WriteString(`"` + strings.ReplaceAll(first, `"`, `""`) + `"`)
	if len(record) == 1 {
		return b.String() + "\n"
	}
	b.WriteRune(sep)
	_ = cw.Write(record[1:])
	cw.Flush()
	return b.String()
}

var csvSeparators = []rune{',', '\t', ';', '\u00e9', '\u65e5', '\U0001f600'}

// FuzzCSVRecord holds csvWriter's record and recordSize to
// referenceCSVRecord, on records of fields split at NUL.
func FuzzCSVRecord(f *testing.F) {
	for _, s := range []string{"a\x00b", "", "\x00", `\.`, ` \.`, " a", "\u00a0x\x00y", "\ufeffa\x00b", "\ufeff", `say "hi"`, "x\r\ny\x00\r", "a;b\x00c", "\u65e5\x00\u672c", "\xff\x00\xfe", "\x00\x00"} {
		f.Add(s, uint8(0))
		f.Add(s, uint8(5))
	}
	f.Fuzz(func(t *testing.T, joined string, sepIndex uint8) {
		record := strings.Split(joined, "\x00")
		sep := csvSeparators[int(sepIndex)%len(csvSeparators)]
		want := referenceCSVRecord(record, sep)
		var buf bytes.Buffer
		out := bufio.NewWriter(&buf)
		cw := &csvWriter{out: out, sep: sep, sepStr: string(sep)}
		cw.record(record)
		if err := out.Flush(); err != nil {
			t.Fatal(err)
		}
		if buf.String() != want {
			t.Fatalf("record(%q) = %q, want %q", record, buf.String(), want)
		}
		if size := cw.recordSize(record); size != int64(len(want)) {
			t.Fatalf("recordSize(%q) = %d, wrote %d", record, size, len(want))
		}
	})
}

// FuzzJSONToCSV checks that every value survives a round trip through
// csv2json, and the output limit at its edge.
func FuzzJSONToCSV(f *testing.F) {
	for _, s := range []string{`[{"a":"1","b":null}]`, `[{"a":""},{"a":"x"},{"a":null}]`, `[{"\ufeffa":"1","b":"2"}]`, `[{"k":"\\."},{"j":" lead"}]`,
		`[{"q":"say \"hi\"","n":"x\r\ny"}]`, `[{"n":1.10,"b":true}]`, `[{"a":1,"a":2}]`, `[{"a":{"b":1}}]`, `[{}]`, `[]`, distinctKeysJSON(40)} {
		f.Add(s, uint8(0))
	}
	f.Fuzz(func(t *testing.T, input string, sepIndex uint8) {
		sep := csvSeparators[int(sepIndex)%len(csvSeparators)]
		var out bytes.Buffer
		if err := JSONToCSV(&out, strings.NewReader(input), sep); err != nil {
			if out.Len() != 0 {
				t.Fatalf("JSONToCSV() wrote %d bytes before failing: %v", out.Len(), err)
			}
			return
		}
		if out.Len() == 0 {
			return
		}
		dec := json.NewDecoder(strings.NewReader(strings.TrimPrefix(input, "\ufeff")))
		dec.UseNumber()
		var objects []map[string]any
		if err := dec.Decode(&objects); err != nil {
			t.Fatal(err)
		}
		var back bytes.Buffer
		if err := CSVToJSON(&back, bytes.NewReader(out.Bytes()), sep); err != nil {
			t.Fatalf("CSVToJSON() of %q: %v", out.String(), err)
		}
		var rows []map[string]string
		if err := json.Unmarshal(back.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != len(objects) {
			t.Fatalf("%d rows came back, want %d", len(rows), len(objects))
		}
		for i, row := range rows {
			for key, value := range objects[i] {
				if _, ok := row[key]; !ok {
					t.Fatalf("row %d has no column %q", i, key)
				}
				want := ""
				if value != nil {
					want = fmt.Sprint(value)
				}
				if row[key] != want {
					t.Fatalf("row %d, column %q = %q, want %q", i, key, row[key], want)
				}
			}
		}
		checkCSVTableSize(t, input, sep, out.Len())
		var edge bytes.Buffer
		if err := jsonToCSV(&edge, strings.NewReader(input), sep, out.Len()); err != nil || edge.String() != out.String() {
			t.Fatalf("at a floor of exactly the output size: error = %v, output equal %v", err, edge.String() == out.String())
		}
		if 64*len(strings.TrimPrefix(input, "\ufeff")) >= out.Len() {
			return
		}
		edge.Reset()
		if err := jsonToCSV(&edge, strings.NewReader(input), sep, out.Len()-1); !errors.Is(err, ErrOutputLimit) || edge.Len() != 0 {
			t.Fatalf("one byte below: error = %v, wrote %d bytes", err, edge.Len())
		}
	})
}
