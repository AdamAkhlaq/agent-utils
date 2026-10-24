package format

import (
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
)

// writeFiles writes name -> content files into a new temporary directory and
// returns its path.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// violations renders a report's errors as "instance_path | schema_path |
// message" lines, which keeps the expectations below readable.
func violations(report SchemaReport) []string {
	lines := make([]string, len(report.Errors))
	for i, v := range report.Errors {
		lines[i] = v.InstancePath + " | " + v.SchemaPath + " | " + v.Message
	}
	return lines
}

func TestJSONSchemaValidate(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		files    map[string]string // other files beside the schema
		instance string
		want     []string // nil: the instance is valid
	}{
		{
			name:     "valid document",
			schema:   `{"type": "object", "required": ["id"], "properties": {"id": {"type": "integer"}}}`,
			instance: `{"id": 7}`,
		},
		{
			name:     "type mismatch at the root has the empty pointer",
			schema:   `{"type": "object"}`,
			instance: `[]`,
			want:     []string{" | /type | got array, want object"},
		},
		{
			name: "nested errors are flattened to their leaves",
			schema: `{"properties": {"items": {"items": {
				"required": ["sku"],
				"properties": {"qty": {"type": "integer", "exclusiveMinimum": 0}}
			}}}}`,
			instance: `{"items": [{"sku": "a", "qty": 1}, {"sku": "b", "qty": 0}, {"qty": 1.5}]}`,
			want: []string{
				"/items/1/qty | /properties/items/items/properties/qty/exclusiveMinimum | must be > 0, got 0",
				"/items/2 | /properties/items/items/required | missing property 'sku'",
				"/items/2/qty | /properties/items/items/properties/qty/type | got number, want integer",
			},
		},
		{
			name:     "array indexes sort numerically",
			schema:   `{"items": {"type": "string"}}`,
			instance: `[1, "a", "b", "c", "d", "e", "f", "g", "h", "i", 2]`,
			want: []string{
				"/0 | /items/type | got number, want string",
				"/10 | /items/type | got number, want string",
			},
		},
		{
			name:     "several errors on one value sort by schema path",
			schema:   `{"properties": {"a": {"minLength": 5, "pattern": "^[0-9]+$", "maxLength": 1}}}`,
			instance: `{"a": "abc"}`,
			want: []string{
				"/a | /properties/a/maxLength | must be at most 1 character long, got 3",
				"/a | /properties/a/minLength | must be at least 5 characters long, got 3",
				"/a | /properties/a/pattern | 'abc' does not match pattern '^[0-9]+$'",
			},
		},
		{
			name:     "a local $ref reports where the keyword lives",
			schema:   `{"$defs": {"id": {"type": "integer"}}, "properties": {"a": {"$ref": "#/$defs/id"}}}`,
			instance: `{"a": "x"}`,
			want:     []string{"/a | /$defs/id/type | got string, want integer"},
		},
		{
			name:     "a $ref to a local file names the file",
			schema:   `{"properties": {"a": {"$ref": "defs/common.json#/$defs/id"}}}`,
			files:    map[string]string{"defs/common.json": `{"$defs": {"id": {"type": "integer"}}}`},
			instance: `{"a": "x"}`,
			want:     []string{"/a | defs/common.json#/$defs/id/type | got string, want integer"},
		},
		{
			name:     "a root $id keeps plain pointers",
			schema:   `{"$id": "https://example.com/order.json", "properties": {"a": {"type": "string"}}}`,
			instance: `{"a": 1}`,
			want:     []string{"/a | /properties/a/type | got number, want string"},
		},
		{
			name:     "a $ref to a built-in meta-schema never needs the network",
			schema:   `{"$ref": "http://json-schema.org/draft-07/schema#"}`,
			instance: `{"minLength": -1}`,
			want:     []string{"/minLength | http://json-schema.org/draft-07/schema#/definitions/nonNegativeInteger/minimum | must be >= 0, got -1"},
		},
		{
			name:     "keys needing escapes",
			schema:   `{"properties": {"a/b~c d": {"type": "string"}}}`,
			instance: `{"a/b~c d": 1}`,
			want:     []string{"/a~1b~0c d | /properties/a~1b~0c d/type | got number, want string"},
		},
		{
			name:     "no $schema means 2020-12",
			schema:   `{"prefixItems": [{"type": "string"}], "items": false}`,
			instance: `["a", 1]`,
			want:     []string{"/1 | /items | not allowed: the schema here is false"},
		},
		{
			name:     "draft-07 tuple items",
			schema:   `{"$schema": "http://json-schema.org/draft-07/schema#", "items": [{"type": "string"}], "additionalItems": false}`,
			instance: `["a", 1, 2]`,
			want:     []string{" | /additionalItems | 2 items too many: additionalItems is false"},
		},
		{
			name:     "draft-04 boolean exclusiveMaximum",
			schema:   `{"$schema": "http://json-schema.org/draft-04/schema#", "maximum": 5, "exclusiveMaximum": true}`,
			instance: `5`,
			want:     []string{" | /exclusiveMaximum | must be < 5, got 5"},
		},
		{
			name:     "format is asserted in 2020-12",
			schema:   `{"format": "email"}`,
			instance: `"not-an-email"`,
			want:     []string{" | /format | 'not-an-email' is not valid email: missing @"},
		},
		{
			name:     "format is asserted in 2019-09",
			schema:   `{"$schema": "https://json-schema.org/draft/2019-09/schema", "format": "ipv4"}`,
			instance: `"1.2.3.999"`,
			want:     []string{" | /format | '1.2.3.999' is not valid ipv4: decimal must be between 0 and 255"},
		},
		{
			name:     "format is asserted in draft-07",
			schema:   `{"$schema": "http://json-schema.org/draft-07/schema#", "format": "date"}`,
			instance: `"2024-02-30"`,
			want:     []string{` | /format | '2024-02-30' is not valid date: parsing time "2024-02-30": day out of range`},
		},
		{
			name:     "valid formats pass",
			schema:   `{"properties": {"e": {"format": "email"}, "d": {"format": "date-time"}, "u": {"format": "uuid"}}}`,
			instance: `{"e": "a@example.com", "d": "2024-02-29T12:00:00Z", "u": "123e4567-e89b-12d3-a456-426614174000"}`,
		},
		{
			name:     "unknown and unimplemented formats are ignored",
			schema:   `{"properties": {"a": {"format": "phone"}, "b": {"format": "idn-email"}}}`,
			instance: `{"a": "anything", "b": "anything"}`,
		},
		{
			name:     "content keywords are annotations",
			schema:   `{"contentEncoding": "base64", "contentMediaType": "application/json"}`,
			instance: `"!!!"`,
		},
		{
			name:     "integers beyond float64 compare exactly",
			schema:   `{"maximum": 9007199254740992}`,
			instance: `9007199254740993`,
			want:     []string{" | /maximum | must be <= 9007199254740992, got 9007199254740993"},
		},
		{
			name:     "decimals compare exactly",
			schema:   `{"maximum": 0.1}`,
			instance: `0.10000000000000000001`,
			want:     []string{" | /maximum | must be <= 0.1, got 0.10000000000000000001"},
		},
		{
			name:     "multipleOf uses exact decimal arithmetic",
			schema:   `{"items": {"multipleOf": 0.01}}`,
			instance: `[0.07, 19.99, 1.005]`,
			want:     []string{"/2 | /items/multipleOf | must be a multiple of 0.01, got 1.005"},
		},
		{
			name:     "an integer-valued number is an integer",
			schema:   `{"items": {"type": "integer"}}`,
			instance: `[1.0, 1e400, 1.5]`,
			want:     []string{"/2 | /items/type | got number, want integer"},
		},
		{
			name:     "huge and tiny numbers use exponents",
			schema:   `{"items": {"maximum": 1e-7}}`,
			instance: `[1e400, 0.0000001]`,
			want:     []string{"/0 | /items/maximum | must be <= 1e-7, got 1e+400"},
		},
		{
			name:     "additional properties are listed in sorted order",
			schema:   `{"properties": {"a": true}, "additionalProperties": false}`,
			instance: `{"a": 1, "zeta": 1, "beta": 1, "mu": 1}`,
			want:     []string{" | /additionalProperties | additional properties 'beta', 'mu', 'zeta' not allowed"},
		},
		{
			name:     "a single additional property",
			schema:   `{"additionalProperties": false}`,
			instance: `{"a": 1}`,
			want:     []string{" | /additionalProperties | additional property 'a' not allowed"},
		},
		{
			name:     "enum lists composite values as JSON",
			schema:   `{"enum": [{"a": 1}, [1, 2], "x", null, 1.50]}`,
			instance: `2`,
			want:     []string{` | /enum | value must be one of {"a":1}, [1,2], 'x', null, 1.50`},
		},
		{
			name:     "const",
			schema:   `{"const": "on"}`,
			instance: `"off"`,
			want:     []string{" | /const | value must be 'on'"},
		},
		{
			name:     "anyOf lists every alternative's failure",
			schema:   `{"anyOf": [{"type": "string"}, {"type": "integer"}]}`,
			instance: `true`,
			want: []string{
				" | /anyOf/0/type | got boolean, want string",
				" | /anyOf/1/type | got boolean, want integer",
			},
		},
		{
			name:     "oneOf matching twice",
			schema:   `{"oneOf": [{"type": "number"}, {"type": "integer"}]}`,
			instance: `1`,
			want:     []string{" | /oneOf | 'oneOf' failed, subschemas 0 and 1 both matched"},
		},
		{
			name:     "not",
			schema:   `{"not": {"type": "integer"}}`,
			instance: `1`,
			want:     []string{" | /not | must not match the 'not' schema"},
		},
		{
			name:     "if then",
			schema:   `{"if": {"properties": {"kind": {"const": "card"}}}, "then": {"required": ["number"]}}`,
			instance: `{"kind": "card"}`,
			want:     []string{" | /then/required | missing property 'number'"},
		},
		{
			name:     "contains reports the summary, not every item",
			schema:   `{"contains": {"type": "string"}}`,
			instance: `[1, 2]`,
			want:     []string{" | /contains | no items match contains schema"},
		},
		{
			name:     "minContains",
			schema:   `{"contains": {"type": "string"}, "minContains": 2}`,
			instance: `[1, "a"]`,
			want:     []string{" | /minContains | must have at least 2 items matching the contains schema, got 1 (at 1)"},
		},
		{
			name:     "a false schema",
			schema:   `false`,
			instance: `{}`,
			want:     []string{" |  | not allowed: the schema here is false"},
		},
		{
			name:     "unevaluatedProperties",
			schema:   `{"properties": {"a": true}, "unevaluatedProperties": false}`,
			instance: `{"a": 1, "b": 2}`,
			want:     []string{"/b | /unevaluatedProperties | not allowed: the schema here is false"},
		},
		{
			name:     "dependentRequired",
			schema:   `{"dependentRequired": {"card": ["expiry"]}}`,
			instance: `{"card": "x"}`,
			want:     []string{" | /dependentRequired/card | properties 'expiry' required, if 'card' exists"},
		},
		{
			name:     "draft-07 dependencies point at the keyword as written",
			schema:   `{"$schema": "http://json-schema.org/draft-07/schema#", "dependencies": {"a": ["b"], "c": {"required": ["d"]}}}`,
			instance: `{"a": 1, "c": 1}`,
			want: []string{
				" | /dependencies/a | properties 'b' required, if 'a' exists",
				" | /dependencies/c/required | missing property 'd'",
			},
		},
		{
			name:     "uniqueItems compares numbers by value",
			schema:   `{"uniqueItems": true}`,
			instance: `[1, 2, 1.0]`,
			want:     []string{" | /uniqueItems | items at 0 and 2 are equal"},
		},
		{
			name:     "size limits",
			schema:   `{"minProperties": 3, "properties": {"a": {"maxItems": 1}}}`,
			instance: `{"a": [1, 2]}`,
			want: []string{
				" | /minProperties | must have at least 3 properties, got 1",
				"/a | /properties/a/maxItems | must have at most 1 item, got 2",
			},
		},
		{
			name:     "propertyNames failures are placed on their objects",
			schema:   `{"items": {"propertyNames": {"maxLength": 2}}}`,
			instance: `[{"bad": 1}, {"ok": 1}, {"bad": 1, "worse": 2}]`,
			want: []string{
				"/0 | /items/propertyNames | invalid property name 'bad': must be at most 2 characters long, got 3",
				"/2 | /items/propertyNames | invalid property name 'bad': must be at most 2 characters long, got 3",
				"/2 | /items/propertyNames | invalid property name 'worse': must be at most 2 characters long, got 5",
			},
		},
		{
			name:     "deep propertyNames failures",
			schema:   `{"properties": {"x": {"items": {"properties": {"y": {"propertyNames": {"pattern": "^[a-z]+$"}}}}}}}`,
			instance: `{"x": [{"y": {"A": 1}}, {"y": {"b": 1}}, {"y": {"b": 1}}]}`,
			want:     []string{"/x/0/y | /properties/x/items/properties/y/propertyNames | invalid property name 'A': 'A' does not match pattern '^[a-z]+$'"},
		},
		{
			name:     "an ambiguous propertyNames failure falls back to an ancestor",
			schema:   `{"properties": {"a": {"propertyNames": {"maxLength": 2}}}}`,
			instance: `{"a": {"bad": 1}, "b": {"bad": 1}}`,
			want:     []string{" | /properties/a/propertyNames | invalid property name 'bad': must be at most 2 characters long, got 3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{"schema.json": tt.schema}
			for name, content := range tt.files {
				files[name] = content
			}
			dir := writeFiles(t, files)
			t.Chdir(dir)
			schema, err := CompileJSONSchema("schema.json")
			if err != nil {
				t.Fatalf("CompileJSONSchema() error = %v", err)
			}
			report, err := schema.Validate(strings.NewReader(tt.instance))
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if report.Valid != (tt.want == nil) {
				t.Errorf("Validate() valid = %v, want %v", report.Valid, tt.want == nil)
			}
			if report.Errors == nil {
				t.Error("Validate() errors = nil, want a non-nil slice")
			}
			got := violations(report)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("Validate() violations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

// Object properties are validated in map iteration order, which Go
// randomizes, so the library's own error order (and its propertyNames
// locations) varies from run to run; the report must not.
func TestJSONSchemaValidateIsDeterministic(t *testing.T) {
	dir := writeFiles(t, map[string]string{"schema.json": `{
		"properties": {"a": {"type": "string"}, "b": {"type": "string"}, "c": {"type": "string"}, "d": {"type": "string"},
			"n": {"items": {"propertyNames": {"maxLength": 1}}}},
		"required": ["x", "y"],
		"additionalProperties": {"type": "integer"}
	}`})
	schema, err := CompileJSONSchema(filepath.Join(dir, "schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	instance := `{"a": 1, "b": 2, "c": 3, "d": 4, "e": "x", "f": "y", "n": [{"ab": 1}, {"a": 1}, {"cd": 1}]}`
	var first string
	for i := range 50 {
		report, err := schema.Validate(strings.NewReader(instance))
		if err != nil {
			t.Fatal(err)
		}
		got, err := SchemaReportJSON(report)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("run %d report differs:\n%s\nfirst run:\n%s", i, got, first)
		}
	}
	if n := strings.Count(first, `"instance_path"`); n != 9 {
		t.Errorf("report has %d violations, want 9:\n%s", n, first)
	}
}

func TestJSONSchemaValidateInputErrors(t *testing.T) {
	tests := []struct {
		name     string
		instance string
		wantErr  string
	}{
		{name: "empty input", instance: "", wantErr: "empty input"},
		{name: "whitespace only", instance: " \n\t\r", wantErr: "empty input"},
		{name: "non-JSON whitespace is not empty", instance: "\u00a0", wantErr: "invalid JSON at line 1"},
		{name: "invalid JSON", instance: "{\n  \"a\": }", wantErr: "invalid JSON at line 2, column 8"},
		{name: "trailing data", instance: `{} {}`, wantErr: "invalid JSON at line 1, column 4: invalid character '{' after top-level value"},
		{name: "duplicate key", instance: "{\"a\": 1,\n \"a\": 2}", wantErr: `duplicate key "a" at line 2, column 2`},
		{name: "nested duplicate key", instance: `[{"b": {"x": 1, "x": 1}}]`, wantErr: `duplicate key "x" at line 1, column 17`},
		{name: "invalid UTF-8", instance: "\"ok \xff\"", wantErr: "invalid UTF-8 at line 1, column 5"},
		{name: "UTF-16", instance: "\xff\xfe{\x00}\x00", wantErr: "this is UTF-16LE"},
		{name: "exponent out of range", instance: `{"n": 1e9999999}`, wantErr: "number 1e9999999 at line 1, column 7 is out of range"},
		{name: "negative exponent out of range", instance: `[1e-1000001]`, wantErr: "number 1e-1000001 at line 1, column 2 is out of range"},
	}
	dir := writeFiles(t, map[string]string{"schema.json": `{"items": {"minimum": 0}, "uniqueItems": true}`})
	schema, err := CompileJSONSchema(filepath.Join(dir, "schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := schema.Validate(strings.NewReader(tt.instance))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
	t.Run("read failure", func(t *testing.T) {
		readErr := errors.New("disk on fire")
		_, err := schema.Validate(iotest.ErrReader(readErr))
		if !errors.Is(err, readErr) {
			t.Fatalf("Validate() error = %v, want it to wrap %v", err, readErr)
		}
	})
	t.Run("largest supported exponent", func(t *testing.T) {
		report, err := schema.Validate(strings.NewReader(`[1e1000000, -1]`))
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if got := violations(report); len(got) != 1 || got[0] != "/1 | /items/minimum | must be >= 0, got -1" {
			t.Errorf("Validate() violations = %q", got)
		}
	})
}

// The library reports a $ref loop while validating, as if the document
// were at fault; it is the schema that can never be evaluated.
func TestJSONSchemaValidateRefLoop(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		wantErr string
	}{
		{
			name:    "self reference",
			schema:  `{"$ref": "#"}`,
			wantErr: `cannot be validated: schema schema.json has a $ref loop: /$ref and (root) both resolve to schema.json# without consuming input (reached at instance path "")`,
		},
		{
			name:    "loop through definitions",
			schema:  `{"$defs": {"a": {"$ref": "#/$defs/b"}, "b": {"$ref": "#/$defs/a"}}, "properties": {"x": {"$ref": "#/$defs/a"}}}`,
			wantErr: `cannot be validated: schema schema.json has a $ref loop: /properties/x/$ref/$ref/$ref and /properties/x/$ref both resolve to schema.json#/$defs/a without consuming input (reached at instance path "/x")`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(writeFiles(t, map[string]string{"schema.json": tt.schema}))
			schema, err := CompileJSONSchema("schema.json")
			if err != nil {
				t.Fatalf("CompileJSONSchema() error = %v", err)
			}
			_, err = schema.Validate(strings.NewReader(`{"x": 1}`))
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestCompileJSONSchemaErrors(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		schema  string // path to compile; defaults to schema.json
		wantErr string
	}{
		{name: "missing file", schema: "missing.json", wantErr: "reading schema: open missing.json"},
		{name: "directory", files: map[string]string{"dir/x.json": "{}"}, schema: "dir", wantErr: "reading schema"},
		{name: "empty file", files: map[string]string{"schema.json": "\n"}, wantErr: "schema schema.json: empty input"},
		{name: "invalid JSON", files: map[string]string{"schema.json": `{"type": }`}, wantErr: "schema schema.json: invalid JSON at line 1, column 10"},
		{name: "duplicate key", files: map[string]string{"schema.json": `{"type": "object", "type": "string"}`}, wantErr: `schema schema.json: duplicate key "type" at line 1, column 20`},
		{name: "exponent out of range", files: map[string]string{"schema.json": `{"minimum": 1e9999999}`}, wantErr: "number 1e9999999 at line 1, column 13 is out of range"},
		{
			name:    "fails its meta-schema",
			files:   map[string]string{"schema.json": `{"properties": {"a": {"minLength": -1}}}`},
			wantErr: "invalid schema schema.json: /properties/a/minLength: must be >= 0, got -1",
		},
		{
			name:    "not an object or boolean",
			files:   map[string]string{"schema.json": `5`},
			wantErr: "invalid schema schema.json: (root): got number, want boolean or object",
		},
		{
			name:    "draft-07 tuple items without $schema",
			files:   map[string]string{"schema.json": `{"items": [{"type": "string"}]}`},
			wantErr: "invalid schema schema.json: /items: got array, want boolean or object",
		},
		{
			name:    "ECMAScript-only regex",
			files:   map[string]string{"schema.json": `{"pattern": "^(?=a)"}`},
			wantErr: "invalid schema schema.json: /pattern: '^(?=a)' is not valid regex",
		},
		{
			name:    "unresolvable pointer",
			files:   map[string]string{"schema.json": `{"$ref": "#/$defs/missing"}`},
			wantErr: `json-pointer in "schema.json#/$defs/missing" not found`,
		},
		{
			name:    "missing local file",
			files:   map[string]string{"schema.json": `{"$ref": "other.json"}`},
			wantErr: "cannot load other.json: no such file or directory",
		},
		{
			name:    "malformed local file",
			files:   map[string]string{"schema.json": `{"$ref": "defs/other.json"}`, "defs/other.json": `{"type":`},
			wantErr: "cannot load defs/other.json: invalid JSON",
		},
		{
			name:    "local file failing its meta-schema",
			files:   map[string]string{"schema.json": `{"$ref": "defs/other.json"}`, "defs/other.json": `{"type": "strin"}`},
			wantErr: "invalid schema defs/other.json: /type: value must be one of",
		},
		{
			name:    "remote $ref",
			files:   map[string]string{"schema.json": `{"$ref": "https://example.com/schema.json"}`},
			wantErr: `cannot resolve "https://example.com/schema.json": only local files and the built-in draft meta-schemas can be referenced (json-schema never accesses the network)`,
		},
		{
			name:    "relative $ref under a remote $id",
			files:   map[string]string{"schema.json": `{"$id": "https://example.com/root.json", "$ref": "other.json"}`},
			wantErr: `cannot resolve "https://example.com/other.json"`,
		},
		{
			name:    "custom remote meta-schema",
			files:   map[string]string{"schema.json": `{"$schema": "https://example.com/meta"}`},
			wantErr: `cannot resolve "https://example.com/meta"`,
		},
		{
			name:    "opaque file URL",
			files:   map[string]string{"schema.json": `{"$ref": "file:other.json"}`},
			wantErr: `cannot load file:other.json: not a usable file URL`,
		},
		{
			name:    "remote json-schema.org file that is not a meta-schema",
			files:   map[string]string{"schema.json": `{"$ref": "https://json-schema.org/learn/examples/address.schema.json"}`},
			wantErr: `cannot resolve "https://json-schema.org/learn/examples/address.schema.json": only local files`,
		},
		{
			name:    "unsupported draft",
			files:   map[string]string{"schema.json": `{"$schema": "http://json-schema.org/draft-03/schema#"}`},
			wantErr: `unsupported meta-schema "http://json-schema.org/draft-03/schema": the supported drafts are 4, 6, 7, 2019-09, and 2020-12`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(writeFiles(t, tt.files))
			path := tt.schema
			if path == "" {
				path = "schema.json"
			}
			_, err := CompileJSONSchema(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CompileJSONSchema() error = %v, want it to contain %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "file://") {
				t.Errorf("CompileJSONSchema() error = %v, want file URLs shown as paths", err)
			}
		})
	}
}

// A schema naming a live server must fail without a single request reaching
// it: the command is offline by contract.
func TestCompileJSONSchemaNeverUsesTheNetwork(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"type": "string"}`))
	}))
	defer server.Close()
	for _, schema := range []string{
		`{"$ref": "` + server.URL + `/schema.json"}`,
		`{"$schema": "` + server.URL + `/meta"}`,
		`{"$id": "` + server.URL + `/root.json", "properties": {"a": {"$ref": "a.json"}}}`,
	} {
		dir := writeFiles(t, map[string]string{"schema.json": schema})
		_, err := CompileJSONSchema(filepath.Join(dir, "schema.json"))
		if err == nil || !strings.Contains(err.Error(), "never accesses the network") {
			t.Errorf("CompileJSONSchema(%s) error = %v, want a refused reference", schema, err)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server received %d requests, want none", n)
	}
}

// A schema path is a file path, never a URL: "#" and spaces are just
// characters in it, and other local files show relative to its directory.
func TestCompileJSONSchemaPathWithURLCharacters(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"a dir #1/schema.json": `{"$ref": "../defs.json"}`,
		"defs.json":            `{"type": "integer"}`,
	})
	t.Chdir(dir)
	schema, err := CompileJSONSchema("a dir #1/schema.json")
	if err != nil {
		t.Fatalf("CompileJSONSchema() error = %v", err)
	}
	report, err := schema.Validate(strings.NewReader(`"x"`))
	if err != nil {
		t.Fatal(err)
	}
	if got := violations(report); len(got) != 1 || got[0] != " | ../defs.json#/type | got string, want integer" {
		t.Errorf("Validate() violations = %q", got)
	}
}

func TestSchemaReportJSON(t *testing.T) {
	tests := []struct {
		name   string
		report SchemaReport
		want   string
	}{
		{
			name:   "valid keeps an empty errors array",
			report: SchemaReport{Valid: true, Errors: []SchemaViolation{}},
			want:   "{\n  \"valid\": true,\n  \"errors\": []\n}",
		},
		{
			name: "violations, without HTML escaping",
			report: SchemaReport{Errors: []SchemaViolation{
				{InstancePath: "/a", SchemaPath: "/properties/a/pattern", Message: "'<x>' does not match pattern '^&$'"},
			}},
			want: "{\n  \"valid\": false,\n  \"errors\": [\n    {\n      \"instance_path\": \"/a\",\n      \"schema_path\": \"/properties/a/pattern\",\n      \"message\": \"'<x>' does not match pattern '^&$'\"\n    }\n  ]\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SchemaReportJSON(tt.report)
			if err != nil {
				t.Fatalf("SchemaReportJSON() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("SchemaReportJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0", "0"},
		{"-0.0", "0"},
		{"7", "7"},
		{"-7", "-7"},
		{"1.50", "1.5"},
		{"9007199254740993", "9007199254740993"},
		{"123456789012345678901", "123456789012345678901"},
		{"1234567890123456789012", "1.234567890123456789012e+21"},
		{"1e21", "1e+21"},
		{"1e20", "100000000000000000000"},
		{"0.10000000000000000001", "0.10000000000000000001"},
		{"0.000001", "0.000001"},
		{"0.0000001", "1e-7"},
		{"-0.00000012", "-1.2e-7"},
		{"1e400", "1e+400"},
		{"12.5e-3", "0.0125"},
		{"1e-999999", "1e-999999"},
		{"-3.75e-999990", "-3.75e-999990"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			r, ok := new(big.Rat).SetString(tt.in)
			if !ok {
				t.Fatalf("bad test number %q", tt.in)
			}
			if got := formatNumber(r); got != tt.want {
				t.Errorf("formatNumber(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
	for _, r := range []*big.Rat{big.NewRat(1, 3), big.NewRat(1, 75), big.NewRat(7, 3*125)} {
		t.Run("non-decimal "+r.String(), func(t *testing.T) {
			if got := formatNumber(r); got != r.RatString() {
				t.Errorf("formatNumber(%s) = %q, want %q", r, got, r.RatString())
			}
		})
	}
}

func TestComparePointers(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "/a", -1},
		{"/a", "/a/b", -1},
		{"/a/b", "/a", 1},
		{"/items/2", "/items/10", -1},
		{"/items/10", "/items/9", 1},
		{"/items/01", "/items/1", -1},
		{"/b", "/a/z", 1},
		{"/10", "/a", -1},
	}
	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			if got := comparePointers(tt.a, tt.b); got != tt.want {
				t.Errorf("comparePointers(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
