package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestXMLToJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "text-only element becomes a string",
			input: "<name>Ada</name>",
			want:  "{\n  \"name\": \"Ada\"\n}",
		},
		{
			name:  "attributes become @-prefixed keys",
			input: `<user id="7" role="admin"/>`,
			want:  "{\n  \"user\": {\n    \"@id\": \"7\",\n    \"@role\": \"admin\"\n  }\n}",
		},
		{
			name:  "text next to attributes goes under #text",
			input: `<user id="7">Ada</user>`,
			want:  "{\n  \"user\": {\n    \"#text\": \"Ada\",\n    \"@id\": \"7\"\n  }\n}",
		},
		{
			name:  "nested structure with sorted keys",
			input: "<service><port>80</port><name>api</name></service>",
			want:  "{\n  \"service\": {\n    \"name\": \"api\",\n    \"port\": \"80\"\n  }\n}",
		},
		{
			name:  "repeated siblings become an array",
			input: "<list><item>a</item><item>b</item><item>c</item></list>",
			want:  "{\n  \"list\": {\n    \"item\": [\n      \"a\",\n      \"b\",\n      \"c\"\n    ]\n  }\n}",
		},
		{
			name:  "single sibling stays a scalar, not a one-element array",
			input: "<list><item>a</item></list>",
			want:  "{\n  \"list\": {\n    \"item\": \"a\"\n  }\n}",
		},
		{
			name:  "mixed text and children",
			input: "<p>hello <b>bold</b> world</p>",
			want:  "{\n  \"p\": {\n    \"#text\": \"hello  world\",\n    \"b\": \"bold\"\n  }\n}",
		},
		{
			name:  "whitespace-only text between elements is dropped",
			input: "<a>\n  <b>1</b>\n  <c>2</c>\n</a>",
			want:  "{\n  \"a\": {\n    \"b\": \"1\",\n    \"c\": \"2\"\n  }\n}",
		},
		{
			name:  "meaningful text is trimmed",
			input: "<a>  spaced out  </a>",
			want:  "{\n  \"a\": \"spaced out\"\n}",
		},
		{
			name:  "empty element becomes an empty string",
			input: "<a/>",
			want:  "{\n  \"a\": \"\"\n}",
		},
		{
			name:  "CDATA is text and html characters stay literal",
			input: `<code><![CDATA[if (a < b && c > d) { return "x"; }]]></code>`,
			want:  "{\n  \"code\": \"if (a < b && c > d) { return \\\"x\\\"; }\"\n}",
		},
		{
			name:  "entities are decoded",
			input: "<a>fish &amp; chips</a>",
			want:  "{\n  \"a\": \"fish & chips\"\n}",
		},
		{
			name:  "comments are dropped",
			input: "<a><!-- note -->1<!-- more --></a>",
			want:  "{\n  \"a\": \"1\"\n}",
		},
		{
			name:  "declaration doctype and processing instructions are dropped",
			input: "<?xml version=\"1.0\"?>\n<!DOCTYPE a>\n<?pi data?>\n<a>1</a>",
			want:  "{\n  \"a\": \"1\"\n}",
		},
		{
			name:  "declared namespace prefix kept as written",
			input: `<x:a xmlns:x="urn:u"><x:b>1</x:b></x:a>`,
			want:  "{\n  \"x:a\": {\n    \"@xmlns:x\": \"urn:u\",\n    \"x:b\": \"1\"\n  }\n}",
		},
		{
			name:  "default namespace leaves names unprefixed",
			input: `<a xmlns="urn:d"><b>1</b></a>`,
			want:  "{\n  \"a\": {\n    \"@xmlns\": \"urn:d\",\n    \"b\": \"1\"\n  }\n}",
		},
		{
			name:  "undeclared namespace prefix kept as written",
			input: "<x:a><x:b>1</x:b></x:a>",
			want:  "{\n  \"x:a\": {\n    \"x:b\": \"1\"\n  }\n}",
		},
		{
			name:  "prefixed attribute kept as written",
			input: `<a xmlns:x="urn:u" x:id="1" plain="2"/>`,
			want:  "{\n  \"a\": {\n    \"@plain\": \"2\",\n    \"@x:id\": \"1\",\n    \"@xmlns:x\": \"urn:u\"\n  }\n}",
		},
		{
			name: "realistic rss snippet",
			input: `<rss version="2.0">
  <channel>
    <title>Example</title>
    <item><title>First</title><link>https://e.com/1</link></item>
    <item><title>Second</title><link>https://e.com/2</link></item>
  </channel>
</rss>`,
			want: "{\n  \"rss\": {\n    \"@version\": \"2.0\",\n    \"channel\": {\n      \"item\": [\n        {\n          \"link\": \"https://e.com/1\",\n          \"title\": \"First\"\n        },\n        {\n          \"link\": \"https://e.com/2\",\n          \"title\": \"Second\"\n        }\n      ],\n      \"title\": \"Example\"\n    }\n  }\n}",
		},
		{name: "empty input", input: "", wantErr: "empty input"},
		{name: "whitespace only", input: "  \n\n", wantErr: "empty input"},
		{name: "unclosed tag", input: "<a><b>", wantErr: "invalid XML"},
		{name: "mismatched close tag", input: "<a><b>1</a>", wantErr: "invalid XML"},
		{name: "garbage input", input: "not xml at all", wantErr: "text outside the root element at line 1"},
		{name: "text after the root", input: "<a>1</a>junk", wantErr: "text outside the root element"},
		{name: "second root element", input: "<a>1</a>\n<b>2</b>", wantErr: "second root element <b> at line 2"},
		{name: "comment-only document has no root", input: "<!-- just a comment -->", wantErr: "no root element"},
		{name: "unknown entity", input: "<a>&nbsp;</a>", wantErr: "invalid character entity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := XMLToJSON(&out, strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("XMLToJSON() expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("XMLToJSON() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("XMLToJSON() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Errorf("XMLToJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestXMLToJSONOutputIsValidJSON(t *testing.T) {
	input := `<project><groupId>com.example</groupId><dependencies><dependency><artifactId>junit</artifactId></dependency></dependencies></project>`
	var out bytes.Buffer
	if err := XMLToJSON(&out, strings.NewReader(input)); err != nil {
		t.Fatalf("XMLToJSON() error = %v", err)
	}
	if err := JSONValid(&out); err != nil {
		t.Errorf("XMLToJSON() output is not valid JSON: %v", err)
	}
}

func TestXMLDepthLimit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "at the limit", input: "<a><b><c>x</c></b></a>", want: `{"a":{"b":{"c":"x"}}}`},
		{name: "attributes at the limit", input: `<a><b><c id="1"/></b></a>`, want: `{"a":{"b":{"c":{"@id":"1"}}}}`},
		{name: "past the limit", input: "<a><b><c><d/></c></b></a>", wantErr: "invalid XML: nesting exceeds the depth limit of 3 levels at line 1"},
		{name: "past the limit in a later sibling", input: "<a>\n<b><c/></b>\n<b><c><d/></c></b>\n</a>", wantErr: "depth limit of 3 levels at line 3"},
		{name: "unclosed past the limit", input: "<a><a><a><a>", wantErr: "depth limit of 3 levels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := xmlDocument([]byte(tt.input), 3)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("xmlDocument() error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("xmlDocument() error = %v", err)
			}
			got, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("xmlDocument() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestXMLDepthLimitEncodes checks maxXMLDepth against encoding/json's own
// nesting limit with the deepest JSON an element tree can produce: every
// level a repeated sibling (so an object inside an array), down to an
// element with attributes (an object, not a string).
func TestXMLDepthLimitEncodes(t *testing.T) {
	worstCase := func(depth int) []byte {
		return []byte(strings.Repeat("<a><a/>", depth-1) + `<a id="1"/>` + strings.Repeat("</a>", depth-1))
	}
	doc, err := xmlDocument(worstCase(maxXMLDepth), maxXMLDepth)
	if err != nil {
		t.Fatalf("xmlDocument() at the depth limit error = %v", err)
	}
	if _, err := json.Marshal(doc); err != nil {
		t.Fatalf("json.Marshal() of a tree at the depth limit error = %v", err)
	}
	// One level more would not encode, so the limit is as high as it can be.
	doc, err = xmlDocument(worstCase(maxXMLDepth+1), maxXMLDepth+1)
	if err != nil {
		t.Fatalf("xmlDocument() one level past the depth limit error = %v", err)
	}
	if _, err := json.Marshal(doc); err == nil {
		t.Fatal("json.Marshal() of a tree one level past the depth limit succeeded; maxXMLDepth can be raised")
	}
}

func TestXMLToJSONDepthLimit(t *testing.T) {
	tests := map[string]string{
		"one level past the limit":         strings.Repeat("<a>", maxXMLDepth+1) + strings.Repeat("</a>", maxXMLDepth+1),
		"repeated siblings past the limit": strings.Repeat("<a><a/>", maxXMLDepth) + strings.Repeat("</a>", maxXMLDepth),
		"a million levels":                 strings.Repeat("<a>", 1_000_000) + strings.Repeat("</a>", 1_000_000),
		"a million unclosed levels":        strings.Repeat("<a>", 1_000_000),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := XMLToJSON(&out, strings.NewReader(input))
			want := fmt.Sprintf("invalid XML: nesting exceeds the depth limit of %d levels at line 1", maxXMLDepth)
			if err == nil || err.Error() != want {
				t.Fatalf("XMLToJSON() error = %v, want %q", err, want)
			}
			if out.Len() != 0 {
				t.Errorf("XMLToJSON() wrote %d bytes before failing", out.Len())
			}
		})
	}
}

// FuzzXMLToJSON checks that XMLToJSON never panics and writes valid JSON or
// nothing at all.
func FuzzXMLToJSON(f *testing.F) {
	seeds := []string{
		"<name>Ada</name>",
		`<user id="7" role="admin">Ada<b/></user>`,
		"<list><item>a</item><item>b</item><item>c</item></list>",
		`<r xmlns="u" xmlns:x="v"><x:a x:k="1"/><b xmlns:x="w"><x:c/></b></r>`,
		"<?xml version=\"1.0\"?><!DOCTYPE a><!-- c --><a><![CDATA[<x>]]>&amp;&#65;</a>",
		"<a><b>1</a>",
		"<a>1</a><b>2</b>",
		strings.Repeat("<a>", 20) + strings.Repeat("</a>", 20),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		var out bytes.Buffer
		err := XMLToJSON(&out, strings.NewReader(input))
		if err != nil && out.Len() != 0 {
			t.Fatalf("XMLToJSON() wrote %d bytes before failing: %v", out.Len(), err)
		}
		if err == nil && !json.Valid(out.Bytes()) {
			t.Fatalf("XMLToJSON() output is not valid JSON: %q", out.String())
		}
	})
}
