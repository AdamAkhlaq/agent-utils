package text

import (
	"bytes"
	"strings"
	"testing"
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
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "ragged row", input: "a,b\n1", wantErr: "wrong number of fields"},
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
