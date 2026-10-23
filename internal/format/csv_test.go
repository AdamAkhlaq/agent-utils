package format

import (
	"bytes"
	"encoding/json"
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
		{name: "objects without keys", input: `[{},{}]`, wantErr: "no columns to write"},
		{name: "trailing data", input: `[] "extra"`, wantErr: "unexpected data after the JSON array"},
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
